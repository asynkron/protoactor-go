package actor

type Properties interface {
	ProduceActor() Actor
	ProduceMailbox() Mailbox
	Supervisor() SupervisionStrategy
}

type Props struct {
	actorProducer       ActorProducer
	mailboxProducer     MailboxProducer
	supervisionStrategy SupervisionStrategy
}

func (props Props) ProduceActor() Actor {
	return props.actorProducer()
}

func (props Props) Supervisor() SupervisionStrategy {
	if props.supervisionStrategy == nil {
		return DefaultSupervisionStrategy()
	}
	return props.supervisionStrategy
}

func (props Props) ProduceMailbox() Mailbox {
	if props.mailboxProducer == nil {
		return NewUnboundedMailbox(100)()
	}
	return props.mailboxProducer()
}

func (props Props) WithMailbox(mailbox MailboxProducer) Props {
	//pass by value, we only modify the copy
	props.mailboxProducer = mailbox
	return props
}

func (props Props) WithSupervisor(supervisor SupervisionStrategy) Props {
	//pass by value, we only modify the copy
	props.supervisionStrategy = supervisor
	return props
}

func FromProducer(actorProducer ActorProducer) Props {
	return Props{
		actorProducer:   actorProducer,
		mailboxProducer: nil,
	}
}

func FromFunc(receive Receive) Props {
	a := &EmptyActor{
		receive: receive,
	}
	p := FromProducer(func() Actor {
		return a
	})
	return p
}

func FromInstance(template Actor) Props {
	producer := func() Actor {
		return template
	}
	p := FromProducer(producer)
	return p
}
