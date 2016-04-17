package actor

type ActorRef interface {
	Tell(message interface{})
	SendSystemMessage(message interface{})
}

type ChannelActorRef struct {
	mailbox *Mailbox
}

func (ref *ChannelActorRef) Tell(message interface{}) {
	ref.mailbox.userMailbox <- message
	ref.mailbox.schedule()
}

func (ref *ChannelActorRef) SendSystemMessage(message interface{}) {
	ref.mailbox.userMailbox <- message
	ref.mailbox.schedule()
}