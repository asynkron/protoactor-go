package actor

import "github.com/AsynkronIT/protoactor-go/mailbox"

// Props represents configuration to define how an actor should be created
type Props struct {
	actorProducer       Producer
	mailboxProducer     mailbox.Producer
	supervisionStrategy SupervisorStrategy
	middleware          []func(next ReceiveFunc) ReceiveFunc
	middlewareChain     ReceiveFunc
	dispatcher          mailbox.Dispatcher
	spawner             Spawner
}

func (props Props) Dispatcher() mailbox.Dispatcher {
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

func (props Props) ProduceMailbox(invoker mailbox.MessageInvoker, dispatcher mailbox.Dispatcher) mailbox.Inbound {
	if props.mailboxProducer == nil {
		return defaultMailboxProducer(invoker, dispatcher)
	}
	return props.mailboxProducer(invoker, dispatcher)
}

func (props Props) spawn(id string, parent *PID) (*PID, error) {
	if props.spawner != nil {
		return props.spawner(id, props, parent)
	}
	return DefaultSpawner(id, props, parent)
}

func (props Props) WithMiddleware(middleware ...func(ReceiveFunc) ReceiveFunc) Props {
	props.middleware = append(props.middleware, middleware...)
	props.middlewareChain = makeMiddlewareChain(props.middleware, localContextReceiver)
	return props
}

func (props Props) WithMailbox(mailbox mailbox.Producer) Props {
	//pass by value, we only modify the copy
	props.mailboxProducer = mailbox
	return props
}

func (props Props) WithSupervisor(supervisor SupervisorStrategy) Props {
	//pass by value, we only modify the copy
	props.supervisionStrategy = supervisor
	return props
}

func (props Props) WithDispatcher(dispatcher mailbox.Dispatcher) Props {
	//pass by value, we only modify the copy
	props.dispatcher = dispatcher
	return props
}

func (props Props) WithSpawn(spawn Spawner) Props {
	props.spawner = spawn
	return props
}

func (props Props) WithFunc(receive ReceiveFunc) Props {
	props.actorProducer = makeProducerFromInstance(receive)
	return props
}

func (props Props) WithInstance(a Actor) Props {
	props.actorProducer = makeProducerFromInstance(a)
	return props
}

func (props Props) WithProducer(p Producer) Props {
	props.actorProducer = p
	return props
}

// FromProducer
func FromProducer(actorProducer Producer) Props {
	return Props{actorProducer: actorProducer}
}

func FromFunc(receive ReceiveFunc) Props {
	return FromInstance(receive)
}

func FromSpawn(spawn Spawner) Props {
	return Props{spawner: spawn}
}

func FromInstance(template Actor) Props {
	return Props{actorProducer: makeProducerFromInstance(template)}
}

func makeProducerFromInstance(a Actor) Producer {
	return func() Actor {
		return a
	}
}
