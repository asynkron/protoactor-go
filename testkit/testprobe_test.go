package testkit

import (
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/require"
)

func TestProbeGetNextMessage(t *testing.T) {
	system := actor.NewActorSystem()
	probe := NewTestProbe()
	pid := system.Root.Spawn(actor.PropsFromProducer(func() actor.Actor { return probe }))

	system.Root.Send(pid, "hello")
	msg, err := GetNextMessageOf[string](probe, time.Second)
	require.NoError(t, err)
	require.Equal(t, "hello", msg)
}

func TestProbeFishForMessage(t *testing.T) {
	system := actor.NewActorSystem()
	probe := NewTestProbe()
	pid := system.Root.Spawn(actor.PropsFromProducer(func() actor.Actor { return probe }))

	system.Root.Send(pid, "a")
	system.Root.Send(pid, "b")
	msg, err := FishForMessage(probe, func(s string) bool { return s == "b" }, time.Second)
	require.NoError(t, err)
	require.Equal(t, "b", msg)
}

func TestProbeSender(t *testing.T) {
	system := actor.NewActorSystem()
	probe := NewTestProbe()
	probePID := system.Root.Spawn(actor.PropsFromProducer(func() actor.Actor { return probe }))

	sender := system.Root.Spawn(actor.PropsFromFunc(func(ctx actor.Context) {
		ctx.Request(probePID, "hi")
	}))

	msg, err := GetNextMessageOf[string](probe, time.Second)
	require.NoError(t, err)
	require.Equal(t, "hi", msg)
	require.Equal(t, sender, probe.Sender())
}

func TestProbeExpectNoMessage(t *testing.T) {
	system := actor.NewActorSystem()
	probe := NewTestProbe()
	pid := system.Root.Spawn(actor.PropsFromProducer(func() actor.Actor { return probe }))

	require.NoError(t, probe.ExpectNoMessage(50*time.Millisecond))

	system.Root.Send(pid, "hi")
	err := probe.ExpectNoMessage(50 * time.Millisecond)
	require.Error(t, err)
}
