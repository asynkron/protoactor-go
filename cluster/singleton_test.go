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

type singletonTestActor struct{}

func (a *singletonTestActor) Receive(ctx actor.Context) {}

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
		return &singletonTestActor{}
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
