package actor

import (
	"errors"
	"log"
	"sync"
	"time"
)

var (
	ErrTimeout = errors.New("timeout")
)

func NewFuture(timeout time.Duration) *Future {
	fut := &Future{cond: sync.NewCond(&sync.Mutex{})}

	ref := &FutureActorRef{f: fut}
	id := ProcessRegistry.getAutoId()

	pid, ok := ProcessRegistry.add(ref, id)
	if !ok {
		log.Printf("[ACTOR] Failed to register future actorref '%v'", id)
		log.Println(id)
	}

	fut.pid = pid
	fut.t = time.AfterFunc(timeout, func() {
		fut.err = ErrTimeout
		ref.Stop(pid)
	})

	return fut
}

type Future struct {
	pid    *PID
	cond   *sync.Cond
	// protected by cond
	done   bool
	result interface{}
	err    error
	t      *time.Timer
}

// PID to the backing actor for the Future result
func (f *Future) PID() *PID {
	return f.pid
}

// PipeTo starts a go routine and waits for the `Future.Result()`, then sends the result to the given `PID`
func (f *Future) PipeTo(pid *PID) {
	go func() {
		res, err := f.Result()
		if err != nil {
			pid.Tell(err)
		} else {
			pid.Tell(res)
		}
	}()
}

func (f *Future) wait() {
	f.cond.L.Lock()
	if !f.done {
		f.cond.Wait()
	}
	f.cond.L.Unlock()
}

func (f *Future) Result() (interface{}, error) {
	f.wait()
	return f.result, f.err
}

func (f *Future) Wait() error {
	f.wait()
	return f.err
}

// FutureActorRef is a struct carrying a response PID and a channel where the response is placed
type FutureActorRef struct {
	f *Future
}

func (ref *FutureActorRef) SendUserMessage(pid *PID, message interface{}, sender *PID) {
	ref.f.result = message
	ref.Stop(pid)
}

func (ref *FutureActorRef) SendSystemMessage(pid *PID, message SystemMessage) {
	ref.f.result = message
	ref.Stop(pid)
}

func (ref *FutureActorRef) Stop(pid *PID) {
	ref.f.cond.L.Lock()
	if ref.f.done {
		ref.f.cond.L.Unlock()
		return
	}

	ref.f.done = true
	ref.f.t.Stop()
	ProcessRegistry.remove(pid)

	ref.f.cond.L.Unlock()
	ref.f.cond.Signal()
}

func (ref *FutureActorRef) Watch(pid *PID)   {}
func (ref *FutureActorRef) UnWatch(pid *PID) {}
