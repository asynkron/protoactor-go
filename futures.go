package actor

import "time"
import "fmt"

func NewFutureActorRef() *FutureActorRef {
	ref := FutureActorRef{
		channel: make(chan interface{}),
	}
	return &ref
}

type FutureActorRef struct {
	channel chan interface{}
}

func (ref *FutureActorRef) Tell(message interface{}) {
	ref.channel <- message
}

func (ref *FutureActorRef) Result() <-chan interface{} {
	return ref.channel
}

func (ref *FutureActorRef) WaitResultTimeout(timeout time.Duration) (interface{}, error) {
	select {
	case res:= <-ref.channel:
		return res, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("Timeout")
	}
}

func (ref *FutureActorRef) WaitResult() interface{} {
	return <-ref.channel
}

func (ref *FutureActorRef) SendSystemMessage(message SystemMessage) {
}

func (ref *FutureActorRef) Stop() {
	close(ref.channel)
}
