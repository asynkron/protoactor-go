package remote

import (
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
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

// TestEndToEnd_UnknownKind_RemoteSpawn verifies that spawning an unknown kind
// on a remote node returns an error response without crashing the activator.
func TestEndToEnd_UnknownKind_RemoteSpawn(t *testing.T) {
	systemA := actor.NewActorSystem()
	configA := Configure("127.0.0.1", 0)
	remoteA := NewRemote(systemA, configA)
	err := remoteA.Start()
	require.NoError(t, err)
	defer remoteA.Shutdown(true)

	systemB := actor.NewActorSystem()
	configB := Configure("127.0.0.1", 0)
	remoteB := NewRemote(systemB, configB)
	// Register a known kind on B so the node is functional
	remoteB.Register("echo", actor.PropsFromFunc(func(ctx actor.Context) {}))
	err = remoteB.Start()
	require.NoError(t, err)
	defer remoteB.Shutdown(true)

	// Try to spawn an unknown kind on B from A
	resp, err := remoteA.SpawnNamed(systemB.Address(), "test", "nonexistent-kind", 5*time.Second)
	require.NoError(t, err, "should get a response, not a transport error")
	assert.Equal(t, ResponseStatusCodeERROR.ToInt32(), resp.StatusCode,
		"unknown kind should return ERROR status")

	// Verify the activator on B is still alive by spawning a known kind
	resp, err = remoteA.SpawnNamed(systemB.Address(), "recovery-test", "echo", 5*time.Second)
	require.NoError(t, err)
	assert.NotNil(t, resp.Pid, "known kind spawn should succeed after unknown kind error")
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

// TestActivator_SpawnNameConflict verifies that duplicate spawn names return
// the appropriate status code without panicking.
func TestActivator_SpawnNameConflict(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("127.0.0.1", 0)
	r := NewRemote(system, config)

	echoProps := actor.PropsFromFunc(func(ctx actor.Context) {})
	r.Register("echo", echoProps)

	props := actor.PropsFromProducer(newActivatorActor(r))
	activatorPID, err := system.Root.SpawnNamed(props, "test-activator-conflict")
	require.NoError(t, err)
	defer system.Root.Stop(activatorPID)

	// First spawn should succeed
	fut := system.Root.RequestFuture(activatorPID, &ActorPidRequest{
		Kind: "echo",
		Name: "duplicate-name",
	}, 5*time.Second)
	res, err := fut.Result()
	require.NoError(t, err)
	resp := res.(*ActorPidResponse)
	assert.NotNil(t, resp.Pid, "first spawn should succeed")

	// Second spawn with same name should return PROCESSNAMEALREADYEXIST
	fut = system.Root.RequestFuture(activatorPID, &ActorPidRequest{
		Kind: "echo",
		Name: "duplicate-name",
	}, 5*time.Second)
	res, err = fut.Result()
	require.NoError(t, err)
	resp = res.(*ActorPidResponse)
	assert.Equal(t, ResponseStatusCodePROCESSNAMEALREADYEXIST.ToInt32(), resp.StatusCode,
		"duplicate name should return PROCESSNAMEALREADYEXIST status")
}

// TestActivator_ActivatorUnavailableError verifies that ErrActivatorUnavailable
// does not cause a panic (DoNotPanic=true).
func TestActivator_ActivatorUnavailableError(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("127.0.0.1", 0)
	r := NewRemote(system, config)

	// Register a kind whose spawn function always returns ErrActivatorUnavailable.
	// We use WithSpawnFunc to override the default spawner.
	failProps := actor.PropsFromFunc(func(ctx actor.Context) {},
		actor.WithSpawnFunc(func(actorSystem *actor.ActorSystem, id string, props *actor.Props, parentContext actor.SpawnerContext) (*actor.PID, error) {
			return nil, ErrActivatorUnavailable
		}),
	)
	r.Register("unavailable-kind", failProps)

	props := actor.PropsFromProducer(newActivatorActor(r))
	activatorPID, err := system.Root.SpawnNamed(props, "test-activator-unavail")
	require.NoError(t, err)
	defer system.Root.Stop(activatorPID)

	fut := system.Root.RequestFuture(activatorPID, &ActorPidRequest{
		Kind: "unavailable-kind",
		Name: "test",
	}, 5*time.Second)
	res, err := fut.Result()
	require.NoError(t, err)
	resp := res.(*ActorPidResponse)
	assert.Equal(t, ResponseStatusCodeUNAVAILABLE.ToInt32(), resp.StatusCode,
		"unavailable activator should return UNAVAILABLE status")
}

// TestActivator_UnknownMessage verifies the activator handles unexpected
// message types without panicking.
func TestActivator_UnknownMessage(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("127.0.0.1", 0)
	r := NewRemote(system, config)

	props := actor.PropsFromProducer(newActivatorActor(r))
	activatorPID, err := system.Root.SpawnNamed(props, "test-activator-unknown-msg")
	require.NoError(t, err)
	defer system.Root.Stop(activatorPID)

	// Send a completely unexpected message type — should not panic
	assert.NotPanics(t, func() {
		system.Root.Send(activatorPID, "unexpected-string-message")
		time.Sleep(100 * time.Millisecond) // give it time to process
	})
}
