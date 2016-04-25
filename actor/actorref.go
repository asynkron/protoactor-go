package actor

type ActorRef interface {
	Tell(message interface{})
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
	ref.mailbox.PostUserMessage(message)
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

}

func (DeadLetterActorRef) SendSystemMessage(message SystemMessage) {
}

func (DeadLetterActorRef) Stop() {
}
