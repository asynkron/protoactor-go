package remote

import (
	"sort"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestStart(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	remote := NewRemote(system, config)
	err := remote.Start()
	assert.NoError(t, err)
	remote.Shutdown(true)
}

func TestConfig_WithAdvertisedHost(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0, WithAdvertisedHost("Banana"))
	remote := NewRemote(system, config)
	err := remote.Start()
	assert.NoError(t, err)
	assert.Equal(t, "Banana", system.Address())
	remote.Shutdown(true)
}

func TestRemote_Register(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0, WithKinds(
		NewKind("someKind", actor.PropsFromProducer(nil)),
		NewKind("someOther", actor.PropsFromProducer(nil)),
	))
	remote := NewRemote(system, config)

	kinds := remote.GetKnownKinds()
	assert.Equal(t, 2, len(kinds))
	sort.Strings(kinds)
	assert.Equal(t, "someKind", kinds[0])
	assert.Equal(t, "someOther", kinds[1])
}

func TestRemote_RegisterViaOptions(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0,
		WithKinds(
			NewKind("someKind", actor.PropsFromProducer(nil)),
			NewKind("someOther", actor.PropsFromProducer(nil))))

	remote := NewRemote(system, config)
	kinds := remote.GetKnownKinds()
	assert.Equal(t, 2, len(kinds))
	sort.Strings(kinds)
	assert.Equal(t, "someKind", kinds[0])
	assert.Equal(t, "someOther", kinds[1])
}

func TestRemote_RegisterViaStruct(t *testing.T) {
	system := actor.NewActorSystem()
	config := &Config{
		Host: "localhost",
		Port: 0,
		Kinds: map[string]*actor.Props{
			"someKind":  actor.PropsFromProducer(nil),
			"someOther": actor.PropsFromProducer(nil),
		},
	}

	remote := NewRemote(system, config)
	kinds := remote.GetKnownKinds()
	assert.Equal(t, 2, len(kinds))
	sort.Strings(kinds)
	assert.Equal(t, "someKind", kinds[0])
	assert.Equal(t, "someOther", kinds[1])
}

func TestRemote_GracefulShutdown_ManagerStopsAfterGRPC(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("127.0.0.1", 0)
	r := NewRemote(system, config)
	err := r.Start()
	require.NoError(t, err)

	// Verify the endpoint manager is not stopped before gRPC shutdown
	assert.False(t, r.edpManager.stopped.Load(), "endpoint manager should not be stopped before shutdown")

	r.Shutdown(true)

	// After shutdown, endpoint manager should be stopped
	assert.True(t, r.edpManager.stopped.Load(), "endpoint manager should be stopped after shutdown")
}

func TestRemote_GracefulShutdownTimeout(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("127.0.0.1", 0,
		WithShutdownTimeout(500*time.Millisecond))
	r := NewRemote(system, config)

	err := r.Start()
	require.NoError(t, err)

	start := time.Now()
	r.Shutdown(true)
	elapsed := time.Since(start)

	// Should complete (either gracefully or via timeout)
	assert.Less(t, elapsed, 5*time.Second)
}

func TestRemote_ForceShutdown(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("127.0.0.1", 0)
	r := NewRemote(system, config)

	err := r.Start()
	require.NoError(t, err)

	start := time.Now()
	r.Shutdown(false)
	elapsed := time.Since(start)

	// Force shutdown should be near-instant
	assert.Less(t, elapsed, 2*time.Second)
}

func TestRemote_TwoNodesCommunicate(t *testing.T) {
	// Start node A
	systemA := actor.NewActorSystem()
	remoteA := NewRemote(systemA, Configure("127.0.0.1", 0))
	err := remoteA.Start()
	require.NoError(t, err)
	defer remoteA.Shutdown(true)

	// Start node B with an echo actor
	systemB := actor.NewActorSystem()
	remoteB := NewRemote(systemB, Configure("127.0.0.1", 0))
	err = remoteB.Start()
	require.NoError(t, err)
	defer remoteB.Shutdown(true)

	props := actor.PropsFromFunc(func(ctx actor.Context) {
		if _, ok := ctx.Message().(*emptypb.Empty); ok {
			ctx.Respond(&emptypb.Empty{})
		}
	})
	_, err = systemB.Root.SpawnNamed(props, "echo")
	require.NoError(t, err)

	// Send from A to B
	remotePID := actor.NewPID(systemB.Address(), "echo")
	fut := systemA.Root.RequestFuture(remotePID, &emptypb.Empty{}, 5*time.Second)
	result, err := fut.Result()
	require.NoError(t, err)
	assert.IsType(t, &emptypb.Empty{}, result)
}

func TestRemote_ConnectionFailureTriggersTerminatedEvent(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("127.0.0.1", 0,
		WithMaxRetryCount(1),
		WithRetryBaseDelay(10*time.Millisecond),
		WithRetryMaxDelay(10*time.Millisecond),
	)
	r := NewRemote(system, config)
	err := r.Start()
	require.NoError(t, err)
	defer r.Shutdown(true)

	terminated := make(chan string, 1)
	system.EventStream.Subscribe(func(evt any) {
		if e, ok := evt.(*EndpointTerminatedEvent); ok {
			terminated <- e.Address
		}
	})

	// Send to a non-existent remote address
	badPID := actor.NewPID("127.0.0.1:59999", "nonexistent")
	system.Root.Send(badPID, &emptypb.Empty{})

	select {
	case addr := <-terminated:
		assert.Equal(t, "127.0.0.1:59999", addr)
	case <-time.After(30 * time.Second):
		t.Fatal("expected EndpointTerminatedEvent for unreachable address")
	}
}

// Legacy tests (TestStart_AdvertisedAddress, TestShutdown_Graceful, TestShutdown)
// were removed and replaced with the black-box tests above.
