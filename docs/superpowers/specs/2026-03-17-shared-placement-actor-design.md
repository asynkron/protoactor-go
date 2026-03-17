# Shared Placement Actor Design

## Problem

All non-disthash identity providers (natskv, natsstream, `IdentityStorageLookup` with Redis/Postgres/NATS KV backends) spawn actors directly via `SpawnNamed` with no local lifecycle management. This causes:

1. **No graceful shutdown** — when a node shuts down, local grain actors are not poisoned with deactivation reasons. They die abruptly when `ActorSystem.Shutdown()` tears everything down.
2. **No local tracking** — no in-memory map of which grains this node owns, making enumeration and cleanup unreliable.
3. **No termination cleanup** — when a grain stops naturally, nothing notifies the identity storage to remove the activation record. The `handleStopped` middleware clears the PID cache but doesn't touch the identity store.
4. **No spawn verification** — no `CanSpawnIdentity` predicate support for rejecting invalid identities before spawning.
5. **No duplicate spawn prevention** — concurrent `Get()` calls for the same identity can race to spawn, leading to the `ErrNameExists` issues we patched earlier.
6. **No remote placement** — no way to direct a grain to spawn on a specific member based on affinity, load, or capacity. The lock-holder always spawns locally.
7. **No capacity-aware placement** — no strategy layer to select the least-loaded member. The Go gossip infrastructure already propagates per-kind actor counts, but nothing uses them for placement decisions.
8. **Orphaned state on RemovePid** — `RemovePid` deletes identity records without stopping the actor, creating the liveness bug we just fixed with a three-layer defense. A placement actor that owns both the record and the process would eliminate this class of bug entirely.

The .NET upstream (`Proto.Actor`) solves this with `IdentityStoragePlacementActor`, `IdentityStorageWorker`, `IdentityActivatorProxy`, and pluggable `IMemberStrategy` implementations. The Go codebase has a TODO acknowledging this gap (`identity_storage_lookup.go` line 144).

## Architecture

### Component Overview

```
┌──────────────────────────────────────────────────────────────────┐
│                       Identity Lookup                             │
│  (natskv / natsstream / storage+redis / storage+pg /              │
│   storage+nats / disthash)                                        │
│                                                                   │
│  Responsibilities:                                                │
│  - Distributed lock acquisition                                   │
│  - Activation record storage/retrieval                            │
│  - Stale member validation                                        │
│  - Member cleanup on topology events                              │
│  - Duplicate-request coalescing (worker pattern)                  │
│                                                                   │
│  Get() flow:                                                      │
│  1. Check PID cache                                               │
│  2. Check existing activation in store (validate member alive)    │
│  3. Acquire lock                                                  │
│  4. Select target member via MemberStrategy                       │
│  5. Send ActivationRequest to target member's placement actor     │
│  6. Return PID from ActivationResponse                            │
└────────────────────────┬─────────────────────────────────────────┘
                         │ ActivationRequest (proto msg)
                         │ (local or remote via ActivatorProxy)
                         ▼
┌──────────────────────────────────────────────────────────────────┐
│  Activator Proxy ($proxy-activator)     Per non-client member     │
│  Receives remote ActivationRequests, forwards to local            │
│  $placement-activator. Thin routing layer.                        │
└────────────────────────┬─────────────────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────────────┐
│  Placement Actor ($placement-activator)  Per non-client member    │
│                                                                   │
│  Responsibilities:                                                │
│  - Spawn actors locally (SpawnNamed + Watch)                      │
│  - Track all local grains (ClusterIdentity → PID map)             │
│  - Persist activation via callback (ReenterAfter)                 │
│  - Poison all local grains on Stopping (graceful)                 │
│  - Handle *actor.Terminated → cleanup storage                     │
│  - CanSpawnIdentity verification (optional per-kind, async)       │
│  - Duplicate spawn prevention (in-flight set)                     │
│  - Topology rebalance (optional, for disthash)                    │
└──────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────┐
│  Member Strategy (per-kind, pluggable)                            │
│                                                                   │
│  Selects which member should host a grain activation.             │
│  Implementations:                                                 │
│  - RoundRobinStrategy (default, simple cyclic selection)          │
│  - LocalAffinityStrategy (prefer local node, fall back to RR)     │
│  - RendezvousStrategy (deterministic hash-based, cache-friendly)  │
│  - GossipStrategy (least-loaded member using gossip heartbeat)    │
└──────────────────────────────────────────────────────────────────┘
```

### Key Design Decisions

**1. The placement actor is a shared component, not an interface.**

A single `placementActor` struct in `cluster/` with behavior customized via functional options (persistence callbacks, rebalance flag). This avoids interface ceremony — all providers need the same core behavior, just with different storage callbacks.

If a provider needs fundamentally different placement behavior, it can implement its own actor that handles `ActivationRequest`/`ActivationResponse` — the identity lookup doesn't care about the actor's implementation, only the message protocol. But the default covers all current providers.

**2. Persistence is a callback, not an interface.**

The placement actor receives two callbacks:
- `PersistActivation(ctx context.Context, ci *ClusterIdentity, pid *actor.PID) error` — store the activation after spawn. A `LockNotHeld` sentinel error signals that the spawn lock was stolen (no retry, poison immediately). Other errors trigger retry.
- `RemoveActivation(ctx context.Context, ci *ClusterIdentity, pid *actor.PID) error` — clean storage when actor terminates. Errors are logged but do not prevent cleanup of the local map.

Each identity lookup provides its own implementations:
- natskv: writes to NATS KV bucket
- natsstream: publishes to NATS stream
- `IdentityStorageLookup`: calls `StorageLookup.StoreActivation()`
- disthash: no-op (in-memory only, no external storage)

**3. Spawn verification uses the existing `CanSpawnIdentity` pattern from .NET.**

Add an optional `CanSpawnIdentity func(ctx context.Context, identity string) (bool, error)` field to `cluster.Kind`. The signature accepts context to allow async implementations (e.g., database lookups to validate an identity). When set, the placement actor calls it inside a goroutine and uses `ReenterAfter` on the result, matching the .NET `SpawnVerificationHelper` pattern. If it returns false, the response has `InvalidIdentity: true` and no actor is created. If it returns an error, the response has `Failed: true`.

**4. ReenterAfter for persistence.**

The placement actor uses `ctx.ReenterAfter()` to wait for persistence to complete before responding with the PID. This prevents the identity lookup from returning a PID that isn't yet stored — eliminating a window where the actor is alive but invisible to other nodes.

If persistence fails after retries, the spawned actor is poisoned and the response has `Failed: true`.

**5. Topology rebalance is opt-in.**

Only disthash needs rebalancing on topology changes (because it uses rendezvous hashing for ownership). Storage-backed providers don't rebalance — ownership lives in the storage layer, and stale activations are cleaned up via `RemovePid` / topology events.

The placement actor accepts `WithRebalanceStrategy(fn)` to enable rebalancing. When set, it receives `ClusterTopology` messages and poisons actors whose owner changed.

**6. Remote placement via ActivatorProxy.**

All storage-backed identity providers (natskv, natsstream, Redis, Postgres, NATS KV) support remote placement. When `Get()` acquires the spawn lock, it selects a target member via the member strategy and sends the `ActivationRequest` to that member's `$proxy-activator`. The proxy forwards to the local `$placement-activator`, which spawns the actor and persists the activation.

Disthash does not use remote placement — it uses its own rendezvous-hash-based partition model.

The `IdentityActivatorProxy` (`$proxy-activator`) is a thin actor on each non-client member that receives remote `ActivationRequest` messages and forwards them to the local `$placement-activator`. It exists as a separate actor to provide a stable well-known name for cross-node communication, decoupled from the placement actor's internal lifecycle.

The .NET proxy also handles `ProxyActivationRequest` (which includes a `replaced_activation` field for stale-PID replacement). The Go proto already defines `ProxyActivationRequest` in `cluster.proto`. The Go proxy handles both:
- `ActivationRequest` — new activation, forwarded to local `$placement-activator`
- `ProxyActivationRequest` — stale PID replacement: the proxy calls `RemovePid` on the stale PID, then forwards the activation request to the local `$placement-activator`. If `replaced_activation` is nil, it behaves identically to `ActivationRequest`.

**7. Duplicate-request coalescing (worker pattern).**

Matching the .NET `IdentityStorageWorker`, concurrent `Get()` calls for the same identity are coalesced. The first request performs the full lock-acquire-spawn flow. Subsequent concurrent requests for the same identity are queued and all receive the same response when the first completes.

Implementation: each identity lookup maintains a `sync.Mutex`-protected in-progress map (`ClusterIdentity → *inflight`). The `inflight` struct contains:

```go
type inflight struct {
    done chan struct{} // closed when activation completes
    pid  *actor.PID   // result (nil on failure)
    err  error        // error (nil on success)
}
```

The first caller creates the `inflight` entry and performs the activation. Subsequent callers find the existing entry and block on `<-done`. When the first caller finishes, it sets `pid`/`err` and closes `done`, unblocking all waiters. The `sync.Mutex` protects the map's read-modify-write pattern (check-if-exists, create-if-not).

If the first caller panics, a deferred recover ensures `done` is closed with an error so waiters are never permanently blocked.

**8. Per-kind member strategy configuration.**

Different kinds can use different placement strategies. This is configured per-kind via `WithActivatorStrategy()` on the kind definition, or globally via `WithDefaultActivatorStrategy()` on the cluster config.

**Relationship to existing `MemberStrategy` interface:** The existing `MemberStrategy` interface in `cluster/member_strategy.go` handles partition-based routing (rendezvous hashing for `GetPartition`) and is used by disthash and the member list. The new `ActivatorStrategy` interface is specifically for placement decisions when spawning new grains via storage-backed identity providers. The two interfaces coexist — `MemberStrategy` continues to handle partition routing, `ActivatorStrategy` handles activation placement. They are not interchangeable.

**API change:** The existing `NewKind(kind string, props *actor.Props) *Kind` constructor does not accept options. We add `Kind.WithActivatorStrategy(s ActivatorStrategy) *Kind` as a builder method (returning `*Kind` for chaining), matching the existing `Kind.WithMemberStrategy()` pattern. This is not a breaking change — the constructor signature is unchanged.

```go
// ActivatorStrategy selects which member should host a grain activation.
// This is distinct from the existing MemberStrategy interface which handles
// partition routing. ActivatorStrategy is specifically for placement decisions
// when spawning new grains via storage-backed identity providers.
type ActivatorStrategy interface {
    GetActivator(ci *ClusterIdentity, senderAddress string) *Member
    AddMember(member *Member)
    RemoveMember(member *Member)
    Close() // Cleanup (unsubscribe from events, etc.)
}
```

## Member Strategies

### RoundRobinStrategy (default)

Simple cyclic selection across all members that support the kind. Uses an atomic counter with modulo. This is the baseline strategy — predictable, no external dependencies.

### LocalAffinityStrategy

Prefers the local node (the node that received the `Get()` request) if it supports the kind. Falls back to round-robin if the local node doesn't support the kind or is unavailable. This optimizes for same-node message delivery, reducing network hops.

### RendezvousStrategy

Deterministic hash-based placement using rendezvous (highest random weight) hashing on the `ClusterIdentity`. The same identity always maps to the same member (given stable topology), providing cache locality. Different from disthash because it's used with storage-backed providers — the hash influences *where* to place, but the storage layer is the source of truth for *what* is placed.

### GossipStrategy (capacity-aware, least-loaded)

Selects the member with the **fewest active grains for the requested kind**, using per-kind actor counts propagated via the existing gossip heartbeat infrastructure.

**How it works:**
1. The Go cluster already gossips `MemberHeartbeat` with `ActorStatistics` containing per-kind actor counts (via `cluster.MemberStats()`). This infrastructure is already built and working.
2. `GossipStrategy` subscribes to gossip heartbeat updates and maintains a `sync.RWMutex`-protected map of `memberId → perKindCounts`. The `RWMutex` is needed because gossip updates write to the map concurrently with `GetActivator()` reading and iterating it. A read lock is held during the min-finding iteration to provide a consistent snapshot.
3. `GetActivator()` filters to members that support the kind, then selects the one with the lowest count. Falls back to round-robin if no gossip data is available yet (cold start).

**Lifecycle:** The strategy subscribes to the EventStream during construction. The `ActivatorStrategy` interface includes a `Close()` method for cleanup. `GossipStrategy.Close()` unsubscribes from the EventStream. Strategies are created per-kind and closed when the identity lookup shuts down. Strategies that don't need cleanup (round-robin, rendezvous) have no-op `Close()` methods.

**User-extensible scoring:** In addition to the default per-kind-count scoring, the strategy accepts an optional `ScoreMember func(member *Member, kindCounts map[string]int64) float64` callback. This allows users to incorporate custom signals (e.g., weighting by available memory, CPU headroom, or business-specific capacity metrics). Members with the lowest score are preferred.

Custom metrics can be propagated by having each node set additional gossip state keys (via `cluster.Gossip.SetState()`), which the scoring function reads from the member's gossip data.

### Strategy Configuration

```go
// Per-kind strategy via builder method (non-breaking, existing NewKind unchanged):
cluster.Configure(name, provider, lookup, remoteCfg,
    cluster.WithKinds(
        cluster.NewKind("OrderGrain", orderProps).
            WithActivatorStrategy(cluster.NewGossipStrategy()),
        cluster.NewKind("CacheGrain", cacheProps).
            WithActivatorStrategy(cluster.NewRendezvousStrategy()),
        cluster.NewKind("SessionGrain", sessionProps).
            WithActivatorStrategy(cluster.NewLocalAffinityStrategy()),
    ),
    // Default strategy for kinds that don't specify one:
    cluster.WithDefaultActivatorStrategy(cluster.NewRoundRobinStrategy()),
)
```

## Proto Message Update

Add `invalid_identity` to `ActivationResponse`:

```protobuf
message ActivationResponse {
  actor.PID pid = 1;
  bool failed = 2;
  uint64 topology_hash = 3;
  bool invalid_identity = 4;
}
```

## Placement Actor Configuration

```go
type PlacementConfig struct {
    // PersistActivation stores the activation after spawn. Called inside
    // ReenterAfter — the placement actor does not respond until this
    // completes (or fails after retries). Nil means no persistence (disthash).
    // A LockNotHeld sentinel error signals the lock was stolen — no retry.
    PersistActivation func(ctx context.Context, ci *ClusterIdentity, pid *actor.PID) error

    // RemoveActivation cleans storage when an actor terminates. Called
    // from the Terminated handler. Errors are logged but do not block
    // cleanup of the local tracking map. Nil means no cleanup needed.
    RemoveActivation func(ctx context.Context, ci *ClusterIdentity, pid *actor.PID) error

    // RebalanceOnTopology, when set, is called on ClusterTopology events.
    // It receives the new topology and returns the set of ClusterIdentity
    // keys that should be rebalanced (poisoned and re-activated elsewhere).
    // Nil means no rebalancing (storage-backed providers).
    RebalanceOnTopology func(topology *ClusterTopology, actors map[string]*GrainMeta) []string

    // PersistenceRetries is how many times to retry PersistActivation
    // before giving up and poisoning the actor. Default: 3.
    PersistenceRetries int

    // PersistenceRetryDelay is the base delay between persistence retries.
    // Default: 50ms.
    PersistenceRetryDelay time.Duration

    // ShutdownTimeout is the maximum time to wait for all local grains to
    // stop during the Stopping handler. If exceeded, remaining grains are
    // force-stopped. Default: 30s.
    ShutdownTimeout time.Duration
}
```

## Termination Mechanism

The placement actor uses `*actor.Terminated` messages (explicit watch) as the primary cleanup trigger, NOT the `ActivationTerminating` EventStream subscription.

Grains are spawned via `Root.SpawnNamed(props, kind+"/"+identity)` for backward compatibility with existing PID formats in identity stores. The placement actor explicitly watches each spawned PID via `ctx.Watch(pid)`. When a grain stops for any reason, the placement actor receives `*actor.Terminated` and cleans up the local map + calls `RemoveActivation`.

The `handleStopped` middleware continues to publish `ActivationTerminating` for PID cache cleanup and `GrainDeactivated` events — these are orthogonal to the placement actor's local cleanup. Both mechanisms are intentionally redundant for defense-in-depth: the placement actor handles storage cleanup, the middleware handles cache cleanup and event publishing.

## PID Naming Strategy

**Decision: Use `ctx.Watch(pid)` with `Root.SpawnNamed` for backward compatibility.** The placement actor spawns grains with the existing naming convention (`kind/identity`) and explicitly watches them. This avoids a breaking PID format change and ensures rolling upgrades work without any activation record migration.

## Stale Member Validation

When the identity lookup's `Get()` path retrieves an existing activation from storage, it validates that the owning member is still in the cluster before returning the PID. If the member has departed, the stale activation records are cleaned up immediately (call `RemoveMemberId`) rather than returning a PID that will dead-letter.

This matches the .NET `IdentityStorageWorker.ValidateAndMapToPid()` behavior. Implementation: a shared utility function `ValidateActivationMember(memberList, memberId) bool` that identity lookups call in their `Get()` path.

## Well-Known Actor Names

- `$placement-activator` — the placement actor on each non-client member. Handles `ActivationRequest` messages, spawns and tracks local grains.
- `$proxy-activator` — the activator proxy on each non-client member. Receives remote `ActivationRequest` messages from other nodes and forwards them to the local `$placement-activator`.

Only spawned on non-client members. Clients resolve identities by talking to remote members.

## Lifecycle

**Setup (non-client only):**
```
IdentityLookup.Setup(cluster, kinds, isClient=false)
  → Spawn $placement-activator with provider-specific config
  → Spawn $proxy-activator (forwards remote requests to local placement actor)
  → Placement actor watches each grain it spawns (via ctx.Watch)
```

**Get (activation — storage-backed providers):**
```
IdentityLookup.Get(ci)
  → Check PID cache
  → Check existing activation in store
    → Validate owning member is alive (stale member check)
    → If stale, clean up and continue
  → If coalescing: check in-progress map, wait if another request is in flight
  → Acquire lock
  → Select target member via ActivatorStrategy.GetActivator()
  → If target is local: send ActivationRequest to local $placement-activator
  → If target is remote: send ActivationRequest to target's $proxy-activator
  → Placement actor: spawn, verify (CanSpawnIdentity), persist (ReenterAfter), track, respond
  → Cache PID, notify coalesced waiters, return PID
```

**Get (activation — disthash, unchanged):**
```
Disthash uses its own partition-based activation flow.
The shared placement actor handles lifecycle (spawn, track, poison)
but activation routing uses rendezvous hashing, not ActivatorStrategy.
```

**Actor terminates naturally:**
```
Grain stops (passivation, crash, etc.)
  → Placement actor receives *actor.Terminated (via Watch)
  → Removes from local tracking map
  → Calls RemoveActivation callback to clean storage (errors logged)
  → handleStopped middleware (separately) clears PID cache, publishes GrainDeactivated
```

**Node shutdown:**
```
Cluster.Shutdown(graceful=true)
  → IdentityLookup.Shutdown()
    → Stops placement actor (poison)
      → Placement actor Stopping handler:
        → Set stopping flag (reject new ActivationRequests with Failed: true)
        → For each tracked local grain:
          → SetDeactivationReason(pid, DeactivationReasonShutdown)
          → PoisonFuture(pid) → wait (up to ShutdownTimeout)
        → All grains stop gracefully (or force-stop on timeout)
    → Stop proxy activator
    → Remove member records from storage
```

**Safety invariant: placement actor only poisons PIDs that it spawned locally.** It never touches remote PIDs. The local tracking map (`ClusterIdentity → PID`) only contains actors spawned by this node's placement actor, so iterating it on shutdown is safe by construction.

## Edge Cases

**Spawn-then-immediate-crash:** If a grain crashes immediately after spawn but before `PersistActivation` completes (inside `ReenterAfter`), the placement actor receives `*actor.Terminated` while the `ReenterAfter` continuation is pending. Since the actor mailbox processes messages sequentially, the termination message is queued behind the persistence continuation. When the continuation runs, it either succeeds (and the PID is already dead — the next lookup will get a dead-letter and trigger `RemovePid`) or fails (and the actor is already dead — the poison is a no-op). Either way, the local map is cleaned up when `*actor.Terminated` is processed.

**ActivationRequest during Stopping:** If an `ActivationRequest` arrives while the placement actor is in its `Stopping` handler, it responds with `Failed: true`. The placement actor sets a `stopping` flag when entering the Stopping handler and checks it at the top of `ActivationRequest` handling. Note: this flag does not need synchronization because the placement actor is a protoactor actor with sequential message processing — messages are processed one at a time through the mailbox.

**PersistActivation callback panics:** The `ReenterAfter` continuation recovers from panics in the persistence callback, poisons the spawned actor, and responds with `Failed: true`.

**Remote placement to unavailable member:** If the target member selected by the strategy is unavailable (dead-letter on `ActivationRequest`), the identity lookup removes the lock and retries. The next retry selects a different member (the unavailable one should be removed from the member list by topology events).

**GossipStrategy cold start:** When a node first joins, it has no gossip data about other members' load. The strategy falls back to round-robin until the first gossip heartbeat cycle propagates counts.

**GossipStrategy stale data:** When a member departs, `RemoveMember()` is called on the strategy, which removes the member's gossip data from the internal map. This prevents stale load data from influencing placement decisions after a member is gone.

**Coalescing context cancellation:** If a caller's context is cancelled while waiting on a coalesced result, the waiter returns immediately with the context error. The first caller continues its activation work regardless — the result is still stored for other waiters.

**Proxy receives request during local placement actor shutdown:** The proxy forwards the request to the local `$placement-activator`, which rejects it with `Failed: true` (via the stopping flag). The proxy returns this response to the remote caller, which retries with a different member.

## Observability

The placement actor, proxy, and identity lookup `Get()` path emit OpenTelemetry metrics matching the .NET `IdentityMetrics` pattern:

- `cluster.activation_request.received` — counter, per-kind, on placement actor
- `cluster.activation_request.duration` — histogram, per-kind, spawn-to-response time
- `cluster.lock_acquire.duration` — histogram, per-kind, in identity lookup Get() path
- `cluster.identity.lookup.duration` — histogram, per-kind, full Get() duration
- `cluster.placement.remote` — counter, per-kind, when activation is sent to a remote member
- `cluster.placement.local` — counter, per-kind, when activation is handled locally

## Identity Lookup Changes

### IdentityStorageLookup (fixes Redis, Postgres, NATS KV)

Current `spawnActivation` method (direct `SpawnNamed`) is replaced with sending `ActivationRequest` to the local or remote placement actor (selected by strategy). The placement actor calls back into the storage backend via the `PersistActivation` callback, which calls `StorageLookup.StoreActivation()`.

`Get()` gains: PID cache check, stale member validation, duplicate-request coalescing, member strategy selection, remote placement support.

`Shutdown()` changes from just `RemoveMemberId()` to: stop placement actor (graceful poison of all local grains) → stop proxy activator → then `RemoveMemberId()`.

### natskv

`spawnActivation` replaced with `ActivationRequest` to placement actor (local or remote via strategy). The `PersistActivation` callback calls `storeActivation` (which does the CAS update to NATS KV). The `RemoveActivation` callback deletes the KV entry and member tracking.

`Get()` gains the same improvements as `IdentityStorageLookup`: stale member validation, coalescing, strategy selection, remote placement.

`Shutdown()` stops the placement actor and proxy first, then calls `removeMemberID`.

The `ErrNameExists` recovery path in `spawnActivation` becomes unnecessary — the placement actor prevents duplicate spawns via its in-flight tracking set.

### natsstream

Same pattern as natskv but with stream-specific persistence callbacks.

### disthash

The existing `placementActor` is replaced with the shared one, configured with:
- `PersistActivation: nil` (no external storage)
- `RemoveActivation: nil` (cleanup is via `ActivationTerminated` broadcast)
- `RebalanceOnTopology`: rendezvous hash function that determines ownership changes

Disthash does NOT use `ActivatorStrategy` or remote placement — it retains its own partition-based routing where the identity always maps to a deterministic owner via rendezvous hash.

The existing `Manager` is simplified — it no longer needs to implement the placement actor, just configure and manage the shared one.

## Testing Strategy

### Placement actor unit tests (Sub-project 1)
- Spawn and persist on ActivationRequest
- Duplicate request returns existing PID
- Persistence failure → poison actor, respond Failed
- Persistence retry succeeds on second attempt
- Persistence LockNotHeld error → no retry, poison immediately
- Unknown kind → respond Failed
- Actor Terminated → RemoveActivation callback invoked, local map cleaned
- Stopping → all local actors poisoned with DeactivationReasonShutdown
- **Stopping only poisons local actors, not remote PIDs**
- Stopping timeout → force-stop remaining grains
- CanSpawnIdentity approved → normal spawn
- CanSpawnIdentity rejected → respond InvalidIdentity
- CanSpawnIdentity with context/error (async) → verified via ReenterAfter
- Topology rebalance poisons actors whose owner changed (when enabled)
- **Spawn-then-immediate-crash** → persistence continuation + Terminated both handled correctly
- **ActivationRequest during Stopping** → respond Failed
- **PersistActivation panic** → recover, poison actor, respond Failed

### ActivatorProxy unit tests (Sub-project 1)
- Forwards ActivationRequest to local placement actor, returns response
- Forwards ProxyActivationRequest, calls RemovePid on stale PID before forwarding
- ProxyActivationRequest with nil replaced_activation behaves like ActivationRequest
- Request while local placement actor is stopping → returns Failed
- Unknown message type → ignored (no panic)
- Timeout on forwarded request → returns Failed

### Member strategy unit tests (Sub-project 1)
- RoundRobinStrategy: cycles through members
- LocalAffinityStrategy: prefers local, falls back to round-robin
- RendezvousStrategy: deterministic mapping, stable under member changes
- GossipStrategy: selects least-loaded member, falls back on cold start
- GossipStrategy with custom ScoreMember callback
- GossipStrategy: stale member data removed on RemoveMember
- GossipStrategy: Close() unsubscribes from EventStream
- Per-kind strategy configuration works correctly
- All strategies handle member add/remove correctly
- All strategies return nil when no members available

### Duplicate-request coalescing tests (Sub-project 2)
- Concurrent Get() for same identity → only one spawn, all callers get same PID
- First request fails → all waiters get nil/error
- Different identities are not coalesced
- Context cancellation while waiting → waiter returns context error, activation continues
- Panic in first request → deferred recovery closes done channel, waiters unblocked with error

### Stale member validation tests (Sub-project 2)
- Existing activation with alive member → return PID
- Existing activation with departed member → clean up, re-activate

### Remote placement tests (Sub-project 2)
- ActivationRequest to remote member → grain spawns on remote, PID returned
- Remote member unavailable → retry with different member
- Strategy selects local → no remote hop

### Per-provider integration tests (Sub-projects 2-5)
- Full Get() → strategy → placement actor → spawn → persist → return PID flow
- Remote placement: grain spawns on selected member, not necessarily lock-holder
- Shutdown → placement actor stops → local grains receive Stopping
- Existing provider test suites continue to pass

### Cross-node safety tests (Sub-project 6)
- **Two-node shutdown: only local grains poisoned** — start 2-node cluster, activate grains on both, shut down node 1, verify node 2's grains are alive and responding
- **Two-node topology rebalance: only local grains affected** — add/remove a node, verify only the local node's placement actor poisons its own affected grains
- **Two-node remote placement** — verify grains can be spawned on a node different from the one that received the Get() request
- **GossipStrategy multi-node** — verify grains distribute according to load across nodes
- Placement conformance suite validating any placement actor implementation

## Sub-project Decomposition

### Sub-project 1a: Foundation — Proto Update + Kind Changes + Utilities
- Add `invalid_identity` to `ActivationResponse` proto, regenerate
- Add `CanSpawnIdentity` field to `cluster.Kind`, with `WithCanSpawnIdentity()` builder method
- Add `WithActivatorStrategy()` builder method to `cluster.Kind`
- Add `WithDefaultActivatorStrategy()` cluster config option
- Add `LockNotHeld` sentinel error
- Add `ValidateActivationMember` utility function
- Unit tests for all of the above

### Sub-project 1b: ActivatorStrategy Interface + Four Implementations
- Define `ActivatorStrategy` interface
- Implement `RoundRobinStrategy`
- Implement `LocalAffinityStrategy`
- Implement `RendezvousStrategy`
- Implement `GossipStrategy` (wired to existing gossip heartbeat data)
- Per-kind strategy manager (dispatches to per-kind strategy)
- Full unit test suite for all strategies

### Sub-project 1c: Shared Placement Actor + Activator Proxy
- Implement `cluster/placement.go` — the shared default placement actor
- Implement `cluster/activator_proxy.go` — the `$proxy-activator`
- Full unit test suite for placement actor and proxy
- No existing identity lookups touched

### Sub-project 2: Integrate into IdentityStorageLookup
- Refactor `Setup()` to spawn placement actor and proxy with `StorageLookup`-backed persistence callbacks
- Refactor `Get()` to: validate stale members, coalesce duplicate requests, select target member via strategy, send `ActivationRequest` to local or remote placement actor
- Refactor `Shutdown()` to stop placement actor and proxy, close strategies, then remove member
- Fixes Redis, Postgres, and NATS KV identity lookups automatically
- Run conformance suite + provider integration tests

### Sub-project 3: Integrate into natskv
- Refactor `Setup()` to spawn placement actor and proxy
- Refactor `Get()` with stale member validation, coalescing, strategy selection, remote placement
- Refactor `Shutdown()` to stop placement actor and proxy
- Remove `ErrNameExists` recovery (now handled by placement actor)
- Update tests, run full suite

### Sub-project 4: Integrate into natsstream
- Same as Sub-project 3

### Sub-project 5: Refactor disthash
- Replace custom `placementActor` with shared one (rebalance enabled, no persistence, no remote placement)
- Simplify `Manager`
- Verify all existing disthash behavior preserved

### Sub-project 6: Conformance + Cross-Node Safety
- `PlacementConformanceSuite`
- Multi-node integration tests (shutdown safety, rebalance safety, remote placement, gossip strategy distribution)
- Final full test pass across all providers
