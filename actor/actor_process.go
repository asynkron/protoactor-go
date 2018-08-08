package actor

import (
	"sync/atomic"

	"github.com/AsynkronIT/protoactor-go/mailbox"
)

type actorProcess struct {
	mailbox mailbox.Mailbox
	dead    int32
}

func newActorProcess(mailbox mailbox.Mailbox) *actorProcess {
	return &actorProcess{mailbox: mailbox}
}

func (ref *actorProcess) SendUserMessage(pid *PID, message interface{}) {
	ref.mailbox.PostUserMessage(message)
}
func (ref *actorProcess) SendSystemMessage(pid *PID, message interface{}) {
	ref.mailbox.PostSystemMessage(message)
}

func (ref *actorProcess) Stop(pid *PID) {
	atomic.StoreInt32(&ref.dead, 1)
	ref.SendSystemMessage(pid, stopMessage)
}
