package actor

//Props or properties of an actor, it defines how the actor should be created
type Props struct {
	actorProducer       Producer
	mailboxProducer     MailboxProducer
	supervisionStrategy SupervisorStrategy
	receivePlugins      []Receive
	dispatcher          Dispatcher
	spawner             Spawner
}

func (props Props) Dispatcher() Dispatcher {
	if props.dispatcher == nil {
		return defaultDispatcher
	}
	return props.dispatcher
}

func (props Props) ProduceActor() Actor {
	return props.actorProducer()
}

func (props Props) Supervisor() SupervisorStrategy {
	if props.supervisionStrategy == nil {
		return defaultSupervisionStrategy
	}
	return props.supervisionStrategy
}

func (props Props) ProduceMailbox() Mailbox {
	if props.mailboxProducer == nil {
		return defaultMailboxProducer()
	}
	return props.mailboxProducer()
}

func (props Props) spawn(id string, parent *PID) *PID {
	if props.spawner != nil {
		return props.spawner(id, props, parent)
	}
	return DefaultSpawner(id, props, parent)
}

func (props Props) WithReceivers(plugin ...Receive) Props {
	//pass by value, we only modify the copy
	props.receivePlugins = append(props.receivePlugins, plugin...)
	return props
}

func (props Props) WithMailbox(mailbox MailboxProducer) Props {
	//pass by value, we only modify the copy
	props.mailboxProducer = mailbox
	return props
}

func (props Props) WithSupervisor(supervisor SupervisorStrategy) Props {
	//pass by value, we only modify the copy
	props.supervisionStrategy = supervisor
	return props
}

func (props Props) WithDispatcher(dispatcher Dispatcher) Props {
	//pass by value, we only modify the copy
	props.dispatcher = dispatcher
	return props
}

func (props Props) WithSpawn(spawn Spawner) Props {
	props.spawner = spawn
	return props
}

func FromProducer(actorProducer Producer) Props {
	return Props{actorProducer: actorProducer}
}

func FromFunc(receive Receive) Props {
	return FromInstance(receive)
}

func FromInstance(template Actor) Props {
	producer := func() Actor {
		return template
	}
	p := FromProducer(producer)
	return p
}
