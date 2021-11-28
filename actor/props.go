package actor

import (
	"context"
	"errors"

	"github.com/AsynkronIT/protoactor-go/mailbox"
	"github.com/AsynkronIT/protoactor-go/metrics"
	"go.opentelemetry.io/otel/metric"
)

// Props types
type SpawnFunc func(actorSystem *ActorSystem, id string, props *Props, parentContext SpawnerContext) (*PID, error)
type ReceiverMiddleware func(next ReceiverFunc) ReceiverFunc
type SenderMiddleware func(next SenderFunc) SenderFunc
type ContextDecorator func(next ContextDecoratorFunc) ContextDecoratorFunc
type SpawnMiddleware func(next SpawnFunc) SpawnFunc

// Default values
var (
	defaultDispatcher      = mailbox.NewDefaultDispatcher(300)
	defaultMailboxProducer = mailbox.Unbounded()
	defaultSpawner         = func(actorSystem *ActorSystem, id string, props *Props, parentContext SpawnerContext) (*PID, error) {
		ctx := newActorContext(actorSystem, props, parentContext.Self())
		mb := props.produceMailbox()

		// prepare the mailbox number counter
		sysMetrics, ok := ctx.actorSystem.Extensions.Get(extensionId).(*Metrics)
		if ok && sysMetrics.enabled {
			if instruments := sysMetrics.metrics.Get(metrics.InternalActorMetrics); instruments != nil {
				sysMetrics.PrepareMailboxLengthGauge(
					func(_ context.Context, result metric.Int64ObserverResult) {

						messageCount := int64(mb.UserMessageCount())
						result.Observe(messageCount, sysMetrics.CommonLabels(ctx)...)
					},
				)
			}
		}

		dp := props.getDispatcher()
		proc := NewActorProcess(mb)
		pid, absent := actorSystem.ProcessRegistry.Add(proc, id)
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

func (props *Props) spawn(actorSystem *ActorSystem, name string, parentContext SpawnerContext) (*PID, error) {
	return props.getSpawner()(actorSystem, name, props, parentContext)
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
	props.senderMiddlewareChain = makeSenderMiddlewareChain(props.senderMiddleware, func(sender SenderContext, target *PID, envelope *MessageEnvelope) {
		target.sendUserMessage(sender.ActorSystem(), envelope)
	})

	return props
}

// WithSpawnFunc assigns a custom spawn func to the props, this is mainly for internal usage
func (props *Props) WithSpawnFunc(spawn SpawnFunc) *Props {
	props.spawner = spawn
	return props
}

// WithFunc assigns a receive func to the props
func (props *Props) WithFunc(f ReceiveFunc) *Props {
	props.producer = func() Actor { return f }
	return props
}

func (props *Props) WithSpawnMiddleware(middleware ...SpawnMiddleware) *Props {
	props.spawnMiddleware = append(props.spawnMiddleware, middleware...)

	// Construct the spawner middleware chain with the final spawner at the end
	props.spawnMiddlewareChain = makeSpawnMiddlewareChain(props.spawnMiddleware, func(actorSystem *ActorSystem, id string, props *Props, parentContext SpawnerContext) (pid *PID, e error) {
		if props.spawner == nil {
			return defaultSpawner(actorSystem, id, props, parentContext)
		}
		return props.spawner(actorSystem, id, props, parentContext)
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
func PropsFromFunc(f ReceiveFunc) *Props {
	return PropsFromProducer(func() Actor { return f })
}
