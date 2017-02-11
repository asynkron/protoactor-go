package actor

import "github.com/AsynkronIT/protoactor-go/mailbox"

// Props represents configuration to define how an actor should be created
type Props struct {
	actorProducer       Producer
	mailboxProducer     mailbox.Producer
	supervisionStrategy SupervisorStrategy
	middleware          []func(next ActorFunc) ActorFunc
	middlewareChain     ActorFunc
	dispatcher          mailbox.Dispatcher
	spawner             SpawnFunc
}

func (props *Props) getDispatcher() mailbox.Dispatcher {
	if props.dispatcher == nil {
		return defaultDispatcher
	}
	return props.dispatcher
}

func (props *Props) getSupervisor() SupervisorStrategy {
	if props.supervisionStrategy == nil {
		return defaultSupervisionStrategy
	}
	return props.supervisionStrategy
}

func (props *Props) produceMailbox(invoker mailbox.MessageInvoker, dispatcher mailbox.Dispatcher) mailbox.Inbound {
	if props.mailboxProducer == nil {
		return defaultMailboxProducer(invoker, dispatcher)
	}
	return props.mailboxProducer(invoker, dispatcher)
}

func (props *Props) spawn(id string, parent *PID) (*PID, error) {
	if props.spawner != nil {
		return props.spawner(id, props, parent)
	}
	return DefaultSpawner(id, props, parent)
}

//WithMiddleware assigns one or more middlewares to the props
func (props *Props) WithMiddleware(middleware ...func(ActorFunc) ActorFunc) *Props {
	props.middleware = append(props.middleware, middleware...)
	props.middlewareChain = makeMiddlewareChain(props.middleware, localContextReceiver)
	return props
}

//WithMailbox assigns the desired mailbox producer to the props
func (props *Props) WithMailbox(mailbox mailbox.Producer) *Props {
	props.mailboxProducer = mailbox
	return props
}

//WithSupervisor assigns a supervision strategy to the props
func (props *Props) WithSupervisor(supervisor SupervisorStrategy) *Props {
	props.supervisionStrategy = supervisor
	return props
}

//WithDispatcher assigns a dispatcher to the props
func (props *Props) WithDispatcher(dispatcher mailbox.Dispatcher) *Props {
	props.dispatcher = dispatcher
	return props
}

//WithSpawnFunc assigns a custom spawn func to the props, this is mainly for internal usage
func (props *Props) WithSpawnFunc(spawn SpawnFunc) *Props {
	props.spawner = spawn
	return props
}

//WithFunc assigns a receive func to the props
func (props *Props) WithFunc(f ActorFunc) *Props {
	props.actorProducer = makeProducerFromInstance(f)
	return props
}

//WithInstance creates a custom actor producer from a given instance and assigns it to the props
func (props *Props) WithInstance(a Actor) *Props {
	props.actorProducer = makeProducerFromInstance(a)
	return props
}

//WithProducer assigns a actor producer to the props
func (props *Props) WithProducer(p Producer) *Props {
	props.actorProducer = p
	return props
}

//FromProducer creates a props whith the given actor producer assigned
func FromProducer(actorProducer Producer) *Props {
	return &Props{actorProducer: actorProducer}
}

//FromFunc creates a props with the given receive func assigned as the actor producer
func FromFunc(f ActorFunc) *Props {
	return FromInstance(f)
}

//FromInstance creates a props with the given instance assigned as the actor producer
func FromInstance(template Actor) *Props {
	return &Props{actorProducer: makeProducerFromInstance(template)}
}

func makeProducerFromInstance(a Actor) Producer {
	return func() Actor {
		return a
	}
}

func FromSpawnFunc(spawn SpawnFunc) *Props {
	return &Props{spawner: spawn}
}
