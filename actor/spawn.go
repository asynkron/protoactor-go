package actor

func Spawn(props PropsValue) ActorRef {
	cell := NewActorCell(props)
	mailbox := props.mailboxProducer(cell.invokeUserMessage,cell.invokeSystemMessage)
	ref := ChannelActorRef{
		mailbox: mailbox,
	}
	cell.Self = &ref
	ref.Tell(Starting{})
	return &ref
}