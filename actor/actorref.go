package actor

type ActorRef interface {
	Tell(message interface{})
	SendSystemMessage(message interface{})
	Stop()
}

type ChannelActorRef struct {
	mailbox Mailbox
}

func (ref *ChannelActorRef) Tell(message interface{}) {
	ref.mailbox.PostUserMessage(message)
}

func (ref *ChannelActorRef) SendSystemMessage(message interface{}) {
	ref.mailbox.PostSystemMessage(message)
}

func (ref *ChannelActorRef) Stop(){
	ref.SendSystemMessage(Stop{})
}