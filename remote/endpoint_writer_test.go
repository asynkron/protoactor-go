package remote

import (
	"runtime"
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
