# Grain Introspection & Registry Design

**Date:** 2026-03-10
**Status:** Approved

## Problem

The protoactor-go cluster can count virtual actors per kind but cannot list them. The `ProcessRegistry.LocalPIDs` contains all PIDs (system actors, not just grains) with no structured metadata. There is no activation event on the EventStream (only termination), so users cannot build a live registry by subscribing.

## Goals

1. Grain enumeration — list active grains locally and cluster-wide
2. Lifecycle events — `GrainActivated` and `GrainDeactivated` with reason
3. Per-grain metrics — opt-in `LastMessageAt` and `MessageCount` tracking
4. Member stats — convenience API over existing gossip `ActorStatistics`
5. Implementations for all 7 identity lookup backends

## Non-Goals

- CPU/Memory stats in MemberStats (use external monitoring)
- MessagesPerSec windowed rate (use OpenTelemetry)
- Full mailbox diagnostics (stall detection, last error, processing message)
- GrainStatus enum (Active/Idle/Failing state machine)
- Gossip-based grain directory (payload too large, stale data)

---

## Core Types

### GrainInfo

```go
// GrainInfo describes an active grain activation.
type GrainInfo struct {
    Identity      string
    Kind          string
    PID           *actor.PID
    MemberID      string
    ActivatedAt   time.Time    // zero if unknown (e.g., remote grains from storage)
    LastMessageAt time.Time    // zero unless WithGrainMetrics() enabled
    MessageCount  int64        // zero unless WithGrainMetrics() enabled
}
```

### GrainRegistry

Lives on `Cluster`. Delegates to the `IdentityLookup` if it implements `GrainEnumerator`.

```go
type GrainRegistry struct {
    cluster *Cluster
}

func (c *Cluster) GrainRegistry() *GrainRegistry

// Count and CountByKind always work (use existing ActivatedKind atomic counters).
func (r *GrainRegistry) Count() int
func (r *GrainRegistry) CountByKind() map[string]int

// These require GrainEnumerator support from the IdentityLookup.
// Returns ErrEnumerationNotSupported if not implemented.
func (r *GrainRegistry) All() ([]*GrainInfo, error)
func (r *GrainRegistry) ByKind(kind string) ([]*GrainInfo, error)
func (r *GrainRegistry) ByMember(memberID string) ([]*GrainInfo, error)
func (r *GrainRegistry) Get(identity, kind string) (*GrainInfo, bool, error)
```

### GrainEnumerator (opt-in on IdentityLookup)

```go
type GrainEnumerator interface {
    ListGrains() ([]*GrainInfo, error)
    ListGrainsByKind(kind string) ([]*GrainInfo, error)
    ListGrainsByMember(memberID string) ([]*GrainInfo, error)
}
```

### StorageGrainEnumerator (opt-in on StorageLookup)

```go
type StorageGrainEnumerator interface {
    ListActivations() ([]*StoredActivationInfo, error)
    ListActivationsByMember(memberID string) ([]*StoredActivationInfo, error)
}

type StoredActivationInfo struct {
    Identity string
    Kind     string
    Pid      string    // "address/id" format
    MemberID string
}
```

The `IdentityStorageLookup` adapter detects `StorageGrainEnumerator` and implements `GrainEnumerator` by delegating and converting results.

---

## Lifecycle Events

### GrainActivated

Published from `handleStarted` middleware in `config.go`, after the actor is started and `ClusterInit` is sent.

```go
type GrainActivated struct {
    ClusterIdentity *ClusterIdentity
    PID             *actor.PID
}
```

### GrainDeactivated

Published from `handleStopped` middleware in `config.go`, alongside the existing `ActivationTerminating` event for backward compatibility.

```go
type GrainDeactivated struct {
    ClusterIdentity *ClusterIdentity
    PID             *actor.PID
    Reason          DeactivationReason
}

type DeactivationReason int
const (
    DeactivationReasonUnknown          DeactivationReason = iota
    DeactivationReasonPassivation      // idle timeout via passivation plugin
    DeactivationReasonShutdown         // cluster shutdown / actor stopped
    DeactivationReasonTopologyChange   // rebalancing due to member join/leave
)
```

**Deactivation reason detection:**
- `DeactivationReasonTopologyChange`: Set by the placement actor (disthash) when it poisons actors during `onClusterTopology`. Requires injecting a context value or a tagged stop message.
- `DeactivationReasonPassivation`: Detected when the passivation plugin's timer fires. Requires the passivation plugin to set a context marker before calling `ctx.Stop(ctx.Self())`.
- `DeactivationReasonShutdown`: When `Cluster.Shutdown()` is called, set a cluster-level flag. The `handleStopped` middleware reads it.
- `DeactivationReasonUnknown`: Default fallback.

**Implementation approach for reason tracking:**
Use a context extension to carry the deactivation reason. The placement actor, passivation plugin, and cluster shutdown path each set the reason before stopping the actor. The `handleStopped` middleware reads it.

```go
// DeactivationReasonKey is the extension key for deactivation reason.
var deactivationReasonExtensionID = extensions.NextExtensionID()

type deactivationReasonHolder struct {
    Reason DeactivationReason
}

func SetDeactivationReason(ctx actor.Context, reason DeactivationReason)
func GetDeactivationReason(ctx actor.Context) DeactivationReason
```

---

## Opt-In Per-Grain Metrics

Enabled via `WithGrainMetrics()` config option on the cluster config.

```go
func WithGrainMetrics() ConfigOption
```

When enabled, adds a receiver middleware that:
1. On each message (not system messages), atomically updates `LastMessageAt` and increments `MessageCount` on a shared metrics store.
2. The metrics store is keyed by `ClusterIdentity.AsKey()`.
3. On grain deactivation, the entry is removed.

**Storage:** A `sync.Map` or sharded map on the `Cluster` struct, keyed by identity key.

```go
type grainMetricsEntry struct {
    lastMessageAt atomic.Int64  // unix nano
    messageCount  atomic.Int64
}
```

The `GrainRegistry` merges these metrics into `GrainInfo` when returning results.

---

## MemberStats

```go
type MemberStats struct {
    MemberID   string
    Address    string
    GrainCount int64
    ByKind     map[string]int64
}

func (c *Cluster) MemberStats() ([]MemberStats, error)
```

Reads `MemberHeartbeat.ActorStatistics.ActorCount` from gossip state for the `HeartbeatKey`. No new gossip data is needed — this is a convenience wrapper.

---

## Per-Implementation Enumeration

### 1. disthash (placement actor)

Add message types to the placement actor:

```go
// Internal messages (not proto, just Go types)
type ListGrainsRequest struct{}
type ListGrainsResponse struct {
    Grains []*GrainInfo
}
```

The `placementActor.Receive` handles `ListGrainsRequest` by iterating `p.actors` and responding with `ListGrainsResponse`.

Enrich `GrainMeta` with `ActivatedAt time.Time`, set at spawn time in `onActivationRequest`.

The `disthash.IdentityLookup` implements `GrainEnumerator`:
- `ListGrains()` — sends `ListGrainsRequest` to the placement actor PID
- `ListGrainsByKind(kind)` — filters response client-side
- `ListGrainsByMember(memberID)` — only returns results if memberID matches local member (placement actor only tracks local grains)

### 2. IdentityStorageLookup adapter

If the underlying `StorageLookup` implements `StorageGrainEnumerator`, the adapter implements `GrainEnumerator` by:
1. Calling `ListActivations()` or `ListActivationsByMember()`
2. Converting `StoredActivationInfo` to `GrainInfo` (parsing the `Pid` string into `*actor.PID`)

### 3. Redis (StorageGrainEnumerator)

**ListActivations():**
- `SCAN 0 MATCH {cluster}:ci:* COUNT 100` to find all identity keys
- Pipeline `HGETALL` on each key
- Filter for completed activations (pid field non-empty)
- Parse key to extract kind and identity (key format: `{cluster}:ci:{kind}/{identity}`)

**ListActivationsByMember(memberID):**
- `SMEMBERS {cluster}:mb:{memberID}` to get identity keys
- Pipeline `HGETALL` on each key
- Filter and parse as above

### 4. Postgres (StorageGrainEnumerator)

**ListActivations():**
```sql
SELECT key, pid_id, pid_address, member_id
FROM {table}
WHERE pid_id != ''
```
Parse `key` (format: `{kind}/{identity}`) to extract kind and identity.

**ListActivationsByMember(memberID):**
```sql
SELECT key, pid_id, pid_address, member_id
FROM {table}
WHERE pid_id != '' AND member_id = $1
```

### 5. InMemory (StorageGrainEnumerator)

**ListActivations():**
Iterate `activations` map under lock. Parse key (format: `{kind}/{identity}`) to extract kind and identity.

**ListActivationsByMember(memberID):**
Same iteration, filter by `MemberID`.

### 6. NATS KV identitylookup (StorageGrainEnumerator)

Same as InMemory approach but reads from the NATS KV storage.

**ListActivations():**
- Read all member IDs from `memberTracker` KV bucket using `Keys()`
- For each member, read the `memberRecord` to get identity keys
- For each identity key, read from `identities` KV bucket and unmarshal `activationRecord`
- Filter for completed activations (PidID non-empty)
- Parse key (format: `{kind}.{identity}`, dots replaced from slashes) to extract kind and identity

**ListActivationsByMember(memberID):**
- Read member record from `memberTracker`
- Look up each key from `identities` bucket

### 7. natskv provider (GrainEnumerator directly)

Similar to NATS KV identitylookup. Implements `GrainEnumerator` directly:
- Read `memberTracker` KV bucket
- Look up identity records from `identities` KV bucket
- Convert to `GrainInfo`

### 8. natsstream provider (GrainEnumerator directly)

Uses in-memory `memberKeys` map:
- Lock `memberKeysMu`, copy the map
- For each subject key, call `identityStream.GetLastMsgForSubject()` to read the activation record
- Convert to `GrainInfo`
- Note: Only reflects local node's knowledge (documented limitation)

---

## Fan-Out Cluster-Wide Query

The `GrainRegistry` supports cluster-wide enumeration by fanning out to all members.

For **disthash**, the placement actor only has local data. Cluster-wide enumeration requires sending `ListGrainsRequest` to each member's placement actor. The `GrainRegistry.All()` method:
1. Gets current members from `MemberList`
2. For each member, constructs the placement actor PID (known naming convention)
3. Sends `ListGrainsRequest` via `RequestFuture` with timeout
4. Collects and merges responses

For **storage-backed lookups**, `ListActivations()` already returns cluster-wide data from the central store.

For **natskv/natsstream**, the KV/stream is shared so enumeration is already cluster-wide.

---

## Configuration

```go
// WithGrainMetrics enables per-grain metrics tracking (LastMessageAt, MessageCount).
// This adds a small overhead to each message processed by a grain.
func WithGrainMetrics() ConfigOption

// No configuration needed for GrainRegistry, events, or MemberStats — they are always available.
```

---

## Testing Strategy

### Conformance Suite Extension

Add optional `StorageGrainEnumerator` tests to `StorageConformanceSuite`:

```go
type EnumeratorConformanceSuite struct {
    NewStorage func() interface{ cluster.StorageLookup; StorageGrainEnumerator }
    Cleanup    func()
}
```

Tests:
- ListActivations returns stored activations
- ListActivationsByMember filters correctly
- ListActivations excludes locked-but-not-activated entries
- ListActivations reflects removals
- ListActivationsByMember after RemoveMemberId returns empty

### Unit Tests

- `GrainRegistry` with mock IdentityLookup (with and without GrainEnumerator)
- `GrainActivated` event fires on grain start
- `GrainDeactivated` event fires on grain stop with correct reason
- Backward compatibility: `ActivationTerminating` still fires
- `MemberStats` parsing from gossip state
- Opt-in grain metrics middleware updates counters correctly
- Grain metrics entries are cleaned up on deactivation

### Integration Tests (per backend)

- disthash: spawn grains, enumerate, verify list matches spawned actors
- Redis: store activations, enumerate, verify
- Postgres: store activations, enumerate, verify
- NATS KV: store activations, enumerate, verify
- natskv provider: spawn grains, enumerate, verify
- natsstream provider: spawn grains, enumerate, verify

### Race Detection

All tests run with `-race` flag. Key concurrent paths:
- Grain metrics `sync.Map` / atomic operations
- `GrainRegistry` reads vs placement actor mutations
- Gossip state reads for `MemberStats`

---

## File Changes Summary

### New Files
- `cluster/grain_registry.go` — GrainRegistry type and methods
- `cluster/grain_events.go` — GrainActivated, GrainDeactivated, DeactivationReason
- `cluster/grain_metrics.go` — opt-in per-grain metrics store and middleware
- `cluster/member_stats.go` — MemberStats type and Cluster.MemberStats()
- `cluster/grain_registry_test.go` — unit tests
- `cluster/grain_events_test.go` — event tests
- `cluster/grain_metrics_test.go` — metrics tests
- `cluster/member_stats_test.go` — MemberStats tests

### Modified Files
- `cluster/identity_lookup.go` — add GrainEnumerator, StorageGrainEnumerator, StoredActivationInfo
- `cluster/config.go` — handleStarted publishes GrainActivated; handleStopped publishes GrainDeactivated
- `cluster/config_opts.go` — add WithGrainMetrics()
- `cluster/cluster.go` — add GrainRegistry() method, grain metrics store field
- `cluster/kind.go` — (no changes needed, counts already work)
- `cluster/identitylookup/disthash/placement_actor.go` — add ActivatedAt to GrainMeta, handle ListGrainsRequest
- `cluster/identitylookup/disthash/identity_lookup.go` — implement GrainEnumerator
- `cluster/identitylookup/storage/identity_storage_lookup.go` — implement GrainEnumerator via StorageGrainEnumerator delegation
- `cluster/identitylookup/redis/redis_identity.go` — implement StorageGrainEnumerator
- `cluster/identitylookup/postgres/postgres_identity.go` — implement StorageGrainEnumerator
- `cluster/identitylookup/inmemory_storage.go` — implement StorageGrainEnumerator
- `cluster/identitylookup/nats/nats_identity.go` — implement StorageGrainEnumerator
- `cluster/identitylookup/conformance.go` — add enumerator conformance tests
- `cluster/clusterproviders/natskv/natskv_identity.go` — implement GrainEnumerator
- `cluster/clusterproviders/natsstream/natsstream_identity.go` — implement GrainEnumerator
- `plugin/passivation.go` — set DeactivationReasonPassivation before stopping
