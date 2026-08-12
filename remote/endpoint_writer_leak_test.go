package remote

import (
	"runtime"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countGRPCGoroutines returns the number of live goroutines whose stack passes
// through gRPC client-connection machinery (transport, addrConn reconnect loop,
// or the per-ClientConn callback serializer). A ClientConn that is never closed
// keeps these goroutines alive indefinitely, so this count is a direct proxy for
// leaked ClientConns.
func countGRPCGoroutines() int {
	buf := make([]byte, 1<<22)
	n := runtime.Stack(buf, true)
	dump := string(buf[:n])
	var count int
	for _, marker := range []string{
		"grpc/internal/grpcsync.(*CallbackSerializer).run",
		"grpc.(*addrConn).resetTransportAndUnlock",
		"grpc/internal/transport.(*http2Client).keepalive",
	} {
		count += countSubstr(dump, marker)
	}
	return count
}

func countSubstr(s, sub string) int {
	var c, idx int
	for {
		i := indexFrom(s, sub, idx)
		if i < 0 {
			return c
		}
		c++
		idx = i + len(sub)
	}
}

func indexFrom(s, sub string, from int) int {
	if from >= len(s) {
		return -1
	}
	i := indexOf(s[from:], sub)
	if i < 0 {
		return -1
	}
	return from + i
}

func indexOf(s, sub string) int {
	n := len(sub)
	if n == 0 {
		return 0
	}
	for i := 0; i+n <= len(s); i++ {
		if s[i:i+n] == sub {
			return i
		}
	}
	return -1
}

// TestEndpointWriter_NoClientConnLeakOnTerminate is a regression test for the
// production defect the natskv restart-resilience soak caught: an endpointWriter
// toward an unreachable peer creates a gRPC ClientConn per connect attempt and
// leaks it. The synchronous connect-retry loop in initialize() also blocks the
// actor mailbox, so the EndpointTerminatedEvent that should stop the writer (and
// close its ClientConn) is not processed until the loop finishes. Under sustained
// membership churn every surviving member accumulates these unclosed ClientConns
// toward every recently-dead peer, so the process leaks gRPC connection
// goroutines without bound.
//
// The test drives many short-lived writers toward an unreachable address,
// terminating each promptly, and asserts the gRPC connection-goroutine
// population does not grow with the number of writers created.
func TestEndpointWriter_NoClientConnLeakOnTerminate(t *testing.T) {
	system := actor.NewActorSystem()
	// Multiple retries per writer: each failed attempt in the pre-fix code
	// orphaned its ClientConn, so the leak scales with retries as well as with
	// the number of writers.
	config := Configure("127.0.0.1", 0,
		WithMaxRetryCount(3),
		WithRetryBaseDelay(5*time.Millisecond),
		WithRetryMaxDelay(20*time.Millisecond),
	)
	r := NewRemote(system, config)
	err := r.Start()
	require.NoError(t, err)
	defer r.Shutdown(true)

	// An address nothing is listening on: connect attempts always fail.
	const deadAddr = "127.0.0.1:1"

	spawnAndTerminate := func() {
		props := actor.PropsFromProducer(
			endpointWriterProducer(r, deadAddr, r.config),
			actor.WithMailbox(endpointWriterMailboxProducer(
				r.config.EndpointWriterBatchSize, r.config.EndpointWriterQueueSize)),
		)
		pid := system.Root.Spawn(props)
		// Terminate the writer while it is (or has just been) trying to connect,
		// mirroring a member-left event arriving during the connect window.
		system.Root.Send(pid, &EndpointTerminatedEvent{Address: deadAddr})
		_ = system.Root.PoisonFuture(pid).Wait()
	}

	// Warm up so the baseline reflects steady state, then measure.
	for i := 0; i < 5; i++ {
		spawnAndTerminate()
	}
	settle(t)
	before := countGRPCGoroutines()

	const iterations = 40
	for i := 0; i < iterations; i++ {
		spawnAndTerminate()
	}
	settle(t)
	after := countGRPCGoroutines()

	// A bounded transient residue is acceptable; growth proportional to the
	// number of writers created is the leak. Allow generous slack for
	// in-flight teardown, but far below the ~1 leaked ClientConn per writer
	// (each with multiple goroutines) that the pre-fix code produced.
	assert.LessOrEqual(t, after, before+8,
		"gRPC connection goroutines grew with writer count (before=%d after=%d over %d writers): "+
			"leaked ClientConns toward unreachable peer", before, after, iterations)
}

// settle waits for terminated writers' goroutines to exit and forces GC so the
// measurement reflects settled state rather than in-flight teardown.
func settle(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	prev := -1
	for time.Now().Before(deadline) {
		runtime.GC()
		time.Sleep(150 * time.Millisecond)
		n := countGRPCGoroutines()
		if n == prev {
			return
		}
		prev = n
	}
}
