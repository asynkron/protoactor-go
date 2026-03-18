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

`OnRoleChanged` behavior is identical to today:
- `RoleLeader`: spawns all registered actors from `props`, stores PIDs.
- `RoleFollower`: poisons all running actors, clears PID list.
- Thread-safe via mutex. Idempotent — spawning when already spawned is safe due to mutex + PID check.

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
    if registrar, ok := c.Provider.(SingletonSchedulerRegistrar); ok {
        registrar.RegisterSingletonScheduler(listener)
        return nil
    }
    return fmt.Errorf("cluster provider %T does not support singleton scheduling", c.Provider)
}
```

Returns an error if the provider doesn't implement the interface. Works before or after `StartMember` since the `Cluster` struct exists in both phases.

### Provider Implementation Changes

Each provider (natskv, natsstream, etcd, zk) receives the same update:

**Removed:**
- Per-provider `singleton.go` file (type definitions and implementation)
- Per-provider `RoleType`, `RoleChangedListener` types
- Per-provider `singleton_test.go` files

**Updated `RegisterSingletonScheduler`:**

```go
func (p *Provider) RegisterSingletonScheduler(listener cluster.RoleChangedListener) {
    p.roleMu.Lock()
    defer p.roleMu.Unlock()
    p.schedulers = append(p.schedulers, listener)
    if p.isLeader.Load() {
        safeRun(p.logger(), func() {
            listener.OnRoleChanged(cluster.RoleLeader)
        })
    }
}
```

Key behaviors:
- Acquires role lock to prevent races with `setRole`.
- Appends to schedulers slice (type changes from `[]*SingletonScheduler` to `[]cluster.RoleChangedListener`).
- If the node is already leader, immediately notifies only the newly registered scheduler.
- Wrapped in `safeRun` for panic recovery.
- If not leader, no notification — the next `setRole(RoleLeader)` handles it.

**Updated `setRole`:**
- Replaces provider-local `RoleType` references with `cluster.RoleType`.
- Otherwise unchanged — iterates all schedulers and calls `OnRoleChanged`.

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

### Integration Tests

Updated in natskv and natsstream provider test files:

- Register via `cluster.RegisterSingletonScheduler()` after `StartMember` — verify actors spawn.
- Register via `cluster.RegisterSingletonScheduler()` before `StartMember` — verify existing behavior.

## Migration

This is a breaking change. Per-provider `SingletonScheduler` types and `RegisterSingletonScheduler` methods are removed. Users must:

1. Replace `natskv.NewSingletonScheduler(rc)` (or equivalent) with `cluster.NewSingletonScheduler(rc)`.
2. Replace `provider.RegisterSingletonScheduler(s)` with `cluster.RegisterSingletonScheduler(s)`.
3. Remove imports of provider-specific singleton types.
4. Custom implementations must satisfy `cluster.RoleChangedListener`.
