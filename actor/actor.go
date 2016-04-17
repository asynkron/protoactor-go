package actor

type Actor interface {
	Receive(message *Context)
}

type Context struct {
	*ActorCell
    Message interface{}	
}

func ActorOf(actor Actor) ActorRef {
	userMailbox := make(chan interface{}, 100)
	systemMailbox := make(chan interface{}, 100)
	cell := NewActorCell(actor)
	mailbox := Mailbox{
		userMailbox:     userMailbox,
		systemMailbox:   systemMailbox,
		hasMoreMessages: MailboxHasNoMessages,
		schedulerStatus: MailboxIdle,
		actorCell:       cell,
	}

	ref := ChannelActorRef{
		mailbox: &mailbox,
	}
	cell.Self = &ref

	return &ref
}
