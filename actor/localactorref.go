package actor

type LocalActorRef struct {
	mailbox Mailbox
}

func (ref *LocalActorRef) Tell(message interface{}) {
	ref.mailbox.PostUserMessage(message)
}

func (ref *LocalActorRef) SendSystemMessage(message SystemMessage) {
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
