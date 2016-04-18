package actor

func Spawn(props PropsValue) ActorRef {
	cell := NewActorCell(props)
	mailbox := NewQueueMailbox(cell)
	ref := ChannelActorRef{
		mailbox: mailbox,
	}
	cell.Self = &ref
	ref.Tell(Starting{})
	return &ref
}