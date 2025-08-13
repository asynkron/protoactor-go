package actor

import (
	"sync/atomic"
)

// ActorProcess is the Process implementation for regular actors.
//
//revive:disable-next-line:exported
type ActorProcess struct {
	mailbox Mailbox
	dead    int32
}

var _ Process = &ActorProcess{}

// NewActorProcess creates a new ActorProcess with the given mailbox.
func NewActorProcess(mailbox Mailbox) *ActorProcess {
	return &ActorProcess{
		mailbox: mailbox,
	}
}

// SendUserMessage posts a user message to the actor's mailbox.
func (ref *ActorProcess) SendUserMessage(_ *PID, message interface{}) {
	ref.mailbox.PostUserMessage(message)
}

// SendSystemMessage posts a system message to the actor's mailbox.
func (ref *ActorProcess) SendSystemMessage(_ *PID, message interface{}) {
	ref.mailbox.PostSystemMessage(message)
}

// Stop terminates the actor process.
func (ref *ActorProcess) Stop(pid *PID) {
	atomic.StoreInt32(&ref.dead, 1)
	ref.SendSystemMessage(pid, stopMessage)
}
