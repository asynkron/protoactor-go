package actor

import (
	"errors"
	"log"
	"sync"
	"time"
)

// ErrTimeout is the error used when a future times out before receiving a result.
var ErrTimeout = errors.New("future: timeout")

// NewFuture creates and returns a new actor.Future with a timeout of duration d
func NewFuture(d time.Duration) *Future {
	ref := &futureProcess{Future{cond: sync.NewCond(&sync.Mutex{})}}
	id := ProcessRegistry.NextId()

	pid, ok := ProcessRegistry.Add(ref, id)
	if !ok {
		log.Printf("[ACTOR] Failed to register future actorref '%v'", id)
		log.Println(id)
	}

	ref.pid = pid
	ref.t = time.AfterFunc(d, func() {
		ref.err = ErrTimeout
		ref.Stop(pid)
	})

	return &ref.Future
}

type Future struct {
	pid  *PID
	cond *sync.Cond
	// protected by cond
	done   bool
	result interface{}
	err    error
	t      *time.Timer
	pipes  []*PID
}

// PID to the backing actor for the Future result
func (f *Future) PID() *PID {
	return f.pid
}

// PipeTo forwards the result or error of the future to the specified pids
func (f *Future) PipeTo(pids ...*PID) {
	f.pipes = append(f.pipes, pids...)
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
		pid.Tell(m)
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

// Result waits for the future to resolve
func (f *Future) Result() (interface{}, error) {
	f.wait()
	return f.result, f.err
}

func (f *Future) Wait() error {
	f.wait()
	return f.err
}

// futureProcess is a struct carrying a response PID and a channel where the response is placed
type futureProcess struct {
	Future
}

func (ref *futureProcess) SendUserMessage(pid *PID, message interface{}, sender *PID) {
	ref.result = message
	ref.Stop(pid)
}

func (ref *futureProcess) SendSystemMessage(pid *PID, message interface{}) {
	ref.result = message
	ref.Stop(pid)
}

func (ref *futureProcess) Stop(pid *PID) {
	ref.cond.L.Lock()
	if ref.done {
		ref.cond.L.Unlock()
		return
	}

	ref.done = true
	ref.t.Stop()
	ProcessRegistry.Remove(pid)

	ref.sendToPipes()
	ref.cond.L.Unlock()
	ref.cond.Signal()
}
