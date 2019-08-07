package actor

import (
	"errors"

	"github.com/AsynkronIT/protoactor-go/mailbox"
)

// Props types
type SpawnFunc func(id string, props *Props, parentContext SpawnerContext) (*PID, error)
type ReceiverMiddleware func(next ReceiverFunc) ReceiverFunc
type SenderMiddleware func(next SenderFunc) SenderFunc
type ContextDecorator func(next ContextDecoratorFunc) ContextDecoratorFunc
type SpawnMiddleware func(next SpawnFunc) SpawnFunc

// Default values
var (
	defaultDispatcher      = mailbox.NewDefaultDispatcher(300)
	defaultMailboxProducer = mailbox.Unbounded()
	defaultSpawner         = func(id string, props *Props, parentContext SpawnerContext) (*PID, error) {
		ctx := newActorContext(props, parentContext.Self())
		mb := props.produceMailbox()
		dp := props.getDispatcher()
		proc := NewActorProcess(mb)
		pid, absent := ProcessRegistry.Add(proc, id)
		if !absent {
			return pid, ErrNameExists
		}
		ctx.self = pid
		mb.Start()
		mb.RegisterHandlers(ctx, dp)
		mb.PostSystemMessage(startedMessage)

		return pid, nil
	}
	defaultContextDecorator = func(ctx Context) Context {
		return ctx
	}
)

// DefaultSpawner this is a hacking way to allow Proto.Router access default spawner func
var DefaultSpawner SpawnFunc = defaultSpawner

// ErrNameExists is the error used when an existing name is used for spawning an actor.
var ErrNameExists = errors.New("spawn: name exists")

// Props represents configuration to define how an actor should be created
type Props struct {
	spawner                 SpawnFunc
	producer                Producer
	mailboxProducer         mailbox.Producer
	guardianStrategy        SupervisorStrategy
	supervisionStrategy     SupervisorStrategy
	dispatcher              mailbox.Dispatcher
	receiverMiddleware      []ReceiverMiddleware
	senderMiddleware        []SenderMiddleware
	spawnMiddleware         []SpawnMiddleware
	receiverMiddlewareChain ReceiverFunc
	senderMiddlewareChain   SenderFunc
	spawnMiddlewareChain    SpawnFunc
	contextDecorator        []ContextDecorator
	contextDecoratorChain   ContextDecoratorFunc
}

func (props *Props) getSpawner() SpawnFunc {
	if props.spawner == nil {
		return defaultSpawner
	}
	return props.spawner
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

func (props *Props) getContextDecoratorChain() ContextDecoratorFunc {
	if props.contextDecoratorChain == nil {
		return defaultContextDecorator
	}
	return props.contextDecoratorChain
}

func (props *Props) produceMailbox() mailbox.Mailbox {
	if props.mailboxProducer == nil {
		return defaultMailboxProducer()
	}
	return props.mailboxProducer()
}

func (props *Props) spawn(name string, parentContext SpawnerContext) (*PID, error) {
	return props.getSpawner()(name, props, parentContext)
}

// WithProducer assigns a actor producer to the props
func (props *Props) WithProducer(p Producer) *Props {
	props.producer = p
	return props
}

// WithDispatcher assigns a dispatcher to the props
func (props *Props) WithDispatcher(dispatcher mailbox.Dispatcher) *Props {
	props.dispatcher = dispatcher
	return props
}

// WithMailbox assigns the desired mailbox producer to the props
func (props *Props) WithMailbox(mailbox mailbox.Producer) *Props {
	props.mailboxProducer = mailbox
	return props
}

// WithContextDecorator assigns context decorator to the props
func (props *Props) WithContextDecorator(contextDecorator ...ContextDecorator) *Props {
	props.contextDecorator = append(props.contextDecorator, contextDecorator...)

	props.contextDecoratorChain = makeContextDecoratorChain(props.contextDecorator, func(ctx Context) Context {
		return ctx
	})

	return props
}

// WithGuardian assigns a guardian strategy to the props
func (props *Props) WithGuardian(guardian SupervisorStrategy) *Props {
	props.guardianStrategy = guardian
	return props
}

// WithSupervisor assigns a supervision strategy to the props
func (props *Props) WithSupervisor(supervisor SupervisorStrategy) *Props {
	props.supervisionStrategy = supervisor
	return props
}

// Assign one or more middleware to the props
func (props *Props) WithReceiverMiddleware(middleware ...ReceiverMiddleware) *Props {
	props.receiverMiddleware = append(props.receiverMiddleware, middleware...)

	// Construct the receiver middleware chain with the final receiver at the end
	props.receiverMiddlewareChain = makeReceiverMiddlewareChain(props.receiverMiddleware, func(ctx ReceiverContext, envelope *MessageEnvelope) {
		ctx.Receive(envelope)
	})

	return props
}

func (props *Props) WithSenderMiddleware(middleware ...SenderMiddleware) *Props {
	props.senderMiddleware = append(props.senderMiddleware, middleware...)

	// Construct the sender middleware chain with the final sender at the end
	props.senderMiddlewareChain = makeSenderMiddlewareChain(props.senderMiddleware, func(_ SenderContext, target *PID, envelope *MessageEnvelope) {
		target.sendUserMessage(envelope)
	})

	return props
}

// WithSpawnFunc assigns a custom spawn func to the props, this is mainly for internal usage
func (props *Props) WithSpawnFunc(spawn SpawnFunc) *Props {
	props.spawner = spawn
	return props
}

// WithFunc assigns a receive func to the props
func (props *Props) WithFunc(f ActorFunc) *Props {
	props.producer = func() Actor { return f }
	return props
}

func (props *Props) WithSpawnMiddleware(middleware ...SpawnMiddleware) *Props {
	props.spawnMiddleware = append(props.spawnMiddleware, middleware...)

	// Construct the spawner middleware chain with the final spawner at the end
	props.spawnMiddlewareChain = makeSpawnMiddlewareChain(props.spawnMiddleware, func(id string, props *Props, parentContext SpawnerContext) (pid *PID, e error) {
		if props.spawner == nil {
			return defaultSpawner(id, props, parentContext)
		}
		return props.spawner(id, props, parentContext)
	})

	return props
}

// PropsFromProducer creates a props with the given actor producer assigned
func PropsFromProducer(producer Producer) *Props {
	return &Props{
		producer:         producer,
		contextDecorator: make([]ContextDecorator, 0),
	}
}

// PropsFromFunc creates a props with the given receive func assigned as the actor producer
func PropsFromFunc(f ActorFunc) *Props {
	return PropsFromProducer(func() Actor { return f })
}

// Deprecated: Use actor.PropsFromProducer instead.
func FromProducer(actorProducer Producer) *Props {
	return PropsFromProducer(actorProducer)
}

// Deprecated: Use actor.PropsFromFunc instead.
func FromFunc(f ActorFunc) *Props {
	return PropsFromFunc(f)
}

// Deprecated: FromSpawnFunc is deprecated.
func FromSpawnFunc(spawn SpawnFunc) *Props {
	return (&Props{}).WithSpawnFunc(spawn)
}

// Deprecated: Use ReceiverMiddleware instead
type InboundMiddleware func(f ActorFunc) ActorFunc

// Deprecated: Use WithReceiverMiddleware instead
func (props *Props) WithMiddleware(middleware ...InboundMiddleware) *Props {
	plog.Error("props.WithMiddleware(middleware ...InboundMiddleware) has been deprecated. Please use WithReceiverMiddleware instead. This middleware will not be applied")
	return props
}

// Deprecated: Use SenderMiddleware instead
type OutboundMiddleware func(next SenderFunc) SenderFunc

// Deprecated: Use WithSenderMiddleware instead
func (props *Props) WithOutboundMiddleware(middleware ...OutboundMiddleware) *Props {
	plog.Error("props.WithOutboundMiddleware(middleware ...OutboundMiddleware) has been deprecated. Please use WithSenderMiddleware instead. This middleware will not be applied")
	return props
}
