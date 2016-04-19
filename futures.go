package actor

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

func (ref *FutureActorRef) SendSystemMessage(message SystemMessage) {
}

func (ref *FutureActorRef) Stop() {
	close(ref.channel)
}
