package natsstream

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSingleton_FromFunc(t *testing.T) {
	system := actor.NewActorSystem()
	s := NewSingletonScheduler(system.Root)
	s.FromFunc(func(ctx actor.Context) {})
	s.Lock()
	defer s.Unlock()
	assert.Len(t, s.props, 1)
}

func TestSingleton_FromProducer(t *testing.T) {
	system := actor.NewActorSystem()
	s := NewSingletonScheduler(system.Root)
	s.FromProducer(func() actor.Actor { return &dummyActor{} })
	s.Lock()
	defer s.Unlock()
	assert.Len(t, s.props, 1)
}

func TestSingleton_OnRoleChanged_Leader_Spawns(t *testing.T) {
	system := actor.NewActorSystem()
	var spawned atomic.Bool
	s := NewSingletonScheduler(system.Root)
	s.FromFunc(func(ctx actor.Context) {
		if _, ok := ctx.Message().(*actor.Started); ok {
			spawned.Store(true)
		}
	})

	s.OnRoleChanged(Leader)

	require.Eventually(t, func() bool {
		return spawned.Load()
	}, 2*time.Second, 50*time.Millisecond)

	s.Lock()
	assert.Len(t, s.pids, 1)
	assert.NotNil(t, s.pids[0])
	s.Unlock()
}

func TestSingleton_OnRoleChanged_Follower_Poisons(t *testing.T) {
	system := actor.NewActorSystem()
	s := NewSingletonScheduler(system.Root)
	s.FromFunc(func(ctx actor.Context) {})

	s.OnRoleChanged(Leader)
	s.Lock()
	require.Len(t, s.pids, 1)
	s.Unlock()

	s.OnRoleChanged(Follower)
	s.Lock()
	assert.Nil(t, s.pids)
	s.Unlock()
}

type dummyActor struct{}

func (d *dummyActor) Receive(ctx actor.Context) {}
