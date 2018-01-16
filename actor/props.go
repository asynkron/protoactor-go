package actor

import "github.com/AsynkronIT/protoactor-go/mailbox"

type InboundMiddleware func(next ActorFunc) ActorFunc
type OutboundMiddleware func(next SenderFunc) SenderFunc

// Props represents configuration to define how an actor should be created
type Props struct {
	actorProducer       Producer
	mailboxProducer     mailbox.Producer
	guardianStrategy    SupervisorStrategy
	supervisionStrategy SupervisorStrategy
	inboundMiddleware   []InboundMiddleware
	outboundMiddleware  []OutboundMiddleware
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

// Assign one or more middlewares to the props
func (props *Props) WithMiddleware(middleware ...InboundMiddleware) *Props {
	props.inboundMiddleware = append(props.inboundMiddleware, middleware...)
	return props
}

func (props *Props) WithOutboundMiddleware(middleware ...OutboundMiddleware) *Props {
	props.outboundMiddleware = append(props.outboundMiddleware, middleware...)
	return props
}

//WithMailbox assigns the desired mailbox producer to the props
func (props *Props) WithMailbox(mailbox mailbox.Producer) *Props {
	props.mailboxProducer = mailbox
	return props
}

//WithGuardian assigns a guardian strategy to the props
func (props *Props) WithGuardian(guardian SupervisorStrategy) *Props {
	props.guardianStrategy = guardian
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
	props.actorProducer = func() Actor { return f }
	return props
}

//WithProducer assigns a actor producer to the props
func (props *Props) WithProducer(p Producer) *Props {
	props.actorProducer = p
	return props
}

//Deprecated: WithInstance is deprecated.
func (props *Props) WithInstance(a Actor) *Props {
	props.actorProducer = makeProducerFromInstance(a)
	return props
}
