# Shared Placement Actor Design

## Problem

All non-disthash identity providers (natskv, natsstream, `IdentityStorageLookup` with Redis/Postgres/NATS KV backends) spawn actors directly via `SpawnNamed` with no local lifecycle management. This causes:

1. **No graceful shutdown** — when a node shuts down, local grain actors are not poisoned with deactivation reasons. They die abruptly when `ActorSystem.Shutdown()` tears everything down.
2. **No local tracking** — no in-memory map of which grains this node owns, making enumeration and cleanup unreliable.
3. **No termination cleanup** — when a grain stops naturally, nothing notifies the identity storage to remove the activation record. The `handleStopped` middleware clears the PID cache but doesn't touch the identity store.
4. **No spawn verification** — no `CanSpawnIdentity` predicate support for rejecting invalid identities before spawning.
5. **No duplicate spawn prevention** — concurrent `Get()` calls for the same identity can race to spawn, leading to the `ErrNameExists` issues we patched earlier.
6. **Orphaned state on RemovePid** — `RemovePid` deletes identity records without stopping the actor, creating the liveness bug we just fixed with a three-layer defense. A placement actor that owns both the record and the process would eliminate this class of bug entirely.

The .NET upstream (`Proto.Actor`) solves this with `IdentityStoragePlacementActor` — a per-node actor that handles all local spawning, tracking, and cleanup for storage-backed identity lookups. The Go codebase has a TODO acknowledging this gap (`identity_storage_lookup.go` line 144).

## Architecture

### Component Overview

```
┌─────────────────────────────────────────────────────────┐
│                    Identity Lookup                        │
│  (natskv / natsstream / storage+redis / storage+pg /     │
│   storage+nats / disthash)                               │
│                                                          │
│  Responsibilities:                                       │
│  - Distributed lock acquisition                          │
│  - Activation record storage/retrieval                   │
│  - Member cleanup on topology events                     │
│                                                          │
│  Get() flow:                                             │
│  1. Check existing activation in store                   │
│  2. Acquire lock                                         │
│  3. Send ActivationRequest to local placement actor      │
│  4. Return PID from ActivationResponse                   │
└────────────────────────┬────────────────────────────────┘
                         │ ActivationRequest (proto msg)
                         ▼
┌─────────────────────────────────────────────────────────┐
│              Placement Actor ($placement-activator)       │
│              One per non-client member node               │
│                                                          │
│  Responsibilities:                                       │
│  - Spawn actors locally (SpawnNamed)                     │
│  - Track all local grains (ClusterIdentity → PID map)    │
│  - Persist activation via callback (ReenterAfter)        │
│  - Poison all local grains on Stopping (graceful)        │
│  - Handle ActivationTerminating → cleanup storage        │
│  - CanSpawnIdentity verification (optional per-kind)     │
│  - Duplicate spawn prevention (in-flight set)            │
│  - Topology rebalance (optional, for disthash)           │
└─────────────────────────────────────────────────────────┘
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

### Proto Message Update

Add `invalid_identity` to `ActivationResponse`:

```protobuf
message ActivationResponse {
  actor.PID pid = 1;
  bool failed = 2;
  uint64 topology_hash = 3;
  bool invalid_identity = 4;
}
```

### Placement Actor Configuration

```go
type PlacementConfig struct {
    // PersistActivation stores the activation after spawn. Called inside
    // ReenterAfter — the placement actor does not respond until this
    // completes (or fails after retries). Nil means no persistence (disthash).
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

### Termination Mechanism

The placement actor uses `*actor.Terminated` messages (child watch) as the primary cleanup trigger, NOT the `ActivationTerminating` EventStream subscription.

Grains are spawned as children of the placement actor via `ctx.Spawn()` (not `Root.SpawnNamed`). This gives automatic child watching — when a grain stops for any reason, the placement actor receives `*actor.Terminated` and cleans up the local map + calls `RemoveActivation`.

This avoids the ambiguity of receiving both `*actor.Terminated` and `ActivationTerminating` for the same grain. The `handleStopped` middleware continues to publish `ActivationTerminating` for PID cache cleanup and `GrainDeactivated` events — these are orthogonal to the placement actor's local cleanup.

### PID Naming Strategy

Grains spawned by the placement actor use `ctx.SpawnPrefix(props, ci.Identity)` which produces PIDs of the form `$placement-activator/identity$N`. This differs from the current natskv/natsstream convention of `kind/identity` (via `Root.SpawnNamed`).

**Migration impact:** Existing activation records in natskv/natsstream/Redis/Postgres store PIDs in the old `kind/identity` format. After the placement actor is integrated, new activations will have the `$placement-activator/identity$N` format. During rolling upgrades:
- Old records are still resolvable (the PID format is just an address + ID string).
- When an old-format PID becomes stale (node restarts), `RemovePid` cleans it up and the new activation gets the new format.
- No explicit migration step needed — records naturally transition as grains are re-activated.

If the naming change is too disruptive, an alternative is to have the placement actor use `Root.SpawnNamed(props, kind+"/"+identity)` and manually watch the PID via `ctx.Watch(pid)`. This preserves the old naming but loses the automatic child-watch benefit. The manual watch is slightly more code but functionally equivalent.

**Decision: Use `ctx.Watch(pid)` with `Root.SpawnNamed` for backward compatibility.** The placement actor spawns grains with the existing naming convention (`kind/identity`) and explicitly watches them. This avoids a breaking PID format change and ensures rolling upgrades work without any activation record migration.

### Stale Member Validation

When the identity lookup's `Get()` path retrieves an existing activation from storage, it should validate that the owning member is still in the cluster before returning the PID. If the member has departed, the stale activation records should be cleaned up immediately (call `RemoveMemberId`) rather than returning a PID that will dead-letter.

This matches the .NET `IdentityStorageWorker.ValidateAndMapToPid()` behavior. Implementation: a shared utility function `ValidateActivationMember(memberList, memberId) bool` that identity lookups call in their `Get()` path.

### Well-Known Actor Name

The placement actor is spawned with the well-known name `$placement-activator`. This allows remote nodes to address it (for future remote placement support, matching .NET's `IdentityActivatorProxy` pattern).

Only spawned on non-client members. Clients resolve identities by talking to remote members.

### Lifecycle

**Setup (non-client only):**
```
IdentityLookup.Setup(cluster, kinds, isClient=false)
  → Spawn $placement-activator with provider-specific config
  → Placement actor watches each grain it spawns (via ctx.Watch)
```

**Get (activation):**
```
IdentityLookup.Get(ci)
  → Check existing activation in store
  → If not found, acquire lock
  → Send ActivationRequest to local $placement-activator
  → Placement actor: spawn, persist (ReenterAfter), track, respond
  → Identity lookup returns PID
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
        → For each tracked local grain:
          → SetDeactivationReason(pid, DeactivationReasonShutdown)
          → PoisonFuture(pid) → wait (up to ShutdownTimeout)
        → All grains stop gracefully (or force-stop on timeout)
    → Remove member records from storage
```

**Safety invariant: placement actor only poisons PIDs that it spawned locally.** It never touches remote PIDs. The local tracking map (`ClusterIdentity → PID`) only contains actors spawned by this node's placement actor, so iterating it on shutdown is safe by construction.

### Edge Cases

**Spawn-then-immediate-crash:** If a grain crashes immediately after spawn but before `PersistActivation` completes (inside `ReenterAfter`), the placement actor receives `*actor.Terminated` while the `ReenterAfter` continuation is pending. Since the actor mailbox processes messages sequentially, the termination message is queued behind the persistence continuation. When the continuation runs, it either succeeds (and the PID is already dead — the next lookup will get a dead-letter and trigger `RemovePid`) or fails (and the actor is already dead — the poison is a no-op). Either way, the local map is cleaned up when `*actor.Terminated` is processed.

**ActivationRequest during Stopping:** If an `ActivationRequest` arrives while the placement actor is in its `Stopping` handler, it should respond with `Failed: true`. The placement actor should set a `stopping` flag and reject new activations.

**PersistActivation callback panics:** The `ReenterAfter` continuation should recover from panics in the persistence callback, poison the spawned actor, and respond with `Failed: true`.

## Identity Lookup Changes

### IdentityStorageLookup (fixes Redis, Postgres, NATS KV)

Current `spawnActivation` method (direct `SpawnNamed`) is replaced with sending `ActivationRequest` to the local placement actor. The placement actor calls back into the storage backend via the `PersistActivation` callback, which calls `StorageLookup.StoreActivation()`.

`Shutdown()` changes from just `RemoveMemberId()` to: stop placement actor (graceful poison of all local grains) → then `RemoveMemberId()`.

### natskv

`spawnActivation` replaced with `ActivationRequest` to placement actor. The `PersistActivation` callback calls `storeActivation` (which does the CAS update to NATS KV). The `RemoveActivation` callback deletes the KV entry and member tracking.

`Shutdown()` stops the placement actor first, then calls `removeMemberID`.

The `ErrNameExists` recovery path in `spawnActivation` becomes unnecessary — the placement actor prevents duplicate spawns via its in-flight tracking set.

### natsstream

Same pattern as natskv but with stream-specific persistence callbacks.

### disthash

The existing `placementActor` is replaced with the shared one, configured with:
- `PersistActivation: nil` (no external storage)
- `RemoveActivation: nil` (cleanup is via `ActivationTerminated` broadcast)
- `RebalanceOnTopology`: rendezvous hash function that determines ownership changes

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

### Per-provider integration tests (Sub-projects 2-5)
- Full Get() → placement actor → spawn → persist → return PID flow
- Shutdown → placement actor stops → local grains receive Stopping
- Existing provider test suites continue to pass

### Cross-node safety tests (Sub-project 6)
- **Two-node shutdown: only local grains poisoned** — start 2-node cluster, activate grains on both, shut down node 1, verify node 2's grains are alive and responding
- **Two-node topology rebalance: only local grains affected** — add/remove a node, verify only the local node's placement actor poisons its own affected grains
- Placement conformance suite validating any placement actor implementation

## Sub-project Decomposition

### Sub-project 1: Shared Placement Actor + Proto Update
- Add `invalid_identity` to `ActivationResponse` proto, regenerate
- Add `CanSpawnIdentity` field to `cluster.Kind`
- Implement `cluster/placement.go` — the shared default placement actor
- Full unit test suite including cross-node safety (unit level)
- No existing identity lookups touched

### Sub-project 2: Integrate into IdentityStorageLookup
- Refactor `Setup()` to spawn placement actor with `StorageLookup`-backed persistence callbacks
- Refactor `Get()` to send `ActivationRequest` to placement actor
- Refactor `Shutdown()` to stop placement actor first
- Fixes Redis, Postgres, and NATS KV identity lookups automatically
- Run conformance suite + provider integration tests

### Sub-project 3: Integrate into natskv
- Refactor `Setup()` to spawn placement actor
- Refactor `Get()` to delegate spawning
- Refactor `Shutdown()` to stop placement actor
- Remove `ErrNameExists` recovery (now handled by placement actor)
- Update tests, run full suite

### Sub-project 4: Integrate into natsstream
- Same as Sub-project 3

### Sub-project 5: Refactor disthash
- Replace custom `placementActor` with shared one (rebalance enabled, no persistence)
- Simplify `Manager`
- Verify all existing disthash behavior preserved

### Sub-project 6: Conformance + Cross-Node Safety
- `PlacementConformanceSuite`
- Multi-node integration tests (shutdown safety, rebalance safety)
- Final full test pass across all providers

## Future Work

These items are explicitly out of scope for this design but are natural follow-ups:

1. **Remote placement (`IdentityActivatorProxy`)** — The .NET upstream's primary activation path sends `ActivationRequest` to a remote member's placement actor (selected by member strategy). This enables spawning grains on specific members based on affinity, load, or resource availability. The well-known `$placement-activator` name and the proto `ActivationRequest`/`ActivationResponse` messages are already in place to support this.

2. **Duplicate-request queuing** — The .NET `IdentityStorageWorker` coalesces concurrent `GetPid` requests for the same identity, responding to all waiters when the first spawn completes. The current design rejects duplicates with `Failed: true` (callers retry), which is simpler but less efficient under high concurrency for the same identity.

3. **Custom member strategies** — The .NET has `IMemberStrategy` with implementations like `LocalAffinityStrategy` (prefer local node) and `SimpleMemberStrategy` (round-robin). The current Go `MemberStrategy` interface exists but is only used for routing, not for activation placement. Connecting member strategies to the placement actor's activation flow would enable sophisticated placement policies.
