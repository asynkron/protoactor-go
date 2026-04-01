# Grain Peek: Non-Activating Liveness Check

## Problem

There is no way to check whether a grain/actor exists and is still alive without triggering activation. Every path through the identity lookup system (`IdentityLookup.Get`, `Cluster.Get`, `Cluster.Request`) is get-or-create — if the grain doesn't exist, it gets spawned.

This makes diagnostics, monitoring, and routing-optimization use cases impossible without side effects. The .NET upstream has the same limitation.

## Goal

Add a `Peek` method that, given an `{identity, kind}`, returns liveness information about a grain without activating it. The result should indicate not just whether an activation record exists, but whether the owning member is alive and whether the actor process is confirmed running.

Stale answers are acceptable — this is primarily a diagnostics feature. There is an inherent TOCTOU gap in any distributed liveness check.

## Design

### Proto Messages

Added to `cluster/cluster.proto` for cross-node serialization (both serializers require `proto.Message`):

```protobuf
message PeekRequest {
  ClusterIdentity cluster_identity = 1;
}

message PeekResponse {
  bool found = 1;
  actor.PID pid = 2;
}
```

`PeekRequest` is sent from the identity lookup on any node to the placement actor (via the activator proxy) on the owning node. The placement actor checks its local `actors` map without spawning and responds.

### Go Types

New file `cluster/peek.go`:

```go
type PeekStatus int

const (
    PeekStatusNotFound   PeekStatus = iota // No activation record anywhere
    PeekStatusAlive                        // Record exists, member alive, process confirmed by placement actor
    PeekStatusMemberDead                   // Record exists, but owning member is not in MemberList
    PeekStatusStale                        // Record exists, member alive, but process not in placement actor's map
)

type PeekResult struct {
    *GrainInfo
    Status PeekStatus
}
```

`PeekStatus` and `PeekResult` are Go-only types — they are the caller-facing return value, not sent over the wire.

### Interface Changes

`IdentityLookup` — `Peek` added as a required method:

```go
type IdentityLookup interface {
    Get(clusterIdentity *ClusterIdentity) *actor.PID
    Peek(clusterIdentity *ClusterIdentity) (*PeekResult, error)
    RemovePid(clusterIdentity *ClusterIdentity, pid *actor.PID)
    Setup(cluster *Cluster, kinds []string, isClient bool)
    Shutdown()
}
```

`Cluster` — convenience method:

```go
func (c *Cluster) Peek(identity, kind string) (*PeekResult, error) {
    return c.IdentityLookup.Peek(NewClusterIdentity(identity, kind))
}
```

### Placement Actor

Add a `*PeekRequest` case in the placement actor's `Receive` method. The handler:

1. Looks up `msg.ClusterIdentity.AsKey()` in `p.actors` map.
2. If found: responds with `PeekResponse{Found: true, Pid: meta.PID}`.
3. If not found (or if `p.stopping`): responds with `PeekResponse{Found: false}`.

No spawning, no side effects. This runs on the placement actor's mailbox so access to `p.actors` is single-threaded — no additional locking needed.

### Activator Proxy

Add a `*PeekRequest` case in the activator proxy's `Receive` method. It forwards to the local placement actor and responds with the result, same pattern as `ActivationRequest` forwarding. Uses `ReenterAfter` for the async forward.

### Implementation Per Backend

All backends follow the same three-step pattern:

1. **Check for activation record** (backend-specific, read-only)
2. **Validate member liveness** via `MemberList.ContainsMemberID` (local, cheap)
3. **Confirm process alive** by sending `PeekRequest` to `$proxy-activator` on the owning member

The final `PeekStatus` is determined by combining the results:

| Record exists? | Member alive? | Placement confirms? | Status       |
|---------------|---------------|-------------------- |------------- |
| No            | —             | —                   | `NotFound`   |
| Yes           | No            | —                   | `MemberDead` |
| Yes           | Yes           | Yes                 | `Alive`      |
| Yes           | Yes           | No                  | `Stale`      |
| Yes           | Yes           | Error/timeout       | Return error |

#### disthash

- Snapshot rendezvous under `rdvMutex.RLock` (same pattern as `Get`)
- Hash identity to owner address via `rdv.GetByClusterIdentity`
- If owner address is empty → `NotFound`
- Resolve address to member: iterate `MemberList.Members()` to find the member with matching `Address()`. If no member matches → `MemberDead` (GrainInfo populated with what we know: identity, kind, address). Note: `MemberSet` is keyed by member ID, not address, so a linear scan is needed. This is acceptable since Peek is a diagnostic operation, not a hot path. Alternatively, a `GetMemberByAddress` helper could be added to `MemberSet` if desired.
- Send `PeekRequest` to `PidOfActivatorActor(ownerAddress)`
- Map response to `Alive` (with GrainInfo from PeekResponse PID) or `NotFound`

Note: disthash has no persistent activation records — the placement actor's in-memory `actors` map is the only source of truth. If the member is alive but the placement actor says not found, the grain simply doesn't exist, so this maps to `NotFound` (not `Stale`). The `Stale` status is only meaningful for storage-backed lookups where a record can outlive the process.

#### natskv

- Call existing `getExistingActivation()` — reads from NATS KV, no side effects
- If no record → `NotFound`
- Parse member ID from record, check `MemberList.ContainsMemberID` → if dead, `MemberDead`
- Send `PeekRequest` to `$proxy-activator` on owning member's address
- Map response: confirmed → `Alive`, not confirmed → `Stale`

#### natsstream

Same pattern as natskv — uses its own `getExistingActivation()` to read from the NATS stream.

#### IdentityStorageLookup (storage-based: redis, postgres, nats, inmemory)

- Call `storage.TryGetExistingActivation(ci)` — existing method, pure read, no side effects
- If nil → `NotFound`
- Validate member via `MemberList.ContainsMemberID` → if dead, `MemberDead`
- Send `PeekRequest` to `$proxy-activator` on owning member
- Map response: confirmed → `Alive`, not confirmed → `Stale`

### Thread Safety

- **Placement actor handler**: Runs on the actor's mailbox — single-threaded access to `p.actors`. No locks needed.
- **Activator proxy handler**: Runs on its mailbox. Uses `ReenterAfter` for the async forward (same as existing `ActivationRequest` forwarding).
- **disthash rendezvous**: Uses existing `rdvMutex.RLock`/`RUnlock` snapshot pattern.
- **MemberList checks**: `ContainsMemberID` acquires `ml.mutex.RLock` internally.
- **NATS KV/stream reads**: Read-only NATS operations, inherently safe.
- **StorageLookup.TryGetExistingActivation**: InMemory uses mutex; Redis/Postgres/NATS are inherently safe.
- **PidCache**: Not consulted in Peek — we go to the source of truth for diagnostics.

### What Is NOT Changed

- `plugin/passivation.go` — unrelated
- `cluster/grain_registry.go` — already delegates to `GrainEnumerator`, no changes
- `cluster/default_context.go` — Peek is a one-shot diagnostic call, not a request-with-retry pattern
- `StorageLookup` interface — `TryGetExistingActivation` already provides what's needed

## Files Touched

### Proto / generated
- `cluster/cluster.proto` — add `PeekRequest`, `PeekResponse` messages
- `cluster/cluster.pb.go` — regenerate

### New files
- `cluster/peek.go` — `PeekStatus`, `PeekResult`, `PeekStatus.String()`

### Core cluster
- `cluster/identity_lookup.go` — add `Peek` to `IdentityLookup` interface
- `cluster/cluster.go` — add `Cluster.Peek()` convenience method
- `cluster/placement.go` — add `*PeekRequest` case in `Receive`, add `onPeekRequest` handler
- `cluster/activator_proxy.go` — add `*PeekRequest` case in `Receive`, forward to placement actor

### Identity lookup implementations
- `cluster/identitylookup/disthash/identity_lookup.go` — implement `Peek`
- `cluster/clusterproviders/natskv/natskv_identity.go` — implement `Peek`
- `cluster/clusterproviders/natsstream/natsstream_identity.go` — implement `Peek`
- `cluster/identitylookup/storage/identity_storage_lookup.go` — implement `Peek`

### Tests
- `cluster/placement_test.go` — PeekRequest handler: found, not found, stopping, concurrent with activation (`-race`)
- `cluster/activator_proxy_test.go` — PeekRequest forwarding, timeout handling
- `cluster/cluster_test.go` — `Cluster.Peek()` delegation
- `cluster/identitylookup/disthash/` — disthash Peek: active grain → `Alive`, no activation �� `NotFound`, after termination → `NotFound`
- `cluster/clusterproviders/natskv/` — natskv Peek: `Alive`, `NotFound`, `MemberDead`, `Stale`
- `cluster/clusterproviders/natsstream/` — natsstream Peek: same matrix as natskv
- `cluster/identitylookup/storage/` — storage Peek: `Alive`, `NotFound`, `MemberDead`, `Stale` (using InMemoryStorageLookup)

All concurrent tests run with `-race`. NATS tests use testcontainers per existing patterns.

## Pre-Existing Issue Noted

`ListGrainsRequest`/`ListGrainsResponse` are plain Go structs sent cross-node in the disthash `fanOutListGrains` implementation. Both remote serializers require `proto.Message`, so this would fail for actual multi-node clusters. The new `PeekRequest`/`PeekResponse` are defined as protobuf messages to avoid this problem. Fixing the `ListGrains` messages is out of scope for this spec but should be tracked separately.
