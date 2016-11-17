package actor

import (
	"fmt"
	"time"
)

func NewFuture() *Future {
	ref := &Future{
		channel: make(chan interface{}, 1),
	}
	id := ProcessRegistry.getAutoId()
	pid, _ := ProcessRegistry.registerPID(ref, id)
	ref.pid = pid
	return ref
}

type Future struct {
	channel chan interface{}
	pid     *PID
}

//PipeTo starts a go routine and waits for the `Future.Result()`, then sends the result to the given `PID`
func (ref *Future) PipeTo(pid *PID) {
	go func() {
		res := ref.Result()
		pid.Tell(res)
	}()
}

//PID to the backing actor for the Future result
func (ref *Future) PID() *PID {
	return ref.pid
}

func (ref *Future) Tell(message interface{}) {
	ref.channel <- message
}

func (ref *Future) Ask(message interface{}, sender *PID) {
	ref.channel <- message
}

func (ref *Future) ResultChannel() <-chan interface{} {
	return ref.channel
}

func (ref *Future) ResultOrTimeout(timeout time.Duration) (interface{}, error) {
	select {
	case res := <-ref.channel:
		return res, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("Timeout")
	}
}

func (ref *Future) Result() interface{} {
	return <-ref.channel
}

func (ref *Future) SendSystemMessage(message SystemMessage) {
}

func (ref *Future) Stop() {
	ProcessRegistry.unregisterPID(ref.pid)
	close(ref.channel)
}
