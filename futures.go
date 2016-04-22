package gam

import "time"
import "fmt"

func FuturePID() *PID {
	ref := &FutureActorRef{
		channel: make(chan interface{}),
	}
	pid := registerPID(ref)
	return pid
}

type FutureActorRef struct {
	channel chan interface{}
}

func (ref *FutureActorRef) Tell(message interface{}) {
	ref.channel <- message
}

func (ref *FutureActorRef) ResultChannel() <-chan interface{} {
	return ref.channel
}

func (ref *FutureActorRef) ResultOrTimeout(timeout time.Duration) (interface{}, error) {
	select {
	case res := <-ref.channel:
		return res, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("Timeout")
	}
}

func (ref *FutureActorRef) Result() interface{} {
	return <-ref.channel
}

func (ref *FutureActorRef) SendSystemMessage(message SystemMessage) {
}

func (ref *FutureActorRef) Stop() {
	close(ref.channel)
}
