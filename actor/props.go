package actor

type Properties interface {
	ProduceActor() Actor
	ProduceMailbox() Mailbox
	Supervisor() SupervisionStrategy
}

type PropsValue struct {
	actorProducer       ActorProducer
	mailboxProducer     MailboxProducer
	supervisionStrategy SupervisionStrategy
}

func (props PropsValue) ProduceActor() Actor {
	return props.actorProducer()
}

func (props PropsValue) Supervisor() SupervisionStrategy {
	if props.supervisionStrategy == nil {
		return DefaultSupervisionStrategy()
	}
	return props.supervisionStrategy
}

func (props PropsValue) ProduceMailbox() Mailbox {
	if props.mailboxProducer == nil {
		return NewUnboundedMailbox(100)()
	}
	return props.mailboxProducer()
}

func (props PropsValue) WithMailbox(mailbox MailboxProducer) PropsValue {
	//pass by value, we only modify the copy
	props.mailboxProducer = mailbox
	return props
}

func (props PropsValue) WithSupervisor(supervisor SupervisionStrategy) PropsValue {
	//pass by value, we only modify the copy
	props.supervisionStrategy = supervisor
	return props
}

func FromProducer(actorProducer ActorProducer) PropsValue {
	return PropsValue{
		actorProducer:   actorProducer,
		mailboxProducer: nil,
	}
}

func FromFunc(receive Receive) PropsValue {
	a := &EmptyActor{
		receive: receive,
	}
	p := FromProducer(func() Actor {
		return a
	})
	return p
}

func FromInstance(template Actor) PropsValue {
	producer := func() Actor {
		return template
	}
	p := FromProducer(producer)
	return p
}
