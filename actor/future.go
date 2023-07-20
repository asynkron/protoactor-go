package actor

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/asynkron/protoactor-go/log"
	"github.com/asynkron/protoactor-go/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// ErrTimeout is the error used when a future times out before receiving a result.
var ErrTimeout = errors.New("future: timeout")

// ErrDeadLetter is meaning you request to a unreachable PID.
var ErrDeadLetter = errors.New("future: dead letter")

// NewFuture creates and returns a new actor.Future with a timeout of duration d.
func NewFuture(actorSystem *ActorSystem, d time.Duration) *Future {
	ref := &futureProcess{Future{actorSystem: actorSystem, cond: sync.NewCond(&sync.Mutex{})}}
	id := actorSystem.ProcessRegistry.NextId()

	pid, ok := actorSystem.ProcessRegistry.Add(ref, "future"+id)
	if !ok {
		plog.Error("failed to register future process", log.Stringer("pid", pid))
	}

	sysMetrics, ok := actorSystem.Extensions.Get(extensionId).(*Metrics)
	if ok && sysMetrics.enabled {
		if instruments := sysMetrics.metrics.Get(metrics.InternalActorMetrics); instruments != nil {
			ctx := context.Background()
			labels := []attribute.KeyValue{
				attribute.String("address", ref.actorSystem.Address()),
			}

			instruments.FuturesStartedCount.Add(ctx, 1, metric.WithAttributes(labels...))
		}
	}

	ref.pid = pid

	if d >= 0 {
		tp := time.AfterFunc(d, func() {
			ref.cond.L.Lock()
			if ref.done {
				ref.cond.L.Unlock()

				return
			}
			ref.err = ErrTimeout
			ref.cond.L.Unlock()
			ref.Stop(pid)
		})
		atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&ref.t)), unsafe.Pointer(tp))
	}

	return &ref.Future
}

type Future struct {
	actorSystem *ActorSystem
	pid         *PID
	cond        *sync.Cond
	// protected by cond
	done        bool
	result      interface{}
	err         error
	t           *time.Timer
	pipes       []*PID
	completions []func(res interface{}, err error)
}

// PID to the backing actor for the Future result.
func (f *Future) PID() *PID {
	return f.pid
}

// PipeTo forwards the result or error of the future to the specified pids.
func (f *Future) PipeTo(pids ...*PID) {
	f.cond.L.Lock()
	f.pipes = append(f.pipes, pids...)
	// for an already completed future, force push the result to targets.
	if f.done {
		f.sendToPipes()
	}
	f.cond.L.Unlock()
}

func (f *Future) sendToPipes() {
	if f.pipes == nil {
		return
	}

	var m interface{}
	if f.err != nil {
		m = f.err
	} else {
		m = f.result
	}

	for _, pid := range f.pipes {
		pid.sendUserMessage(f.actorSystem, m)
	}

	f.pipes = nil
}

func (f *Future) wait() {
	f.cond.L.Lock()
	for !f.done {
		f.cond.Wait()
	}
	f.cond.L.Unlock()
}

// Result waits for the future to resolve.
func (f *Future) Result() (interface{}, error) {
	f.wait()

	return f.result, f.err
}

func (f *Future) Wait() error {
	f.wait()

	return f.err
}

func (f *Future) continueWith(continuation func(res interface{}, err error)) {
	f.cond.L.Lock()
	defer f.cond.L.Unlock() // use defer as the continuation co
	// uld blow up
	if f.done {
		continuation(f.result, f.err)
	} else {
		f.completions = append(f.completions, continuation)
	}
}

// futureProcess is a struct carrying a response PID and a channel where the response is placed.
type futureProcess struct {
	Future
}

var _ Process = &futureProcess{}

func (ref *futureProcess) SendUserMessage(pid *PID, message interface{}) {
	defer ref.instrument()

	_, msg, _ := UnwrapEnvelope(message)

	if _, ok := msg.(*DeadLetterResponse); ok {
		ref.result = nil
		ref.err = ErrDeadLetter
	} else {
		ref.result = msg
	}

	ref.Stop(pid)
}

func (ref *futureProcess) SendSystemMessage(pid *PID, message interface{}) {
	defer ref.instrument()
	ref.result = message
	ref.Stop(pid)
}

func (ref *futureProcess) instrument() {
	sysMetrics, ok := ref.actorSystem.Extensions.Get(extensionId).(*Metrics)
	if ok && sysMetrics.enabled {
		ctx := context.Background()
		labels := []attribute.KeyValue{
			attribute.String("address", ref.actorSystem.Address()),
		}

		instruments := sysMetrics.metrics.Get(metrics.InternalActorMetrics)
		if instruments != nil {
			if ref.err == nil {
				instruments.FuturesCompletedCount.Add(ctx, 1, metric.WithAttributes(labels...))
			} else {
				instruments.FuturesTimedOutCount.Add(ctx, 1, metric.WithAttributes(labels...))
			}
		}
	}
}

func (ref *futureProcess) Stop(pid *PID) {
	ref.cond.L.Lock()
	if ref.done {
		ref.cond.L.Unlock()

		return
	}

	ref.done = true
	tp := (*time.Timer)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&ref.t))))

	if tp != nil {
		tp.Stop()
	}

	ref.actorSystem.ProcessRegistry.Remove(pid)

	ref.sendToPipes()
	ref.runCompletions()
	ref.cond.L.Unlock()
	ref.cond.Signal()
}

// TODO: we could replace "pipes" with this
// instead of pushing PIDs to pipes, we could push wrapper funcs that tells the pid
// as a completion, that would unify the model.
func (f *Future) runCompletions() {
	if f.completions == nil {
		return
	}

	for _, c := range f.completions {
		c(f.result, f.err)
	}

	f.completions = nil
}
