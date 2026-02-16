package actor

import (
	"sync"
	"testing"

	"github.com/asynkron/protoactor-go/internal/queue/mpsc"
)

// benchInvoker is a minimal MessageInvoker for benchmarking mailbox throughput.
type benchInvoker struct {
	wg *sync.WaitGroup
}

func (bi *benchInvoker) InvokeSystemMessage(interface{}) {}
func (bi *benchInvoker) InvokeUserMessage(interface{}) {
	bi.wg.Done()
}
func (bi *benchInvoker) EscalateFailure(_ interface{}, _ interface{}) {}

// BenchmarkUnboundedMailbox_PostAndProcess measures the throughput of the
// unbounded (goring) mailbox: posting messages from a single producer and
// processing them through the dispatcher.
func BenchmarkUnboundedMailbox_PostAndProcess(b *testing.B) {
	var wg sync.WaitGroup
	wg.Add(b.N)

	inv := &benchInvoker{wg: &wg}
	p := Unbounded()
	mb := p()
	mb.RegisterHandlers(inv, NewDefaultDispatcher(300))
	mb.Start()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mb.PostUserMessage("msg")
	}
	wg.Wait()
}

// BenchmarkBoundedMailbox_PostAndProcess measures the throughput of the
// bounded (ring buffer) mailbox with a large capacity to avoid drops.
func BenchmarkBoundedMailbox_PostAndProcess(b *testing.B) {
	var wg sync.WaitGroup
	wg.Add(b.N)

	inv := &benchInvoker{wg: &wg}
	// Use a large capacity to avoid message drops during benchmark.
	p := Bounded(b.N + 1024)
	mb := p()
	mb.RegisterHandlers(inv, NewDefaultDispatcher(300))
	mb.Start()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mb.PostUserMessage("msg")
	}
	wg.Wait()
}

// BenchmarkUnboundedLockfreeMailbox_PostAndProcess measures the throughput of
// the lock-free (MPSC-based) unbounded mailbox variant.
func BenchmarkUnboundedLockfreeMailbox_PostAndProcess(b *testing.B) {
	var wg sync.WaitGroup
	wg.Add(b.N)

	inv := &benchInvoker{wg: &wg}
	p := UnboundedLockfree()
	mb := p()
	mb.RegisterHandlers(inv, NewDefaultDispatcher(300))
	mb.Start()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mb.PostUserMessage("msg")
	}
	wg.Wait()
}

// BenchmarkMPSCQueue_PushPop benchmarks the raw MPSC queue push/pop operations
// without any mailbox or dispatcher overhead. Single-producer, single-consumer.
func BenchmarkMPSCQueue_PushPop(b *testing.B) {
	q := mpsc.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Push("msg")
		q.Pop()
	}
}

// BenchmarkMPSCQueue_Push benchmarks just the push side of the MPSC queue,
// which is the multi-producer safe path.
func BenchmarkMPSCQueue_Push(b *testing.B) {
	q := mpsc.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Push("msg")
	}
}
