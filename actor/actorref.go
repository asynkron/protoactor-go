package actor

import "log"

//ActorRef is an interface that defines the base contract for interaction of actors
type ActorRef interface {
	Tell(pid *PID, message interface{})
	Ask(pid *PID, message interface{}, sender *PID)
	SendSystemMessage(pid *PID, message SystemMessage)
	Stop(pid *PID)
}

type LocalActorRef struct {
	mailbox Mailbox
}

func NewLocalActorRef(mailbox Mailbox) *LocalActorRef {
	return &LocalActorRef{
		mailbox: mailbox,
	}
}

func (ref *LocalActorRef) Tell(pid *PID, message interface{}) {
	ref.mailbox.PostUserMessage(UserMessage{Message: message})
}

func (ref *LocalActorRef) Ask(pid *PID, message interface{}, sender *PID) {
	ref.mailbox.PostUserMessage(UserMessage{Message: message, Sender: sender})
}

func (ref *LocalActorRef) SendSystemMessage(pid *PID, message SystemMessage) {
	ref.mailbox.PostSystemMessage(message)
}

func (ref *LocalActorRef) Stop(pid *PID) {
	ref.SendSystemMessage(pid, &stop{})
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

func (DeadLetterActorRef) Tell(pid *PID, message interface{}) {
	log.Printf("Deadletter for %v got %+v", pid, message)
}

func (DeadLetterActorRef) Ask(pid *PID, message interface{}, sender *PID) {
	log.Printf("Deadletter was %v asked %+v", pid, message)
}

func (DeadLetterActorRef) SendSystemMessage(pid *PID, message SystemMessage) {
}

func (DeadLetterActorRef) Stop(pid *PID) {
}
