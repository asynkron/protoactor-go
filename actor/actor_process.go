package actor

import (
	"sync/atomic"
)

type ActorProcess struct {
	mailbox Mailbox
	dead    int32
}

var _ Process = &ActorProcess{}

func NewActorProcess(mailbox Mailbox) *ActorProcess {
	return &ActorProcess{
		mailbox: mailbox,
	}
}

func (ref *ActorProcess) SendUserMessage(_ *PID, message interface{}) {
	ref.mailbox.PostUserMessage(message)
}

func (ref *ActorProcess) SendSystemMessage(_ *PID, message interface{}) {
	ref.mailbox.PostSystemMessage(message)
}

func (ref *ActorProcess) Stop(pid *PID) {
	atomic.StoreInt32(&ref.dead, 1)
	ref.SendSystemMessage(pid, stopMessage)
}
