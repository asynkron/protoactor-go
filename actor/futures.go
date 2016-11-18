package actor

import (
	"fmt"
	"time"
)

func NewFuture() *Future {
	ref := &FutureActorRef{
		channel: make(chan interface{}, 1),
	}
	id := ProcessRegistry.getAutoId()
	pid, _ := ProcessRegistry.registerPID(ref, id)
	ref.pid = pid

	fut := &Future{
		ref: ref,
	}
	return fut
}

type Future struct {
	ref *FutureActorRef
}

//PID to the backing actor for the Future result
func (fut *Future) PID() *PID {
	return fut.ref.pid
}

//PipeTo starts a go routine and waits for the `Future.Result()`, then sends the result to the given `PID`
func (ref *Future) PipeTo(pid *PID) {
	go func() {
		res := ref.Result()
		pid.Tell(res)
	}()
}

func (fut *Future) ResultChannel() <-chan interface{} {
	return fut.ref.channel
}

func (fut *Future) ResultOrTimeout(timeout time.Duration) (interface{}, error) {
	select {
	case res := <-fut.ref.channel:
		return res, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("Timeout")
	}
}

func (fut *Future) Stop() {
	fut.ref.Stop(fut.PID())
}

func (fut *Future) Wait() {
	<-fut.ref.channel
}

func (fut *Future) Result() interface{} {
	return <-fut.ref.channel
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
