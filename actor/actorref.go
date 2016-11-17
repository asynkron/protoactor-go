package actor

import "log"

//ActorRef is an interface that defines the base contract for interaction of actors
type ActorRef interface {
	Tell(message interface{})
	Ask(message interface{}, sender *PID)
	SendSystemMessage(message SystemMessage)
	Stop()
}

type LocalActorRef struct {
	mailbox Mailbox
}

func NewLocalActorRef(mailbox Mailbox) *LocalActorRef {
	return &LocalActorRef{
		mailbox: mailbox,
	}
}

func (ref *LocalActorRef) Tell(message interface{}) {
	ref.mailbox.PostUserMessage(UserMessage{Message: message})
}

func (ref *LocalActorRef) Ask(message interface{}, sender *PID) {
	ref.mailbox.PostUserMessage(UserMessage{Message: message, Sender: sender})
}

func (ref *LocalActorRef) SendSystemMessage(message SystemMessage) {
	ref.mailbox.PostSystemMessage(message)
}

func (ref *LocalActorRef) Stop() {
	ref.SendSystemMessage(&stop{})
}

func (ref *LocalActorRef) Suspend() {
	ref.mailbox.Suspend()
}

func (ref *LocalActorRef) Resume() {
	ref.mailbox.Resume()
}

type DeadLetterActorRef struct {
}

var deadLetter ActorRef = new(DeadLetterActorRef)

func (DeadLetterActorRef) Tell(message interface{}) {
	log.Printf("Deadletter got %+v", message)
}

func (DeadLetterActorRef) Ask(message interface{}, sender *PID) {
	log.Printf("Deadletter was asked %+v", message)
}

func (DeadLetterActorRef) SendSystemMessage(message SystemMessage) {
}

func (DeadLetterActorRef) Stop() {
}
