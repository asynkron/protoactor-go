package actor

func Spawn(props PropsValue) ActorRef {
	return SpawnChild(props, nil)
}

func SpawnChild(props PropsValue,parent ActorRef) ActorRef {
	cell := NewActorCell(props,parent)
	mailbox := props.mailboxProducer(cell.invokeUserMessage, cell.invokeSystemMessage)
	ref := ChannelActorRef{
		mailbox: mailbox,
	}
	cell.self = &ref
	ref.Tell(Starting{})
	return &ref
}