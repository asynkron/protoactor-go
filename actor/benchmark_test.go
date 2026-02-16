package actor_test

import (
	"sync"
	"testing"
	"time"

	. "github.com/asynkron/protoactor-go/actor"
	"go.opentelemetry.io/otel/attribute"
)

// BenchmarkActorSendMessage measures the throughput of fire-and-forget message
// sends through the full actor pipeline (mailbox enqueue, dispatch, invoke).
func BenchmarkActorSendMessage(b *testing.B) {
	system := NewActorSystem()
	defer system.Shutdown()

	var wg sync.WaitGroup
	wg.Add(b.N)

	pid := system.Root.Spawn(PropsFromFunc(func(ctx Context) {
		switch ctx.Message().(type) {
		case string:
			wg.Done()
		}
	}))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		system.Root.Send(pid, "hello")
	}
	wg.Wait()
}

// BenchmarkActorRequestResponse measures the latency of a full request/response
// round-trip through the actor system, including future allocation and resolution.
func BenchmarkActorRequestResponse(b *testing.B) {
	system := NewActorSystem()
	defer system.Shutdown()

	pid := system.Root.Spawn(PropsFromFunc(func(ctx Context) {
		switch ctx.Message().(type) {
		case string:
			ctx.Respond("reply")
		}
	}))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f := system.Root.RequestFuture(pid, "hello", 5*time.Second)
		if _, err := f.Result(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSpawnAndStop measures the cost of the full actor lifecycle: spawning
// (allocating the mailbox, registering in the process registry) and stopping
// (deregistration, sending/processing the stop system message).
func BenchmarkSpawnAndStop(b *testing.B) {
	system := NewActorSystem()
	defer system.Shutdown()

	props := PropsFromFunc(func(ctx Context) {})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pid := system.Root.Spawn(props)
		if err := system.Root.StopFuture(pid).Wait(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkActorSendMessage_Parallel measures message send throughput under
// concurrent producer load (multiple goroutines sending to one actor).
func BenchmarkActorSendMessage_Parallel(b *testing.B) {
	system := NewActorSystem()
	defer system.Shutdown()

	var wg sync.WaitGroup
	wg.Add(b.N)

	pid := system.Root.Spawn(PropsFromFunc(func(ctx Context) {
		switch ctx.Message().(type) {
		case string:
			wg.Done()
		}
	}))

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			system.Root.Send(pid, "hello")
		}
	})
	wg.Wait()
}

// BenchmarkSystemLabels measures the allocation cost of constructing
// OpenTelemetry attribute key-value pairs for system-level metrics labels.
func BenchmarkSystemLabels(b *testing.B) {
	system := NewActorSystem()
	defer system.Shutdown()

	b.ResetTimer()
	var result []attribute.KeyValue
	for i := 0; i < b.N; i++ {
		result = SystemLabels(system)
	}
	_ = result
}

// BenchmarkCommonLabels measures the allocation cost of constructing the full
// set of common metric labels (system labels + actor type). This exercises the
// fmt.Sprintf("%T", ...) call path that generates the actor type string.
func BenchmarkCommonLabels(b *testing.B) {
	system := NewActorSystem()
	defer system.Shutdown()

	m := NewMetrics(system, nil)

	// Spawn an actor and capture its context for use in the benchmark loop.
	var captured Context
	ready := make(chan struct{})
	pid := system.Root.Spawn(PropsFromFunc(func(ctx Context) {
		switch ctx.Message().(type) {
		case *Started:
			captured = ctx
			close(ready)
		}
	}))
	<-ready

	b.ResetTimer()
	var result []attribute.KeyValue
	for i := 0; i < b.N; i++ {
		result = m.CommonLabels(captured)
	}
	_ = result

	system.Root.Stop(pid)
}
