package natskv

import (
	"testing"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

func TestSingletonScheduler_FromFunc(t *testing.T) {
	system := actor.NewActorSystem()
	s := NewSingletonScheduler(system.Root)
	s.FromFunc(func(ctx actor.Context) {})
	assert.Len(t, s.props, 1)
}

func TestSingletonScheduler_FromProducer(t *testing.T) {
	system := actor.NewActorSystem()
	s := NewSingletonScheduler(system.Root)
	s.FromProducer(func() actor.Actor { return &testActor{} })
	assert.Len(t, s.props, 1)
}

func TestSingletonScheduler_OnRoleChanged_Leader(t *testing.T) {
	system := actor.NewActorSystem()
	s := NewSingletonScheduler(system.Root)
	s.FromFunc(func(ctx actor.Context) {})
	s.OnRoleChanged(Leader)
	assert.Len(t, s.pids, 1)
	assert.NotNil(t, s.pids[0])
}

func TestSingletonScheduler_OnRoleChanged_Follower(t *testing.T) {
	system := actor.NewActorSystem()
	s := NewSingletonScheduler(system.Root)
	s.FromFunc(func(ctx actor.Context) {})
	s.OnRoleChanged(Leader)
	assert.Len(t, s.pids, 1)
	s.OnRoleChanged(Follower)
	assert.Nil(t, s.pids)
}

type testActor struct{}

func (a *testActor) Receive(ctx actor.Context) {}
