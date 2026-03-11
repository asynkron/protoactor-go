package remote

import (
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestActivator_UnknownKind_DoesNotPanic verifies that requesting an
// unregistered kind returns an error response without panicking.
func TestActivator_UnknownKind_DoesNotPanic(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("127.0.0.1", 0)
	r := NewRemote(system, config)
	// Do NOT register any kinds — all kinds are unknown

	// Spawn the activator directly
	props := actor.PropsFromProducer(newActivatorActor(r))
	activatorPID, err := system.Root.SpawnNamed(props, "test-activator")
	require.NoError(t, err)
	defer system.Root.Stop(activatorPID)

	// Request activation for an unknown kind
	fut := system.Root.RequestFuture(activatorPID, &ActorPidRequest{
		Kind: "nonexistent-kind",
		Name: "test-actor",
	}, 5*time.Second)

	res, err := fut.Result()
	require.NoError(t, err, "activator should respond without error")

	resp, ok := res.(*ActorPidResponse)
	require.True(t, ok, "response should be *ActorPidResponse")
	assert.Equal(t, ResponseStatusCodeERROR.ToInt32(), resp.StatusCode,
		"response should indicate error status")
	assert.Nil(t, resp.Pid, "response should not contain a PID")
}

// TestActivator_UnknownKind_ActorSurvives verifies the activator continues
// processing requests after handling an unknown kind.
func TestActivator_UnknownKind_ActorSurvives(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("127.0.0.1", 0)
	r := NewRemote(system, config)

	// Register one kind
	echoProps := actor.PropsFromFunc(func(ctx actor.Context) {})
	r.Register("echo", echoProps)

	props := actor.PropsFromProducer(newActivatorActor(r))
	activatorPID, err := system.Root.SpawnNamed(props, "test-activator2")
	require.NoError(t, err)
	defer system.Root.Stop(activatorPID)

	// First: request an unknown kind
	fut := system.Root.RequestFuture(activatorPID, &ActorPidRequest{
		Kind: "nonexistent",
		Name: "test1",
	}, 5*time.Second)
	res, err := fut.Result()
	require.NoError(t, err)
	resp := res.(*ActorPidResponse)
	assert.Equal(t, ResponseStatusCodeERROR.ToInt32(), resp.StatusCode)

	// Second: request a known kind — activator should still be alive
	fut = system.Root.RequestFuture(activatorPID, &ActorPidRequest{
		Kind: "echo",
		Name: "test2",
	}, 5*time.Second)
	res, err = fut.Result()
	require.NoError(t, err)
	resp = res.(*ActorPidResponse)
	assert.NotNil(t, resp.Pid, "known kind should produce a PID")
}

// TestActivator_Ping verifies basic activator health check.
func TestActivator_Ping(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("127.0.0.1", 0)
	r := NewRemote(system, config)

	props := actor.PropsFromProducer(newActivatorActor(r))
	activatorPID, err := system.Root.SpawnNamed(props, "test-activator3")
	require.NoError(t, err)
	defer system.Root.Stop(activatorPID)

	fut := system.Root.RequestFuture(activatorPID, &Ping{}, 5*time.Second)
	res, err := fut.Result()
	require.NoError(t, err)
	_, ok := res.(*Pong)
	assert.True(t, ok, "ping should return pong")
}
