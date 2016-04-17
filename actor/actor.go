package actor

type Actor interface {
	Receive(message interface{})
}

func ActorOf(actor Actor) ActorRef {
	userMailbox := make(chan interface{}, 100)
	systemMailbox := make(chan interface{}, 100)
	cell := &ActorCell{
		actor: actor,
	}
	mailbox := Mailbox{
		userMailbox:     userMailbox,
		systemMailbox:   systemMailbox,
		hasMoreMessages: int32(0),
		schedulerStatus: int32(0),
		actorCell:       cell,
	}

	ref := ChannelActorRef{
		mailbox: &mailbox,
	}

	return &ref
}
