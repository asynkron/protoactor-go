# Singleton Scheduler Rework Design

**Date:** 2026-03-18
**Status:** Draft

## Problem

SingletonScheduler is duplicated across four providers (natskv, natsstream, etcd, zk) with nearly identical code. It must be registered before `StartMember()`, which makes it difficult for users to add singleton actors to a running cluster. The per-provider API forces users to import provider-specific packages for a concept that is provider-agnostic.

## Goals

1. Eliminate the four duplicate SingletonScheduler implementations.
2. Allow `RegisterSingletonScheduler` to be called at any time — before or after cluster start.
3. Provide a shared interface so users can supply custom implementations.
4. Keep providers in control of leadership knowledge and role change notification.

## Non-Goals

- Deregistration of singleton schedulers.
- Changing how providers detect leadership.
- .NET compatibility.

## Design

### Shared Types (`cluster/singleton.go`)

New file in the `cluster/` package containing:

**`RoleType`** — shared enum replacing four per-provider copies:

```go
type RoleType int

const (
    RoleFollower RoleType = iota
    RoleLeader
)

func (r RoleType) String() string // Returns "Leader" or "Follower"
```

**`RoleChangedListener`** — interface any scheduler must satisfy:

```go
type RoleChangedListener interface {
    OnRoleChanged(RoleType)
}
```

**`SingletonScheduler`** — shared concrete implementation:

```go
type SingletonScheduler struct {
    sync.Mutex
    root  *actor.RootContext
    props []*actor.Props
    pids  []*actor.PID
}

func NewSingletonScheduler(rc *actor.RootContext) *SingletonScheduler
func (s *SingletonScheduler) FromFunc(f actor.ReceiveFunc) *SingletonScheduler
func (s *SingletonScheduler) FromProducer(f actor.Producer) *SingletonScheduler
func (s *SingletonScheduler) OnRoleChanged(rt RoleType)
```

`OnRoleChanged` implementation:

```go
func (s *SingletonScheduler) OnRoleChanged(rt RoleType) {
    s.Lock()
    defer s.Unlock()
    switch rt {
    case RoleFollower:
        if len(s.pids) > 0 {
            s.root.Logger().Info("I am follower, poison singleton actors")
            for _, pid := range s.pids {
                s.root.Poison(pid)
            }
            s.pids = nil
        }
    case RoleLeader:
        if len(s.pids) > 0 {
            return // already leader with running actors, no-op
        }
        if len(s.props) > 0 {
            s.root.Logger().Info("I am leader now, start singleton actors")
            s.pids = make([]*actor.PID, len(s.props))
            for i, p := range s.props {
                s.pids[i] = s.root.Spawn(p)
            }
        }
    }
}
```

Key behaviors:
- `RoleLeader`: if `pids` is already populated (already leader), no-op. This guards against double-spawn if `OnRoleChanged(RoleLeader)` is called twice (e.g., immediate notification on registration + a leadership flap).
- `RoleFollower`: poisons all running actors, clears PID list.
- Thread-safe via mutex.
- **Invariant:** `OnRoleChanged` must not call back into `RegisterSingletonScheduler` on the provider — doing so would deadlock under the role lock.

### Provider Interface Extension (`cluster/cluster_provider.go`)

New optional interface:

```go
type SingletonSchedulerRegistrar interface {
    RegisterSingletonScheduler(listener RoleChangedListener)
}
```

Opt-in, same pattern as the existing `KindUpdater` interface. The `ClusterProvider` interface is unchanged.

### Cluster API (`cluster/cluster.go`)

New method on the `Cluster` struct:

```go
func (c *Cluster) RegisterSingletonScheduler(listener RoleChangedListener) error {
    if registrar, ok := c.provider.(SingletonSchedulerRegistrar); ok {
        registrar.RegisterSingletonScheduler(listener)
        return nil
    }
    return fmt.Errorf("cluster provider %T does not support singleton scheduling", c.provider)
}
```

Returns an error if the provider doesn't implement the interface. Works before or after `StartMember` since the `Cluster` struct exists in both phases.

**Note:** The `automanaged` provider does not support leader election and will not implement `SingletonSchedulerRegistrar`. Calling `cluster.RegisterSingletonScheduler()` with an automanaged provider returns an error.

### Provider Implementation Changes

Each provider (natskv, natsstream, etcd, zk) receives the same update:

**Removed:**
- Per-provider `singleton.go` file (type definitions and implementation)
- Per-provider `RoleType`, `RoleChangedListener` types — note that zk defines `RoleType` inline in `zk_provider.go` (not in a separate file), so those inline definitions are also removed
- Per-provider `singleton_test.go` files

**New fields (etcd, zk):**
- etcd and zk providers currently lack `schedulers`, `roleMu`, and structured `setRole` methods. These providers need:
  - `schedulers []cluster.RoleChangedListener` field added to provider struct
  - `roleMu sync.Mutex` field added to provider struct
  - Their `updateLeadership`/`onEvent` methods refactored to use `roleMu` and notify schedulers, following the same pattern as natskv/natsstream `setRole`

**Updated `RegisterSingletonScheduler`:**

```go
func (p *Provider) RegisterSingletonScheduler(listener cluster.RoleChangedListener) {
    if p.shutdown.Load() {
        return
    }
    p.roleMu.Lock()
    defer p.roleMu.Unlock()
    p.schedulers = append(p.schedulers, listener)
    if p.isLeader.Load() {
        cluster.SafeRunRoleChange(p.logger(), func() {
            listener.OnRoleChanged(cluster.RoleLeader)
        })
    }
}
```

Key behaviors:
- Checks shutdown flag first — registration after shutdown is a no-op.
- Acquires role lock to prevent races with `setRole`.
- Appends to schedulers slice (type changes from `[]*SingletonScheduler` to `[]cluster.RoleChangedListener`).
- If the node is already leader, immediately notifies only the newly registered scheduler.
- Wrapped in `SafeRunRoleChange` for panic recovery.
- If not leader, no notification — the next `setRole(RoleLeader)` handles it.

**Updated `setRole` / `updateLeadership`:**
- Replaces provider-local `RoleType` references with `cluster.RoleType`.
- Must hold `roleMu` for the entire method, including scheduler iteration. Currently some providers release the lock before iterating schedulers, which races with `RegisterSingletonScheduler` appending to the slice. The fix is to hold `roleMu` through the iteration. This is safe because `OnRoleChanged` is wrapped in `SafeRunRoleChange` (panic recovery) and must not call back into the provider (see invariant above).
- The non-blocking channel send to `roleChangedChan` (for `WithRoleChangedListener`) moves inside the lock. The `select`/`default` pattern is preserved to avoid deadlock on a full channel.

### `WithRoleChangedListener` Config Option

Each provider (natskv, natsstream) has a `WithRoleChangedListener` config option that sets a single `RoleChangedListener` notified via the provider's role-changed channel/goroutine. This is a separate mechanism from the schedulers slice.

This option is preserved but updated to use the shared `cluster.RoleChangedListener` type instead of the per-provider type. The notification mechanism (channel + goroutine) remains provider-specific since it predates and is distinct from the singleton scheduler pattern. The per-provider `RoleChangedListener` interface definition is removed since `cluster.RoleChangedListener` has the same signature.

### Shared `safeRun` Helper

The `safeRun` panic-recovery wrapper is duplicated across providers identically. It is lifted to `cluster/singleton.go` as an exported `SafeRunRoleChange` function. It must be exported because providers in sub-packages (`cluster/clusterproviders/natskv/`, etc.) need to call it.

### Deleted Files

- `cluster/clusterproviders/natskv/singleton.go`
- `cluster/clusterproviders/natsstream/singleton.go`
- `cluster/clusterproviders/etcd/singleton.go`
- `cluster/clusterproviders/zk/singleton.go`
- `cluster/clusterproviders/natskv/singleton_test.go`
- `cluster/clusterproviders/natsstream/singleton_test.go`

## Test Strategy

### Shared Unit Tests (`cluster/singleton_test.go`)

- `TestSingletonScheduler_FromFunc` — registration via `FromFunc` works.
- `TestSingletonScheduler_FromProducer` — registration via `FromProducer` works.
- `TestSingletonScheduler_OnRoleChanged_Leader` — spawns actors on leader transition.
- `TestSingletonScheduler_OnRoleChanged_Follower` — poisons actors on follower transition.
- `TestCustomRoleChangedListener` — user-defined `RoleChangedListener` implementation works.

### Provider Unit Tests

Updated in each provider's test file:

- Late registration: `RegisterSingletonScheduler` called after leader election immediately notifies.
- Early registration: `RegisterSingletonScheduler` called before leader election notifies on role change.
- Concurrent registration + role change: goroutines calling `RegisterSingletonScheduler` and `setRole` concurrently under `-race`.
- Shutdown guard: `RegisterSingletonScheduler` called after shutdown is a no-op.

### Integration Tests

Updated in natskv and natsstream provider test files:

- Register via `cluster.RegisterSingletonScheduler()` after `StartMember` — verify actors spawn.
- Register via `cluster.RegisterSingletonScheduler()` before `StartMember` — verify existing behavior.

## Migration

This is a breaking change. Per-provider `SingletonScheduler` types and `RegisterSingletonScheduler` methods are removed. Users must:

1. Replace `natskv.NewSingletonScheduler(rc)` (or equivalent) with `cluster.NewSingletonScheduler(rc)`.
2. Replace `provider.RegisterSingletonScheduler(s)` with `c.RegisterSingletonScheduler(s)` where `c` is the `*cluster.Cluster` instance.
3. Remove imports of provider-specific singleton types.
4. Custom implementations must satisfy `cluster.RoleChangedListener`.
5. `WithRoleChangedListener` config options now accept `cluster.RoleChangedListener` instead of the per-provider type.
