package actor

type localActorRef struct {
	mailbox Mailbox
}

func newLocalActorRef(mailbox Mailbox) *localActorRef {
	return &localActorRef{
		mailbox: mailbox,
	}
}

func (ref *localActorRef) SendUserMessage(pid *PID, message interface{}, sender *PID) {
	if sender != nil {
		ref.mailbox.PostUserMessage(&Request{Message: message, Sender: sender})
	} else {
		ref.mailbox.PostUserMessage(message)
	}
}

func (ref *localActorRef) SendSystemMessage(pid *PID, message SystemMessage) {
	ref.mailbox.PostSystemMessage(message)
}

func (ref *localActorRef) Stop(pid *PID) {
	ref.SendSystemMessage(pid, stopMessage)
}

func (ref *localActorRef) Watch(pid *PID) {
	ref.SendSystemMessage(pid, &Watch{Watcher: pid})
}

func (ref *localActorRef) Unwatch(pid *PID) {
	ref.SendSystemMessage(pid, &Unwatch{Watcher: pid})
}
