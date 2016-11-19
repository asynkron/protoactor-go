package actor

import (
	"fmt"
	"time"
)

func NewFuture(timeout time.Duration) *Future {
	ref := &FutureActorRef{
		channel: make(chan interface{}, 1),
	}
	id := ProcessRegistry.getAutoId()
	pid, _ := ProcessRegistry.registerPID(ref, id)
	ref.pid = pid

	fut := &Future{
		ref:     ref,
		timeout: timeout,
	}
	return fut
}

type Future struct {
	ref     *FutureActorRef
	timeout time.Duration
}

//PID to the backing actor for the Future result
func (fut *Future) PID() *PID {
	return fut.ref.pid
}

//PipeTo starts a go routine and waits for the `Future.Result()`, then sends the result to the given `PID`
func (ref *Future) PipeTo(pid *PID) {
	go func() {
		res, _ := ref.Result()
		pid.Tell(res)
	}()
}

func (fut *Future) ResultChannel() <-chan interface{} {
	return fut.ref.channel
}

func (fut *Future) Result() (interface{}, error) {
	select {
	case res := <-fut.ref.channel:
		return res, nil
	case <-time.After(fut.timeout):
		return nil, fmt.Errorf("Timeout")
	}
}

func (fut *Future) Stop() {
	fut.ref.Stop(fut.PID())
}

func (fut *Future) Wait() {
	<-fut.ref.channel
}

//Future is a struct carrying a response PID and a channel where the response is placed
type FutureActorRef struct {
	channel chan interface{}
	pid     *PID
}

func (ref *FutureActorRef) Tell(pid *PID, message interface{}) {
	ref.channel <- message
}

func (ref *FutureActorRef) Ask(pid *PID, message interface{}, sender *PID) {
	ref.channel <- message
}

func (ref *FutureActorRef) SendSystemMessage(pid *PID, message SystemMessage) {
}

func (ref *FutureActorRef) Stop(pid *PID) {
	ProcessRegistry.unregisterPID(ref.pid)
	close(ref.channel)
}
