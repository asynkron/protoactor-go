package actor

import "github.com/rogeralsing/goactor/interfaces"

type LocalActorRef struct {
	mailbox interfaces.Mailbox
}

func (ref *LocalActorRef) Tell(message interface{}) {
	ref.mailbox.PostUserMessage(message)
}

func (ref *LocalActorRef) SendSystemMessage(message interfaces.SystemMessage) {
	ref.mailbox.PostSystemMessage(message)
}

func (ref *LocalActorRef) Stop() {
	ref.SendSystemMessage(&Stop{})
}

func (ref *LocalActorRef) Suspend() {
	ref.mailbox.Suspend()
}

func (ref *LocalActorRef) Resume() {
	ref.mailbox.Resume()
}
