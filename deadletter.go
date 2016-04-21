package gam

type DeadLetterActorRef struct {
}

var deadLetter ActorRef = new(DeadLetterActorRef)

func (DeadLetterActorRef) Tell(message interface{}) {

}

func (DeadLetterActorRef) SendSystemMessage(message SystemMessage) {
}

func (DeadLetterActorRef) Stop() {
}
