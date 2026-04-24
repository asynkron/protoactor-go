# Singleton Scheduler Rework Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Lift SingletonScheduler from four duplicate per-provider implementations into a shared `cluster/` package, and make registration callable before or after cluster start.

**Architecture:** Shared `RoleType`, `RoleChangedListener` interface, and `SingletonScheduler` struct in `cluster/singleton.go`. New `SingletonSchedulerRegistrar` opt-in provider interface in `cluster/cluster_provider.go`. `Cluster.RegisterSingletonScheduler()` delegates to the provider. Each provider's `RegisterSingletonScheduler` is updated to immediately notify if already leader, with proper locking.

**Tech Stack:** Go, testify (assert/require), proto.actor actor system

**Spec:** `docs/superpowers/specs/2026-03-18-singleton-scheduler-rework-design.md`

---

## File Structure

| Action | File | Responsibility |
|--------|------|---------------|
| Create | `cluster/singleton.go` | `RoleType`, `RoleChangedListener`, `SingletonScheduler`, `SafeRunRoleChange` |
| Create | `cluster/singleton_test.go` | Unit tests for shared types |
| Modify | `cluster/cluster_provider.go` | Add `SingletonSchedulerRegistrar` interface |
| Modify | `cluster/cluster.go` | Add `Cluster.RegisterSingletonScheduler()` method |
| Delete | `cluster/clusterproviders/natskv/singleton.go` | Replaced by shared types |
| Delete | `cluster/clusterproviders/natskv/singleton_test.go` | Replaced by shared tests |
| Modify | `cluster/clusterproviders/natskv/natskv_provider.go:27-59,87-90,107-111,656-725` | Update Provider struct, constructor, RegisterSingletonScheduler, setRole, remove safeRun |
| Modify | `cluster/clusterproviders/natskv/config.go:16-39,53,109-112` | Remove RoleType/String()/RoleChangedListener, update WithRoleChangedListener |
| Modify | `cluster/clusterproviders/natskv/config_test.go:115-127` | Remove TestRoleTypeString, update mockRoleChangedListener to use cluster types |
| Modify | `cluster/clusterproviders/natskv/natskv_provider_test.go:21-26,269-290,350-382` | Update mockRoleListener, TestRoleChangedListener_Called, TestSingletonScheduler_SpawnOnLeader |
| Delete | `cluster/clusterproviders/natsstream/singleton.go` | Replaced by shared types |
| Delete | `cluster/clusterproviders/natsstream/singleton_test.go` | Replaced by shared tests |
| Modify | `cluster/clusterproviders/natsstream/natsstream_provider.go:29-65,89,109-113,739-808` | Update Provider struct, constructor, RegisterSingletonScheduler, setRole, remove safeRun |
| Modify | `cluster/clusterproviders/natsstream/config.go:23-43,81-83` | Remove RoleType/String()/RoleChangedListener, update WithRoleChangedListener |
| Modify | `cluster/clusterproviders/natsstream/config_test.go` | Remove TestRoleType_String |
| Modify | `cluster/clusterproviders/natsstream/natsstream_provider_test.go` | Update mockRoleListener, integration tests |
| Delete | `cluster/clusterproviders/etcd/singleton.go` | Replaced by shared types |
| Modify | `cluster/clusterproviders/etcd/etcd_provider.go:20-43,73-83,491-533,566-575` | Add roleMu, update struct/constructor, RegisterSingletonScheduler, updateLeadership, startRoleChangedNotifyLoop, remove safeRun/getRunTimeStack |
| Modify | `cluster/clusterproviders/etcd/config.go:15-18,37-42,68` | Remove RoleChangedListener, update WithRoleChangedListener, update config field |
| Modify | `cluster/clusterproviders/etcd/etcd_provider_shutdown_test.go:14` | Update roleChangedChan type |
| Delete | `cluster/clusterproviders/zk/singleton.go` | Replaced by shared types |
| Modify | `cluster/clusterproviders/zk/zk_provider.go:19-34,37-53,73-74,324-359` | Remove RoleType, add roleMu/schedulers, update constructor, updateLeadership, onEvent, startRoleChangedNotifyLoop |
| Modify | `cluster/clusterproviders/zk/config.go:40-51,68-79,86` | Update RoleChangedListener/OnRoleChangedFunc/WithRoleChangedListener/WithRoleChangedFunc/config field |
| Modify | `cluster/clusterproviders/zk/utils.go:57-70` | Remove safeRun and getRunTimeStack |
| Modify | `cluster/clusterproviders/zk/misc_test.go:51` | Remove or update safeRun test |

---

## Chunk 1: Shared Types and Unit Tests

### Task 1: Create shared `RoleType`, `RoleChangedListener`, `SafeRunRoleChange`, and `SingletonScheduler`

**Files:**
- Create: `cluster/singleton.go`
- Create: `cluster/singleton_test.go`

- [ ] **Step 1: Create `cluster/singleton.go` with all shared types**

```go
package cluster

import (
	"fmt"
	"log/slog"
	"runtime"
	"sync"

	"github.com/awevoke/protoactor-go/actor"
)

// RoleType represents the leadership role of a cluster node.
type RoleType int

const (
	// RoleFollower indicates the node is not the leader.
	RoleFollower RoleType = iota
	// RoleLeader indicates the node currently holds leadership.
	RoleLeader
)

// String returns the human-readable name of the role.
func (r RoleType) String() string {
	switch r {
	case RoleFollower:
		return "Follower"
	case RoleLeader:
		return "Leader"
	default:
		return fmt.Sprintf("RoleType(%d)", int(r))
	}
}

// RoleChangedListener is notified when the cluster node's leadership role changes.
// Implementations must not call back into the provider's RegisterSingletonScheduler
// from within OnRoleChanged — doing so will deadlock.
type RoleChangedListener interface {
	OnRoleChanged(RoleType)
}

// Compile-time check that SingletonScheduler implements RoleChangedListener.
var _ RoleChangedListener = (*SingletonScheduler)(nil)

// SafeRunRoleChange wraps a function call with panic recovery, logging any panic
// that occurs during role change notification.
func SafeRunRoleChange(logger *slog.Logger, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 64<<10)
			buf = buf[:runtime.Stack(buf, false)]
			logger.Warn("OnRoleChanged panic recovered",
				slog.Any("error", fmt.Errorf("%v\n%s", r, buf)))
		}
	}()
	fn()
}

// SingletonScheduler manages actors that should run on exactly one node — the leader.
// When the node becomes leader, all registered actors are spawned.
// When the node loses leadership, all running actors are poisoned.
type SingletonScheduler struct {
	sync.Mutex
	root  *actor.RootContext
	props []*actor.Props
	pids  []*actor.PID
}

// NewSingletonScheduler creates a new SingletonScheduler.
func NewSingletonScheduler(rc *actor.RootContext) *SingletonScheduler {
	return &SingletonScheduler{root: rc}
}

// FromFunc registers an actor receive function to run on the leader.
func (s *SingletonScheduler) FromFunc(f actor.ReceiveFunc) *SingletonScheduler {
	s.Lock()
	defer s.Unlock()
	s.props = append(s.props, actor.PropsFromFunc(f))
	return s
}

// FromProducer registers an actor producer to run on the leader.
func (s *SingletonScheduler) FromProducer(f actor.Producer) *SingletonScheduler {
	s.Lock()
	defer s.Unlock()
	s.props = append(s.props, actor.PropsFromProducer(f))
	return s
}

// OnRoleChanged is called when the cluster node's leadership role changes.
// On RoleLeader: spawns all registered actors (no-op if already leader with running actors).
// On RoleFollower: poisons all running actors.
// This is a behavioral change from prior per-provider implementations which spawned
// unconditionally — the double-spawn guard prevents duplicate actors.
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

- [ ] **Step 2: Verify it compiles**

Run: `go build ./cluster/`
Expected: Success.

- [ ] **Step 3: Create `cluster/singleton_test.go` with all unit tests**

```go
package cluster

import (
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSystem(t *testing.T) *actor.ActorSystem {
	t.Helper()
	system := actor.NewActorSystem()
	t.Cleanup(func() { system.Shutdown() })
	return system
}

type testActor struct{}

func (a *testActor) Receive(ctx actor.Context) {}

func TestSingletonScheduler_FromFunc(t *testing.T) {
	system := newTestSystem(t)
	s := NewSingletonScheduler(system.Root)
	s.FromFunc(func(ctx actor.Context) {})
	assert.Len(t, s.props, 1)
}

func TestSingletonScheduler_FromProducer(t *testing.T) {
	system := newTestSystem(t)
	s := NewSingletonScheduler(system.Root)
	s.FromProducer(func() actor.Actor {
		return &testActor{}
	})
	assert.Len(t, s.props, 1)
}

func TestSingletonScheduler_FromFunc_Chaining(t *testing.T) {
	system := newTestSystem(t)
	s := NewSingletonScheduler(system.Root)
	result := s.FromFunc(func(ctx actor.Context) {}).FromFunc(func(ctx actor.Context) {})
	assert.Same(t, s, result)
	assert.Len(t, s.props, 2)
}

func TestSingletonScheduler_OnRoleChanged_Leader_Spawns(t *testing.T) {
	system := newTestSystem(t)
	var started atomic.Bool
	s := NewSingletonScheduler(system.Root)
	s.FromFunc(func(ctx actor.Context) {
		if _, ok := ctx.Message().(*actor.Started); ok {
			started.Store(true)
		}
	})

	s.OnRoleChanged(RoleLeader)

	require.Eventually(t, started.Load, 2*time.Second, 10*time.Millisecond)
	assert.Len(t, s.pids, 1)
	assert.NotNil(t, s.pids[0])
}

func TestSingletonScheduler_OnRoleChanged_Follower_Poisons(t *testing.T) {
	system := newTestSystem(t)
	var stopped atomic.Bool
	s := NewSingletonScheduler(system.Root)
	s.FromFunc(func(ctx actor.Context) {
		if _, ok := ctx.Message().(*actor.Stopping); ok {
			stopped.Store(true)
		}
	})

	s.OnRoleChanged(RoleLeader)
	require.Len(t, s.pids, 1)

	s.OnRoleChanged(RoleFollower)

	require.Eventually(t, stopped.Load, 2*time.Second, 10*time.Millisecond)
	assert.Nil(t, s.pids)
}

func TestSingletonScheduler_OnRoleChanged_Leader_DoubleCall_NoDuplicateSpawn(t *testing.T) {
	system := newTestSystem(t)
	var spawnCount atomic.Int32
	s := NewSingletonScheduler(system.Root)
	s.FromFunc(func(ctx actor.Context) {
		if _, ok := ctx.Message().(*actor.Started); ok {
			spawnCount.Add(1)
		}
	})

	s.OnRoleChanged(RoleLeader)
	require.Eventually(t, func() bool { return spawnCount.Load() >= 1 }, 2*time.Second, 10*time.Millisecond)

	s.OnRoleChanged(RoleLeader) // second call should be no-op
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, int32(1), spawnCount.Load())
	assert.Len(t, s.pids, 1)
}

func TestSingletonScheduler_OnRoleChanged_Follower_WhenAlreadyFollower_Noop(t *testing.T) {
	system := newTestSystem(t)
	s := NewSingletonScheduler(system.Root)
	s.FromFunc(func(ctx actor.Context) {})

	// Should not panic or error when called without any spawned actors
	s.OnRoleChanged(RoleFollower)
	assert.Nil(t, s.pids)
}

type mockRoleChangedListener struct {
	roles []RoleType
	mu    sync.Mutex
}

func (m *mockRoleChangedListener) OnRoleChanged(rt RoleType) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.roles = append(m.roles, rt)
}

func TestCustomRoleChangedListener(t *testing.T) {
	mock := &mockRoleChangedListener{}
	var listener RoleChangedListener = mock // compile-time interface check

	listener.OnRoleChanged(RoleLeader)
	listener.OnRoleChanged(RoleFollower)

	mock.mu.Lock()
	defer mock.mu.Unlock()
	assert.Equal(t, []RoleType{RoleLeader, RoleFollower}, mock.roles)
}

func TestSafeRunRoleChange_PanicRecovery(t *testing.T) {
	logger := slog.Default()
	// Should not panic — SafeRunRoleChange recovers
	assert.NotPanics(t, func() {
		SafeRunRoleChange(logger, func() {
			panic("test panic")
		})
	})
}

func TestRoleType_String(t *testing.T) {
	assert.Equal(t, "Follower", RoleFollower.String())
	assert.Equal(t, "Leader", RoleLeader.String())
	assert.Equal(t, "RoleType(99)", RoleType(99).String())
}
```

- [ ] **Step 4: Run all tests**

Run: `go test ./cluster/ -run "TestSingletonScheduler|TestCustomRoleChanged|TestSafeRun|TestRoleType_String" -v -race`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cluster/singleton.go cluster/singleton_test.go
git commit -m "feat(cluster): add shared RoleType, RoleChangedListener, SingletonScheduler, and SafeRunRoleChange"
```

## Chunk 2: Provider Interface and Cluster API

### Task 2: Add `SingletonSchedulerRegistrar` interface and `Cluster.RegisterSingletonScheduler`

**Files:**
- Modify: `cluster/cluster_provider.go:15-26` (add after `KindUpdater`)
- Modify: `cluster/cluster.go:388-400` (add after `notifyKindUpdate`)

- [ ] **Step 1: Add the interface to `cluster/cluster_provider.go`**

Add after the `KindUpdater` interface (after line 26):

```go
// SingletonSchedulerRegistrar is an optional interface that cluster providers
// can implement to support singleton actor scheduling. Providers that support
// leader election should implement this interface.
type SingletonSchedulerRegistrar interface {
	RegisterSingletonScheduler(listener RoleChangedListener)
}
```

- [ ] **Step 2: Add the method to `cluster/cluster.go`**

Add after the `notifyKindUpdate` method (after line 400). Note: `"fmt"` is already imported in `cluster.go`.

```go
// RegisterSingletonScheduler registers a RoleChangedListener with the cluster provider.
// The listener will be notified of leadership role changes. If the provider is already
// the leader, the listener is immediately notified.
// Returns an error if the cluster provider does not support singleton scheduling.
// The Cluster must have been created (via cluster.Configure) before calling this method,
// but it may be called before or after StartMember.
func (c *Cluster) RegisterSingletonScheduler(listener RoleChangedListener) error {
	if c.provider == nil {
		return fmt.Errorf("cluster provider not configured")
	}
	if registrar, ok := c.provider.(SingletonSchedulerRegistrar); ok {
		registrar.RegisterSingletonScheduler(listener)
		return nil
	}
	return fmt.Errorf("cluster provider %T does not support singleton scheduling", c.provider)
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./cluster/`
Expected: Success.

- [ ] **Step 4: Commit**

```bash
git add cluster/cluster_provider.go cluster/cluster.go
git commit -m "feat(cluster): add SingletonSchedulerRegistrar interface and Cluster.RegisterSingletonScheduler"
```

## Chunk 3: natskv Provider Update

### Task 3: Update natskv to use shared types

All changes in this task must be done atomically — do NOT attempt to build or test between steps.

**Files:**
- Delete: `cluster/clusterproviders/natskv/singleton.go`
- Delete: `cluster/clusterproviders/natskv/singleton_test.go`
- Modify: `cluster/clusterproviders/natskv/config.go`
- Modify: `cluster/clusterproviders/natskv/config_test.go`
- Modify: `cluster/clusterproviders/natskv/natskv_provider.go`
- Modify: `cluster/clusterproviders/natskv/natskv_provider_test.go`

- [ ] **Step 1: Delete per-provider singleton files**

```bash
rm cluster/clusterproviders/natskv/singleton.go
rm cluster/clusterproviders/natskv/singleton_test.go
```

- [ ] **Step 2: Update `config.go` — remove local types**

In `cluster/clusterproviders/natskv/config.go`:

Remove lines 16-39 entirely — this includes:
- `RoleType` type definition (line 17)
- `Follower`/`Leader` constants (lines 19-24)
- `String()` method (lines 27-34)
- `RoleChangedListener` interface (lines 36-39)

Add `"github.com/awevoke/protoactor-go/cluster"` to imports.

Update the `config` struct field (line 53):
```go
// Before:
RoleChanged     RoleChangedListener
// After:
RoleChanged     cluster.RoleChangedListener
```

Update `WithRoleChangedListener` (lines 109-112):
```go
// Before:
func WithRoleChangedListener(l RoleChangedListener) Option {
// After:
func WithRoleChangedListener(l cluster.RoleChangedListener) Option {
```

- [ ] **Step 3: Update `config_test.go` — remove/update type references**

In `cluster/clusterproviders/natskv/config_test.go`:

Remove `TestRoleTypeString` test (lines 115-118) — this is now tested in `cluster/singleton_test.go`.

Update `mockRoleChangedListener` (lines 120-127):
```go
// Before:
type mockRoleChangedListener struct {
	lastRole RoleType
}
func (m *mockRoleChangedListener) OnRoleChanged(role RoleType) {
	m.lastRole = role
}

// After:
type mockRoleChangedListener struct {
	lastRole cluster.RoleType
}
func (m *mockRoleChangedListener) OnRoleChanged(role cluster.RoleType) {
	m.lastRole = role
}
```

Add `"github.com/awevoke/protoactor-go/cluster"` to test imports.

- [ ] **Step 4: Update Provider struct fields in `natskv_provider.go` (lines 46-51)**

```go
// Before:
	role                RoleType
	roleMu              sync.Mutex
	roleChangedChan     chan RoleType
	roleChangedListener RoleChangedListener
	schedulers []*SingletonScheduler
	isLeader   atomic.Bool

// After:
	role                cluster.RoleType
	roleMu              sync.Mutex
	roleChangedChan     chan cluster.RoleType
	roleChangedListener cluster.RoleChangedListener
	schedulers []cluster.RoleChangedListener
	isLeader   atomic.Bool
```

- [ ] **Step 5: Update constructor initialization (lines 83-90)**

```go
// Before:
	role:                Follower,
	roleChangedChan:     make(chan RoleType, 1),

// After:
	role:                cluster.RoleFollower,
	roleChangedChan:     make(chan cluster.RoleType, 1),
```

- [ ] **Step 6: Replace `RegisterSingletonScheduler` method (lines 107-111)**

```go
// Before:
func (p *Provider) RegisterSingletonScheduler(scheduler *SingletonScheduler) {
	p.schedulers = append(p.schedulers, scheduler)
}

// After:
func (p *Provider) RegisterSingletonScheduler(listener cluster.RoleChangedListener) {
	if p.shutdown.Load() {
		return
	}
	p.roleMu.Lock()
	defer p.roleMu.Unlock()
	p.schedulers = append(p.schedulers, listener)
	if p.role == cluster.RoleLeader {
		cluster.SafeRunRoleChange(p.logger(), func() {
			listener.OnRoleChanged(cluster.RoleLeader)
		})
	}
}
```

- [ ] **Step 7: Replace `setRole` method (lines 656-685)**

The current code releases `roleMu` before iterating schedulers — this is a data race. Replace the entire method:

```go
// Before (lines 656-685):
func (p *Provider) setRole(role RoleType) {
	p.roleMu.Lock()
	if role == p.role {
		p.roleMu.Unlock()
		return
	}
	p.logger().Info("Role changed",
		slog.String("provider", "natskv"),
		slog.String("from", p.role.String()),
		slog.String("to", role.String()))
	p.role = role
	p.roleMu.Unlock()
	select {
	case p.roleChangedChan <- role:
	default:
	}
	for _, scheduler := range p.schedulers {
		safeRun(p.logger(), func() {
			scheduler.OnRoleChanged(role)
		})
	}
}

// After:
func (p *Provider) setRole(role cluster.RoleType) {
	p.roleMu.Lock()
	defer p.roleMu.Unlock()

	if role == p.role {
		return
	}

	p.logger().Info("Role changed",
		slog.String("provider", "natskv"),
		slog.String("from", p.role.String()),
		slog.String("to", role.String()))

	p.role = role

	// Non-blocking send to role changed channel
	select {
	case p.roleChangedChan <- role:
	default:
	}

	// Notify all registered singleton schedulers
	for _, scheduler := range p.schedulers {
		cluster.SafeRunRoleChange(p.logger(), func() {
			scheduler.OnRoleChanged(role)
		})
	}
}
```

- [ ] **Step 8: Update `startRoleChangedNotifyLoop` (lines 687-704)**

Replace the `safeRun` call at line 697:

```go
// Before:
safeRun(p.logger(), func() { lis.OnRoleChanged(role) })

// After:
cluster.SafeRunRoleChange(p.logger(), func() { lis.OnRoleChanged(role) })
```

- [ ] **Step 9: Remove local `safeRun` function (lines 714-725)**

Delete the entire `safeRun` function.

- [ ] **Step 10: Add compile-time interface assertion**

Add near the top of `natskv_provider.go` (next to the existing `var _ cluster.ClusterProvider` line):
```go
var _ cluster.SingletonSchedulerRegistrar = (*Provider)(nil)
```

- [ ] **Step 11: Update all remaining `Follower`/`Leader`/`RoleType` references**

Search `natskv_provider.go` for any remaining bare `Follower`, `Leader`, or `RoleType` references and replace with `cluster.RoleFollower`, `cluster.RoleLeader`, `cluster.RoleType`. Key locations:
- `isLeader.Store(true/false)` calls — these use `atomic.Bool`, no change needed
- Any `switch` cases on role values
- The `role.String()` calls now go through `cluster.RoleType.String()`

- [ ] **Step 11: Update `natskv_provider_test.go` — mock types and test references**

Update `mockRoleListener` (lines 21-26):
```go
// Before:
type mockRoleListener struct {
	callback func(RoleType)
}
func (m *mockRoleListener) OnRoleChanged(r RoleType) { m.callback(r) }

// After:
type mockRoleListener struct {
	callback func(cluster.RoleType)
}
func (m *mockRoleListener) OnRoleChanged(r cluster.RoleType) { m.callback(r) }
```

Update `TestRoleChangedListener_Called` (around line 269) — replace `int32(Leader)` with `int32(cluster.RoleLeader)`.

Update `TestSingletonScheduler_SpawnOnLeader` (lines 350-382):
- Replace `NewSingletonScheduler(c.ActorSystem.Root)` with `cluster.NewSingletonScheduler(c.ActorSystem.Root)`
- Replace `p.RegisterSingletonScheduler(scheduler)` with `c.RegisterSingletonScheduler(scheduler)` (note: `c` is the `*cluster.Cluster` variable — check the test's variable names)
- **Remove `scheduler.pids` assertions** — after moving `SingletonScheduler` to the `cluster` package, `pids` is unexported and inaccessible from the `natskv` test package. The `spawned` atomic bool assertion already proves correctness. The field-level test coverage is in `cluster/singleton_test.go`.
- Add `"github.com/awevoke/protoactor-go/cluster"` to test imports

- [ ] **Step 13: Verify compilation**

Run: `go build ./cluster/clusterproviders/natskv/`
Expected: Success.

- [ ] **Step 14: Run natskv tests**

Run: `go test ./cluster/clusterproviders/natskv/ -v -race -count=1 -short`
Expected: PASS

- [ ] **Step 15: Commit**

```bash
git add -A cluster/clusterproviders/natskv/
git commit -m "feat(natskv): use shared cluster.RoleType and cluster.RoleChangedListener, remove per-provider singleton"
```

## Chunk 4: natsstream Provider Update

### Task 4: Update natsstream to use shared types

All changes must be done atomically. Follow the same pattern as Task 3 (natskv). The natsstream provider is structurally identical.

**Files:**
- Delete: `cluster/clusterproviders/natsstream/singleton.go`
- Delete: `cluster/clusterproviders/natsstream/singleton_test.go`
- Modify: `cluster/clusterproviders/natsstream/config.go`
- Modify: `cluster/clusterproviders/natsstream/config_test.go`
- Modify: `cluster/clusterproviders/natsstream/natsstream_provider.go`
- Modify: `cluster/clusterproviders/natsstream/natsstream_provider_test.go`

- [ ] **Step 1: Delete per-provider singleton files**

```bash
rm cluster/clusterproviders/natsstream/singleton.go
rm cluster/clusterproviders/natsstream/singleton_test.go
```

- [ ] **Step 2: Update `config.go` — remove local types**

In `cluster/clusterproviders/natsstream/config.go`:

Remove lines 23-43 entirely — this includes:
- `RoleType` type definition (line 24)
- `Follower`/`Leader` constants (lines 27-28)
- `String()` method (lines 31-37)
- `RoleChangedListener` interface (lines 41-43)

Add `"github.com/awevoke/protoactor-go/cluster"` to imports.

Update the `config` struct's `RoleChanged` field to `cluster.RoleChangedListener`.

Update `WithRoleChangedListener` (lines 81-83) parameter type to `cluster.RoleChangedListener`.

- [ ] **Step 3: Update `config_test.go`**

Remove `TestRoleType_String` test. No mock updates needed in this file (natsstream's `config_test.go` has no `mockRoleChangedListener`). Add cluster import if needed.

- [ ] **Step 4: Update Provider struct fields in `natsstream_provider.go` (lines 29-65)**

Same changes as natskv Task 3 Step 4: `RoleType` → `cluster.RoleType`, `[]*SingletonScheduler` → `[]cluster.RoleChangedListener`, etc.

- [ ] **Step 5: Update constructor initialization (around line 89)**

`role: Follower` → `role: cluster.RoleFollower`, `make(chan RoleType, 1)` → `make(chan cluster.RoleType, 1)`.

- [ ] **Step 6: Replace `RegisterSingletonScheduler` (lines 109-113)**

Same implementation as natskv Task 3 Step 6: shutdown guard, lock, append, immediate notify if leader.

- [ ] **Step 7: Replace `setRole` (lines 739-768)**

Same pattern as natskv Task 3 Step 7: hold `roleMu` for entire method including scheduler iteration, replace constants and `safeRun`.

- [ ] **Step 8: Update `startRoleChangedNotifyLoop`**

Replace `safeRun` call with `cluster.SafeRunRoleChange`.

- [ ] **Step 9: Remove local `safeRun` (lines 797-808)**

Delete the entire function.

- [ ] **Step 10: Update all remaining `Follower`/`Leader`/`RoleType` references**

Same as natskv — search and replace throughout the file.

- [ ] **Step 11: Add compile-time interface assertion**

Add near the top of `natsstream_provider.go` (next to the existing `var _ cluster.ClusterProvider` line):
```go
var _ cluster.SingletonSchedulerRegistrar = (*Provider)(nil)
```

- [ ] **Step 12: Update `natsstream_provider_test.go`**

Update `mockRoleListener` to use `cluster.RoleType`. Update `TestRoleChangedListener_Called` references to `Leader` → `cluster.RoleLeader`. Update `TestSingletonScheduler_SpawnOnLeader` to use `cluster.NewSingletonScheduler` and `c.RegisterSingletonScheduler`. **Remove `scheduler.pids` assertions** — `pids` is unexported from `cluster` package; use the `spawned` atomic bool assertion instead. Add cluster import.

- [ ] **Step 13: Verify compilation and tests**

Run: `go build ./cluster/clusterproviders/natsstream/ && go test ./cluster/clusterproviders/natsstream/ -v -race -count=1 -short`
Expected: PASS

- [ ] **Step 14: Commit**

```bash
git add -A cluster/clusterproviders/natsstream/
git commit -m "feat(natsstream): use shared cluster.RoleType and cluster.RoleChangedListener, remove per-provider singleton"
```

## Chunk 5: etcd Provider Update

### Task 5: Update etcd to use shared types

The etcd provider differs from natskv/natsstream:
- `RoleType` is in `singleton.go` (lines 8-16), not `config.go`
- `RoleChangedListener` is in `config.go` (lines 15-18)
- No `roleMu` — must be added
- `updateLeadership` (lines 497-519) mutates `p.role` without locking
- `roleChangedChan` send is **blocking** — must convert to non-blocking `select`/`default`
- `safeRun` (lines 521-526) and `getRunTimeStack` (lines 530-533) are both in `etcd_provider.go`
- `startRoleChangedNotifyLoop` (line 571) also calls `safeRun`
- `etcd_provider_shutdown_test.go` (line 14) references `RoleType`

All changes must be done atomically.

**Files:**
- Delete: `cluster/clusterproviders/etcd/singleton.go`
- Modify: `cluster/clusterproviders/etcd/config.go`
- Modify: `cluster/clusterproviders/etcd/etcd_provider.go`
- Modify: `cluster/clusterproviders/etcd/etcd_provider_shutdown_test.go`

- [ ] **Step 1: Delete `singleton.go`**

```bash
rm cluster/clusterproviders/etcd/singleton.go
```

- [ ] **Step 2: Update `config.go` — remove local `RoleChangedListener`**

In `cluster/clusterproviders/etcd/config.go`:

Remove `RoleChangedListener` interface (lines 15-18):
```go
type RoleChangedListener interface {
	OnRoleChanged(RoleType)
}
```

Add `"github.com/awevoke/protoactor-go/cluster"` to imports.

Update `WithRoleChangedListener` (lines 37-42) parameter type:
```go
// Before:
func WithRoleChangedListener(l RoleChangedListener) Option {
// After:
func WithRoleChangedListener(l cluster.RoleChangedListener) Option {
```

Update `config` struct field (line 68):
```go
// Before:
RoleChanged   RoleChangedListener
// After:
RoleChanged   cluster.RoleChangedListener
```

- [ ] **Step 3: Update Provider struct (lines 20-43)**

```go
// Before:
	schedulers          []*SingletonScheduler
	role                RoleType
	roleChangedChan     chan RoleType
	roleChangedListener RoleChangedListener

// After:
	schedulers          []cluster.RoleChangedListener
	role                cluster.RoleType
	roleMu              sync.Mutex
	roleChangedChan     chan cluster.RoleType
	roleChangedListener cluster.RoleChangedListener
```

Add `"sync"` to imports (note: `"sync/atomic"` is already imported but `"sync"` is not).
Add `"github.com/awevoke/protoactor-go/cluster"` to imports.

- [ ] **Step 4: Update constructor (lines 73-83)**

```go
// Before:
	role:                Follower,
	roleChangedChan:     make(chan RoleType, 1),

// After:
	role:                cluster.RoleFollower,
	roleChangedChan:     make(chan cluster.RoleType, 1),
```

- [ ] **Step 5: Replace `RegisterSingletonScheduler` (lines 491-494)**

```go
// Before:
func (p *Provider) RegisterSingletonScheduler(scheduler *SingletonScheduler) {
	p.schedulers = append(p.schedulers, scheduler)
}

// After:
func (p *Provider) RegisterSingletonScheduler(listener cluster.RoleChangedListener) {
	if p.shutdown.Load() {
		return
	}
	p.roleMu.Lock()
	defer p.roleMu.Unlock()
	p.schedulers = append(p.schedulers, listener)
	if p.role == cluster.RoleLeader {
		cluster.SafeRunRoleChange(p.cluster.Logger(), func() {
			listener.OnRoleChanged(cluster.RoleLeader)
		})
	}
}
```

- [ ] **Step 6: Replace `updateLeadership` (lines 497-519)**

The network call (`fetchNodes`) must stay outside the lock. The role change + scheduler notification goes inside.

```go
// Before (lines 497-519):
func (p *Provider) updateLeadership() {
	role := Follower
	ns, err := p.fetchNodes()
	if err != nil {
		p.cluster.Logger().Error("Failed to fetch nodes in updateLeadership.", slog.Any("error", err))
	}
	if p.isLeaderOf(ns) {
		role = Leader
	}
	if role != p.role {
		p.cluster.Logger().Info("Role changed.", slog.String("from", p.role.String()), slog.String("to", role.String()))
		p.role = role
		p.roleChangedChan <- role
		for _, scheduler := range p.schedulers {
			safeRun(p.cluster.Logger(), func() {
				scheduler.OnRoleChanged(role)
			})
		}
	}
}

// After:
func (p *Provider) updateLeadership() {
	role := cluster.RoleFollower
	ns, err := p.fetchNodes()
	if err != nil {
		p.cluster.Logger().Error("Failed to fetch nodes in updateLeadership.", slog.Any("error", err))
	}
	if p.isLeaderOf(ns) {
		role = cluster.RoleLeader
	}

	p.roleMu.Lock()
	defer p.roleMu.Unlock()

	if role != p.role {
		p.cluster.Logger().Info("Role changed.", slog.String("from", p.role.String()), slog.String("to", role.String()))
		p.role = role
		select {
		case p.roleChangedChan <- role:
		default:
		}
		for _, scheduler := range p.schedulers {
			cluster.SafeRunRoleChange(p.cluster.Logger(), func() {
				scheduler.OnRoleChanged(role)
			})
		}
	}
}
```

Note: `Shutdown()` calls `updateLeadership()` after `p.cancel()`, which causes `fetchNodes()` to fail. This is safe — the method sets role to `RoleFollower` on failure, which is the desired shutdown behavior. No deadlock risk because `Shutdown` does not hold `roleMu`.

- [ ] **Step 7: Rewrite `startRoleChangedNotifyLoop` (lines 566-575)**

The current etcd implementation uses a bare channel receive that blocks forever on shutdown. Rewrite to use a `select` with channel close for clean shutdown:

```go
// Before (lines 566-575):
func (p *Provider) startRoleChangedNotifyLoop() {
	go func() {
		for !p.shutdown.Load() {
			role := <-p.roleChangedChan
			if lis := p.roleChangedListener; lis != nil {
				safeRun(p.cluster.Logger(), func() { lis.OnRoleChanged(role) })
			}
		}
	}()
}

// After:
func (p *Provider) startRoleChangedNotifyLoop() {
	go func() {
		for {
			role, ok := <-p.roleChangedChan
			if !ok || p.shutdown.Load() {
				return
			}
			if lis := p.roleChangedListener; lis != nil {
				cluster.SafeRunRoleChange(p.cluster.Logger(), func() { lis.OnRoleChanged(role) })
			}
		}
	}()
}
```

Also, in `Shutdown()`, close the channel after setting shutdown to true:
```go
p.shutdown.Store(true)
close(p.roleChangedChan)
```

Verify `Shutdown` does not already close the channel to avoid double-close. If needed, use a `sync.Once` for the close.

- [ ] **Step 8: Remove `safeRun` and `getRunTimeStack`**

Delete `safeRun` (lines 521-528) and `getRunTimeStack` (lines 530-533). `getRunTimeStack` is only used by `safeRun` — no other callers.

- [ ] **Step 9: Add compile-time interface assertion**

Add near the top of `etcd_provider.go`:
```go
var _ cluster.SingletonSchedulerRegistrar = (*Provider)(nil)
```

- [ ] **Step 10: Update all remaining `Follower`/`Leader`/`RoleType` references**

Replace throughout `etcd_provider.go`.

- [ ] **Step 11: Update `etcd_provider_shutdown_test.go`**

At line 14, update:
```go
// Before:
roleChangedChan: make(chan RoleType, 1),

// After:
roleChangedChan: make(chan cluster.RoleType, 1),
```

Add `"github.com/awevoke/protoactor-go/cluster"` to test imports.

- [ ] **Step 12: Verify compilation and tests**

Run: `go build ./cluster/clusterproviders/etcd/ && go test ./cluster/clusterproviders/etcd/ -v -race -count=1 -short`
Expected: PASS

- [ ] **Step 13: Commit**

```bash
git add -A cluster/clusterproviders/etcd/
git commit -m "feat(etcd): use shared cluster.RoleType and cluster.RoleChangedListener, add roleMu, remove per-provider singleton"
```

## Chunk 6: zk Provider Update

### Task 6: Update zk to use shared types

The zk provider has the largest delta:
- `RoleType` is defined inline in `zk_provider.go` (lines 19-34) with `String()` returning "LEADER"/"FOLLOWER"
- No `schedulers` field — must be added
- No `roleMu` — must be added
- No `RegisterSingletonScheduler` method — must be added from scratch
- `updateLeadership` (lines 335-345) has **no scheduler notification** — must be added from scratch
- `onEvent` (lines 347-359) directly mutates `p.role` without locks — must be wrapped
- `roleChangedChan` sends are **blocking** — must convert to non-blocking
- `safeRun` and `getRunTimeStack` are in `utils.go` (lines 57-70), not the provider file
- `startRoleChangedNotifyLoop` (line 329) calls `safeRun`
- `misc_test.go` (line 51) calls `safeRun` directly
- `config.go` has `RoleChangedListener` (lines 68-71), `OnRoleChangedFunc` adapter (lines 73-79), `WithRoleChangedListener` (line 40), and `WithRoleChangedFunc` (lines 47-49)

All changes must be done atomically.

**Files:**
- Delete: `cluster/clusterproviders/zk/singleton.go`
- Modify: `cluster/clusterproviders/zk/zk_provider.go`
- Modify: `cluster/clusterproviders/zk/config.go`
- Modify: `cluster/clusterproviders/zk/utils.go`
- Modify: `cluster/clusterproviders/zk/misc_test.go`

- [ ] **Step 1: Delete `singleton.go`**

```bash
rm cluster/clusterproviders/zk/singleton.go
```

- [ ] **Step 2: Update `config.go` — remove/update local types**

In `cluster/clusterproviders/zk/config.go`:

Remove `RoleChangedListener` interface (lines 68-71):
```go
type RoleChangedListener interface {
	OnRoleChanged(RoleType)
}
```

Update `OnRoleChangedFunc` adapter (lines 73-79):
```go
// Before:
type OnRoleChangedFunc func(RoleType)
func (fn OnRoleChangedFunc) OnRoleChanged(rt RoleType) {

// After:
type OnRoleChangedFunc func(cluster.RoleType)
func (fn OnRoleChangedFunc) OnRoleChanged(rt cluster.RoleType) {
```

Update `WithRoleChangedListener` (line 40):
```go
// Before:
func WithRoleChangedListener(l RoleChangedListener) Option {
// After:
func WithRoleChangedListener(l cluster.RoleChangedListener) Option {
```

Update `WithRoleChangedFunc` (lines 47-49) — the parameter type `OnRoleChangedFunc` stays the same name but now implements `cluster.RoleChangedListener` via its updated `OnRoleChanged` method.

Update `config` struct field (line 86):
```go
// Before:
RoleChanged    RoleChangedListener
// After:
RoleChanged    cluster.RoleChangedListener
```

Add `"github.com/awevoke/protoactor-go/cluster"` to imports.

- [ ] **Step 3: Remove inline `RoleType` from `zk_provider.go` (lines 19-34)**

Remove the `RoleType` type, `Follower`/`Leader` constants, and `String()` method.

- [ ] **Step 4: Update Provider struct (lines 37-53)**

```go
// Before:
	roleChangedListener RoleChangedListener
	role                RoleType
	roleChangedChan     chan RoleType

// After:
	roleChangedListener cluster.RoleChangedListener
	role                cluster.RoleType
	roleMu              sync.Mutex
	roleChangedChan     chan cluster.RoleType
	schedulers          []cluster.RoleChangedListener
```

Add `"github.com/awevoke/protoactor-go/cluster"` to imports.

- [ ] **Step 5: Update constructor (lines 73-74)**

```go
// Before:
	roleChangedChan: make(chan RoleType, 1),
	role:            Follower,

// After:
	roleChangedChan: make(chan cluster.RoleType, 1),
	role:            cluster.RoleFollower,
```

- [ ] **Step 6: Add `RegisterSingletonScheduler` method (new — does not exist today)**

```go
func (p *Provider) RegisterSingletonScheduler(listener cluster.RoleChangedListener) {
	if p.shutdown.Load() {
		return
	}
	p.roleMu.Lock()
	defer p.roleMu.Unlock()
	p.schedulers = append(p.schedulers, listener)
	if p.role == cluster.RoleLeader {
		cluster.SafeRunRoleChange(p.cluster.Logger(), func() {
			listener.OnRoleChanged(cluster.RoleLeader)
		})
	}
}
```

- [ ] **Step 7: Replace `updateLeadership` (lines 335-345)**

Currently has no scheduler notification and no locking. Full replacement:

```go
// Before (lines 335-345):
func (p *Provider) updateLeadership(ns []*Node) {
	role := Follower
	if p.isLeaderOf(ns) {
		role = Leader
	}
	if role != p.role {
		p.cluster.Logger().Info("Role changed.", slog.String("from", p.role.String()), slog.String("to", role.String()))
		p.role = role
		p.roleChangedChan <- role
	}
}

// After:
func (p *Provider) updateLeadership(ns []*Node) {
	role := cluster.RoleFollower
	if p.isLeaderOf(ns) {
		role = cluster.RoleLeader
	}

	p.roleMu.Lock()
	defer p.roleMu.Unlock()

	if role != p.role {
		p.cluster.Logger().Info("Role changed.", slog.String("from", p.role.String()), slog.String("to", role.String()))
		p.role = role
		select {
		case p.roleChangedChan <- role:
		default:
		}
		for _, scheduler := range p.schedulers {
			cluster.SafeRunRoleChange(p.cluster.Logger(), func() {
				scheduler.OnRoleChanged(role)
			})
		}
	}
}
```

- [ ] **Step 8: Replace `onEvent` (lines 347-359)**

`onEvent` is called from ZooKeeper's event callback goroutine (registered at line 76: `WithEventCallback(p.onEvent)`), so it runs concurrently with `updateLeadership`. Must use `roleMu` and notify schedulers. Full replacement:

```go
// Before (lines 347-359):
func (p *Provider) onEvent(evt zk.Event) {
	if evt.Type != zk.EventSession {
		return
	}
	switch evt.State {
	case zk.StateConnecting, zk.StateDisconnected, zk.StateExpired:
		if p.role == Leader {
			p.role = Follower
			p.roleChangedChan <- Follower
		}
	case zk.StateConnected, zk.StateHasSession:
	}
}

// After:
func (p *Provider) onEvent(evt zk.Event) {
	if evt.Type != zk.EventSession {
		return
	}
	switch evt.State {
	case zk.StateConnecting, zk.StateDisconnected, zk.StateExpired:
		p.roleMu.Lock()
		defer p.roleMu.Unlock()
		if p.role == cluster.RoleLeader {
			p.role = cluster.RoleFollower
			select {
			case p.roleChangedChan <- cluster.RoleFollower:
			default:
			}
			for _, scheduler := range p.schedulers {
				cluster.SafeRunRoleChange(p.cluster.Logger(), func() {
					scheduler.OnRoleChanged(cluster.RoleFollower)
				})
			}
		}
	case zk.StateConnected, zk.StateHasSession:
	}
}
```

- [ ] **Step 9: Rewrite `startRoleChangedNotifyLoop` (lines 324-332)**

The current zk implementation uses a bare channel receive that blocks forever on shutdown. Rewrite to use channel close:

```go
// Before (lines 324-332):
func (p *Provider) startRoleChangedNotifyLoop() {
	go func() {
		for !p.shutdown.Load() {
			role := <-p.roleChangedChan
			if lis := p.roleChangedListener; lis != nil {
				safeRun(p.cluster.Logger(), func() { lis.OnRoleChanged(role) })
			}
		}
	}()
}

// After:
func (p *Provider) startRoleChangedNotifyLoop() {
	go func() {
		for {
			role, ok := <-p.roleChangedChan
			if !ok || p.shutdown.Load() {
				return
			}
			if lis := p.roleChangedListener; lis != nil {
				cluster.SafeRunRoleChange(p.cluster.Logger(), func() { lis.OnRoleChanged(role) })
			}
		}
	}()
}
```

Also, in `Shutdown()`, close the channel after setting shutdown to true:
```go
p.shutdown.Store(true)
close(p.roleChangedChan)
```

Use a `sync.Once` if there's risk of double-close.

- [ ] **Step 10: Update `utils.go` — remove `safeRun` and `getRunTimeStack` (lines 57-70)**

Delete both functions. They are replaced by `cluster.SafeRunRoleChange`. No other callers in the zk package use these functions (verified: only `misc_test.go` line 51 and `zk_provider.go` line 329 reference `safeRun`).

- [ ] **Step 11: Update `misc_test.go` — remove `safeRun` test (line 51)**

Remove or replace the test at line 51:
```go
// Before:
suite.NotPanics(func() { safeRun(slog.Default(), func() { panic("don't worry, should panic here") }) })

// After:
suite.NotPanics(func() { cluster.SafeRunRoleChange(slog.Default(), func() { panic("don't worry, should panic here") }) })
```

Add `"github.com/awevoke/protoactor-go/cluster"` to test imports.

- [ ] **Step 12: Add compile-time interface assertion**

Add near the top of `zk_provider.go`:
```go
var _ cluster.SingletonSchedulerRegistrar = (*Provider)(nil)
```

- [ ] **Step 13: Update all remaining `Follower`/`Leader`/`RoleType` references**

Replace throughout `zk_provider.go`.

- [ ] **Step 14: Verify compilation and tests**

Run: `go build ./cluster/clusterproviders/zk/ && go test ./cluster/clusterproviders/zk/ -v -race -count=1 -short`
Expected: PASS

- [ ] **Step 15: Commit**

```bash
git add -A cluster/clusterproviders/zk/
git commit -m "feat(zk): use shared cluster.RoleType and cluster.RoleChangedListener, add roleMu/schedulers, remove per-provider singleton"
```

## Chunk 7: Integration Tests and Final Verification

### Task 7: Add late-registration integration tests

**Files:**
- Modify: `cluster/clusterproviders/natskv/natskv_provider_test.go`
- Modify: `cluster/clusterproviders/natsstream/natsstream_provider_test.go`

- [ ] **Step 1: Add natskv late-registration test**

In `natskv_provider_test.go`, add:

```go
func TestSingletonScheduler_RegisterAfterStart_SpawnsImmediately(t *testing.T) {
	// 1. setupCluster and call p.StartMember(c)
	// 2. Wait for leader election (require.Eventually checking p.IsLeader())
	// 3. Create scheduler: scheduler := cluster.NewSingletonScheduler(c.ActorSystem.Root)
	// 4. Register actor via scheduler.FromFunc(...)
	// 5. Register AFTER start: err := c.RegisterSingletonScheduler(scheduler)
	// 6. require.NoError(t, err)
	// 7. Assert actor spawns within timeout via require.Eventually
}
```

- [ ] **Step 2: Add natskv concurrent registration race test**

```go
func TestSingletonScheduler_ConcurrentRegisterAndRoleChange(t *testing.T) {
	// Start cluster, then concurrently:
	// - goroutine 1: register schedulers in a loop (100 iterations)
	// - goroutine 2: call p.setRole(cluster.RoleLeader) / p.setRole(cluster.RoleFollower)
	// Run with -race to verify no data races
	// Use sync.WaitGroup to coordinate goroutine completion
}
```

- [ ] **Step 3: Add natsstream late-registration test**

Same pattern as Step 1 for natsstream.

- [ ] **Step 4: Run integration tests**

Run: `go test ./cluster/clusterproviders/natskv/ -v -race -count=1 -run TestSingletonScheduler`
Run: `go test ./cluster/clusterproviders/natsstream/ -v -race -count=1 -run TestSingletonScheduler`
Expected: PASS (requires NATS testcontainer)

- [ ] **Step 5: Commit**

```bash
git add cluster/clusterproviders/natskv/natskv_provider_test.go cluster/clusterproviders/natsstream/natsstream_provider_test.go
git commit -m "test: add late-registration and concurrent race tests for singleton schedulers"
```

### Task 8: Full cross-package verification

- [ ] **Step 1: Build all packages**

Run: `go build ./...`
Expected: Success — no compilation errors anywhere.

- [ ] **Step 2: Run all tests**

Run: `go test ./... -race -count=1`
Expected: PASS across all packages.

- [ ] **Step 3: Verify no remaining references to old types**

Run these grep commands and verify no results:

```bash
# No per-provider SingletonScheduler types (excluding cluster.SingletonScheduler and test files)
grep -rn "SingletonScheduler" cluster/clusterproviders/ --include="*.go" | grep -v "cluster\." | grep -v "_test.go"

# No local safeRun functions
grep -rn "func safeRun" cluster/clusterproviders/ --include="*.go"

# No local getRunTimeStack functions
grep -rn "getRunTimeStack" cluster/clusterproviders/ --include="*.go"

# No bare Follower/Leader constants (should all be cluster.RoleFollower/cluster.RoleLeader)
grep -rn "\bFollower\b" cluster/clusterproviders/ --include="*.go" | grep -v "cluster\.RoleFollower" | grep -v "// "
grep -rn "\bLeader\b" cluster/clusterproviders/ --include="*.go" | grep -v "cluster\.RoleLeader" | grep -v "IsLeader" | grep -v "isLeader" | grep -v "leaderSeq" | grep -v "leaderMemberID" | grep -v "leaderBucket" | grep -v "leaderKey" | grep -v "refreshLeader" | grep -v "attemptLeader" | grep -v "RegisterLeader" | grep -v "// "

# No local RoleType definitions
grep -rn "type RoleType" cluster/clusterproviders/ --include="*.go"

# No local RoleChangedListener definitions (except OnRoleChangedFunc which is kept in zk)
grep -rn "type RoleChangedListener" cluster/clusterproviders/ --include="*.go"
```

Expected: No results for any of these.

- [ ] **Step 4: Commit if any remaining fixes**

```bash
git add -A
git commit -m "fix: address any remaining compilation or test issues from singleton rework"
```
