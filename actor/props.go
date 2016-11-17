package actor

//Props or properties of an actor, it defines how the actor should be created
type Props struct {
	actorProducer       ActorProducer
	mailboxProducer     MailboxProducer
	supervisionStrategy SupervisionStrategy
	routerConfig        RouterConfig
	receivePluins       []Receive
}

func (props Props) RouterConfig() RouterConfig {
	return props.routerConfig
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

func (props Props) WithReceivers(plugin ...Receive) Props {
	//pass by value, we only modify the copy
	props.receivePluins = append(props.receivePluins, plugin...)
	return props
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

func (props Props) WithPoolRouter(routerConfig PoolRouterConfig) Props {
	if props.routerConfig != nil {
		panic("The props already have a router")
	}
	//pass by value, we only modify the copy
	props.routerConfig = routerConfig
	return props
}

func FromProducer(actorProducer ActorProducer) Props {
	return Props{
		actorProducer:   actorProducer,
		mailboxProducer: nil,
		routerConfig:    nil,
	}
}

func FromFunc(receive Receive) Props {
	a := &emptyActor{
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

func FromGroupRouter(router GroupRouterConfig) Props {
	return Props{
		routerConfig:  router,
		actorProducer: nil,
	}
}
