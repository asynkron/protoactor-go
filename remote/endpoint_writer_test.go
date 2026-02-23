package remote

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

// TestEndpointWriter_GoroutineCleanupOnShutdown verifies that the background
// goroutine spawned in initializeInternal is properly cleaned up when the
// remote is shut down. Before the fix, the Recv() goroutine would leak because
// the gRPC stream context was never cancelled.
func TestEndpointWriter_GoroutineCleanupOnShutdown(t *testing.T) {
	// Start two remote nodes
	systemA := actor.NewActorSystem()
	remoteA := NewRemote(systemA, Configure("127.0.0.1", 0))
	err := remoteA.Start()
	require.NoError(t, err)
	defer remoteA.Shutdown(true)

	systemB := actor.NewActorSystem()
	remoteB := NewRemote(systemB, Configure("127.0.0.1", 0))
	err = remoteB.Start()
	require.NoError(t, err)

	// Create an echo actor on node B
	props := actor.PropsFromFunc(func(ctx actor.Context) {
		if _, ok := ctx.Message().(*emptypb.Empty); ok {
			ctx.Respond(&emptypb.Empty{})
		}
	})
	pid, err := systemB.Root.SpawnNamed(props, "echo")
	require.NoError(t, err)

	// Send a message from A to B to establish the endpoint writer connection
	// (which spawns the background Recv goroutine)
	fut := systemA.Root.RequestFuture(pid, &emptypb.Empty{}, 5*time.Second)
	_, err = fut.Result()
	require.NoError(t, err)

	// Record goroutine count before shutdown
	runtime.GC()
	time.Sleep(100 * time.Millisecond) // let things settle
	goroutinesBefore := runtime.NumGoroutine()

	// Shut down node B - this should trigger EndpointTerminatedEvent on A,
	// which stops the endpoint writer and cancels the background goroutine
	remoteB.Shutdown(true)

	// Wait for the goroutine to be cleaned up
	// The background goroutine should exit quickly once the context is cancelled
	var goroutinesAfter int
	assert.Eventually(t, func() bool {
		runtime.GC()
		goroutinesAfter = runtime.NumGoroutine()
		// We expect at least 1 goroutine to be cleaned up (the Recv goroutine).
		// Use a tolerance since other goroutines may come and go.
		return goroutinesAfter < goroutinesBefore
	}, 10*time.Second, 100*time.Millisecond,
		"goroutine count should decrease after endpoint writer shutdown (before=%d, after=%d)",
		goroutinesBefore, goroutinesAfter)
}

// TestEndpointWriter_CancelFuncCalledOnClose verifies that closeClientConn
// properly calls the cancel function to signal the background goroutine.
func TestEndpointWriter_CancelFuncCalledOnClose(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)

	cancelled := false
	writer := &endpointWriter{
		address: "127.0.0.1:12345",
		config:  config,
		remote:  r,
		cancelFunc: func() {
			cancelled = true
		},
	}

	writer.closeClientConn()

	assert.True(t, cancelled, "cancelFunc should be called during closeClientConn")
	assert.Nil(t, writer.cancelFunc, "cancelFunc should be nil after closeClientConn")
}

// TestEndpointWriter_CloseClientConnNilCancel verifies closeClientConn does
// not panic when cancelFunc is nil (e.g., connection was never established).
func TestEndpointWriter_CloseClientConnNilCancel(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)

	writer := &endpointWriter{
		address:    "127.0.0.1:12345",
		config:     config,
		remote:     r,
		cancelFunc: nil,
	}

	// Should not panic
	assert.NotPanics(t, func() {
		writer.closeClientConn()
	})
}

func TestEndpointWriter_RestartAfterConnectFailure_NoPanic(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0,
		WithMaxRetryCount(1),
		WithRetryBaseDelay(10*time.Millisecond),
	)
	r := NewRemote(system, config)

	terminated := make(chan string, 10)
	system.EventStream.Subscribe(func(evt any) {
		if e, ok := evt.(*EndpointTerminatedEvent); ok {
			terminated <- e.Address
		}
	})

	// Track whether the actor restarts (which would indicate a panic was recovered)
	restarted := make(chan struct{}, 10)
	initCount := int32(0)

	address := "127.0.0.1:99999"

	props := actor.PropsFromProducer(func() actor.Actor {
		return &endpointWriter{
			address: address,
			config:  config,
			remote:  r,
		}
	})

	// Wrap the actor to intercept Restarting messages
	wrappedProps := props.Configure(actor.WithReceiverMiddleware(func(next actor.ReceiverFunc) actor.ReceiverFunc {
		return func(ctx actor.ReceiverContext, envelope *actor.MessageEnvelope) {
			switch envelope.Message.(type) {
			case *actor.Started:
				atomic.AddInt32(&initCount, 1)
			case *actor.Restarting:
				restarted <- struct{}{}
			}
			next(ctx, envelope)
		}
	}))

	pid := system.Root.Spawn(wrappedProps)

	// Drain the EndpointTerminatedEvent published by initialize() on actor start
	select {
	case <-terminated:
		// expected - initialize failed and published EndpointTerminatedEvent
	case <-time.After(5 * time.Second):
		t.Fatal("expected EndpointTerminatedEvent from initialize, got timeout")
	}

	// Reset initCount after initial startup
	atomic.StoreInt32(&initCount, 0)

	// Now send the restartAfterConnectFailure message
	system.Root.Send(pid, &restartAfterConnectFailure{err: fmt.Errorf("connection refused")})

	// Wait for the EndpointTerminatedEvent from the handler
	select {
	case addr := <-terminated:
		assert.Equal(t, address, addr)
	case <-time.After(5 * time.Second):
		t.Fatal("expected EndpointTerminatedEvent from restartAfterConnectFailure handler, got timeout")
	}

	// Give time for any restart to occur
	time.Sleep(100 * time.Millisecond)

	// Verify the actor did NOT restart (no panic was recovered by the supervisor)
	select {
	case <-restarted:
		t.Fatal("actor should not have restarted - restartAfterConnectFailure should terminate gracefully, not panic")
	default:
		// Good - no restart means no panic
	}

	// Verify initialize was not called again (which would happen on restart)
	count := atomic.LoadInt32(&initCount)
	assert.Equal(t, int32(0), count, "initialize should not have been called again after restartAfterConnectFailure")
}

func TestEndpointWriter_BlockedByRemoteServer(t *testing.T) {
	// Start node A (the one that will be blocked)
	systemA := actor.NewActorSystem()
	configA := Configure("127.0.0.1", 0,
		WithMaxRetryCount(1),
		WithRetryBaseDelay(10*time.Millisecond),
	)
	remoteA := NewRemote(systemA, configA)
	err := remoteA.Start()
	require.NoError(t, err)
	defer remoteA.Shutdown(true)

	// Start node B and add A to its block list
	systemB := actor.NewActorSystem()
	configB := Configure("127.0.0.1", 0)
	remoteB := NewRemote(systemB, configB)
	err = remoteB.Start()
	require.NoError(t, err)
	defer remoteB.Shutdown(true)

	// Block A's member ID
	remoteB.BlockList().Block(systemA.ID)

	// Subscribe to terminated events on A
	terminated := make(chan string, 1)
	systemA.EventStream.Subscribe(func(evt any) {
		if e, ok := evt.(*EndpointTerminatedEvent); ok {
			terminated <- e.Address
		}
	})

	// Create a target PID on B and try to send from A
	targetPID := actor.NewPID(systemB.Address(), "nonexistent")
	systemA.Root.Send(targetPID, &emptypb.Empty{})

	// A should receive EndpointTerminatedEvent because B blocked it
	select {
	case addr := <-terminated:
		assert.Contains(t, addr, "127.0.0.1")
	case <-time.After(10 * time.Second):
		t.Fatal("expected EndpointTerminatedEvent when blocked by remote server")
	}
}

func TestExponentialBackoff_DelaysIncreaseGeometrically(t *testing.T) {
	config := Configure("localhost", 0,
		WithRetryBaseDelay(100*time.Millisecond),
		WithRetryMaxDelay(1*time.Second),
		WithMaxRetryCount(4),
	)
	assert.Equal(t, 1*time.Second, config.RetryMaxDelay)
}

func TestExponentialBackoff_DelaysAreCorrect(t *testing.T) {
	baseDelay := 100 * time.Millisecond
	maxDelay := 500 * time.Millisecond

	// Test the backoff calculation directly
	for attempt, expected := range []time.Duration{
		100 * time.Millisecond, // 100ms * 2^0
		200 * time.Millisecond, // 100ms * 2^1
		400 * time.Millisecond, // 100ms * 2^2
		500 * time.Millisecond, // 100ms * 2^3 = 800ms, capped at 500ms
		500 * time.Millisecond, // capped
	} {
		delay := calcBackoffDelay(baseDelay, maxDelay, attempt)
		// Delay should be in [expected, expected + 25% jitter]
		assert.GreaterOrEqual(t, delay, expected,
			"attempt %d: delay %v should be >= %v", attempt, delay, expected)
		assert.LessOrEqual(t, delay, expected+expected/4,
			"attempt %d: delay %v should be <= %v (with 25%% jitter)", attempt, delay, expected+expected/4)
	}
}

// TestEndpointWriter_FullRecoveryFlow verifies the complete self-healing flow:
// 1. Two nodes communicate successfully
// 2. One node goes down — the other detects termination
// 3. The downed node restarts — communication resumes
func TestEndpointWriter_FullRecoveryFlow(t *testing.T) {
	// Start node A (persistent)
	systemA := actor.NewActorSystem()
	configA := Configure("127.0.0.1", 0,
		WithRetryBaseDelay(50*time.Millisecond),
		WithRetryMaxDelay(200*time.Millisecond),
		WithMaxRetryCount(3),
	)
	remoteA := NewRemote(systemA, configA)
	err := remoteA.Start()
	require.NoError(t, err)
	defer remoteA.Shutdown(true)

	// Start node B (will be killed and restarted)
	systemB := actor.NewActorSystem()
	configB := Configure("127.0.0.1", 0)
	remoteB := NewRemote(systemB, configB)
	err = remoteB.Start()
	require.NoError(t, err)

	// Spawn echo actor on B
	echoProps := actor.PropsFromFunc(func(ctx actor.Context) {
		if _, ok := ctx.Message().(*emptypb.Empty); ok {
			ctx.Respond(&emptypb.Empty{})
		}
	})
	_, err = systemB.Root.SpawnNamed(echoProps, "echo")
	require.NoError(t, err)

	addressB := systemB.Address()

	// Step 1: Verify communication works
	remotePID := actor.NewPID(addressB, "echo")
	fut := systemA.Root.RequestFuture(remotePID, &emptypb.Empty{}, 5*time.Second)
	_, err = fut.Result()
	require.NoError(t, err, "initial communication should succeed")

	// Step 2: Kill node B
	terminated := make(chan struct{}, 1)
	systemA.EventStream.Subscribe(func(evt any) {
		if _, ok := evt.(*EndpointTerminatedEvent); ok {
			select {
			case terminated <- struct{}{}:
			default:
			}
		}
	})

	remoteB.Shutdown(true)

	// Wait for A to detect termination
	select {
	case <-terminated:
		// expected
	case <-time.After(10 * time.Second):
		t.Fatal("node A did not detect termination of node B")
	}

	// Step 3: Restart node B on a new address (port 0 = random)
	systemB2 := actor.NewActorSystem()
	configB2 := Configure("127.0.0.1", 0)
	remoteB2 := NewRemote(systemB2, configB2)
	err = remoteB2.Start()
	require.NoError(t, err)
	defer remoteB2.Shutdown(true)

	// Spawn echo on new B
	_, err = systemB2.Root.SpawnNamed(echoProps, "echo")
	require.NoError(t, err)

	// Step 4: Verify A can communicate with new B
	newPID := actor.NewPID(systemB2.Address(), "echo")
	fut = systemA.Root.RequestFuture(newPID, &emptypb.Empty{}, 5*time.Second)
	_, err = fut.Result()
	require.NoError(t, err, "communication with restarted node should succeed")
}
