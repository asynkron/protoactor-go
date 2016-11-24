package actor

//ActorRef is an interface that defines the base contract for interaction of actors
type ActorRef interface {
	Tell(pid *PID, message interface{})
	Ask(pid *PID, message interface{}, sender *PID)
	SendSystemMessage(pid *PID, message SystemMessage)
	Stop(pid *PID)
	Watch(pid *PID)
	UnWatch(pid *PID)
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
	ref.SendSystemMessage(pid, &Stop{})
}

func (ref *LocalActorRef) Watch(pid *PID) {
	ref.SendSystemMessage(pid, &Watch{Watcher: pid})
}

func (ref *LocalActorRef) UnWatch(pid *PID) {
	ref.SendSystemMessage(pid, &Unwatch{Watcher: pid})
}

func (ref *LocalActorRef) Suspend() {
	ref.mailbox.Suspend()
}

func (ref *LocalActorRef) Resume() {
	ref.mailbox.Resume()
}
