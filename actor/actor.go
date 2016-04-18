package actor

type Actor interface {
	Receive(message *Context)
}

func ActorOf(props PropsValue) ActorRef {
	cell := NewActorCell(props)
	mailbox := NewDefaultMailbox(cell)
	ref := ChannelActorRef{
		mailbox: mailbox,
	}
	cell.Self = &ref

	return &ref
}