package actor

import (
	"context"
	"errors"

	"github.com/asynkron/protoactor-go/metrics"
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
	defaultDispatcher      = NewDefaultDispatcher(300)
	defaultMailboxProducer = Unbounded()
	defaultSpawner         = func(actorSystem *ActorSystem, id string, props *Props, parentContext SpawnerContext) (*PID, error) {
		ctx := newActorContext(actorSystem, props, parentContext.Self())
		mb := props.produceMailbox()

		// prepare the mailbox number counter
		if ctx.actorSystem.Config.MetricsProvider != nil {
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
		}

		dp := props.getDispatcher()
		proc := NewActorProcess(mb)
		pid, absent := actorSystem.ProcessRegistry.Add(proc, id)
		if !absent {
			return pid, ErrNameExists
		}
		ctx.self = pid

		initialize(props, ctx)

		mb.RegisterHandlers(ctx, dp)
		mb.PostSystemMessage(startedMessage)
		mb.Start()

		return pid, nil
	}
	defaultContextDecorator = func(ctx Context) Context {
		return ctx
	}
)

func initialize(props *Props, ctx *actorContext) {
	if props.onInit == nil {
		return
	}

	for _, init := range props.onInit {
		init(ctx)
	}
}

// DefaultSpawner this is a hacking way to allow Proto.Router access default spawner func
var DefaultSpawner SpawnFunc = defaultSpawner

// ErrNameExists is the error used when an existing name is used for spawning an actor.
var ErrNameExists = errors.New("spawn: name exists")

// Props represents configuration to define how an actor should be created
type Props struct {
	spawner                 SpawnFunc
	producer                Producer
	mailboxProducer         MailboxProducer
	guardianStrategy        SupervisorStrategy
	supervisionStrategy     SupervisorStrategy
	dispatcher              Dispatcher
	receiverMiddleware      []ReceiverMiddleware
	senderMiddleware        []SenderMiddleware
	spawnMiddleware         []SpawnMiddleware
	receiverMiddlewareChain ReceiverFunc
	senderMiddlewareChain   SenderFunc
	spawnMiddlewareChain    SpawnFunc
	contextDecorator        []ContextDecorator
	contextDecoratorChain   ContextDecoratorFunc
	onInit                  []func(ctx Context)
}

func (props *Props) getSpawner() SpawnFunc {
	if props.spawner == nil {
		return defaultSpawner
	}
	return props.spawner
}

func (props *Props) getDispatcher() Dispatcher {
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

func (props *Props) produceMailbox() Mailbox {
	if props.mailboxProducer == nil {
		return defaultMailboxProducer()
	}
	return props.mailboxProducer()
}

func (props *Props) spawn(actorSystem *ActorSystem, name string, parentContext SpawnerContext) (*PID, error) {
	return props.getSpawner()(actorSystem, name, props, parentContext)
}

func (props *Props) Configure(opts ...PropsOption) *Props {
	for _, opt := range opts {
		opt(props)
	}
	return props
}

//props options
type PropsOption func(props *Props)

func WithOnInit(init ...func(ctx Context)) PropsOption {
	return func(props *Props) {
		props.onInit = append(props.onInit, init...)
	}
}
func WithProducer(p Producer) PropsOption {
	return func(props *Props) {
		props.producer = p
	}
}

func WithDispatcher(dispatcher Dispatcher) PropsOption {
	return func(props *Props) {
		props.dispatcher = dispatcher
	}
}

func WithMailbox(mailbox MailboxProducer) PropsOption {
	return func(props *Props) {
		props.mailboxProducer = mailbox
	}
}

func WithContextDecorator(contextDecorator ...ContextDecorator) PropsOption {
	return func(props *Props) {
		props.contextDecorator = append(props.contextDecorator, contextDecorator...)

		props.contextDecoratorChain = makeContextDecoratorChain(props.contextDecorator, func(ctx Context) Context {
			return ctx
		})
	}
}

func WithGuardian(guardian SupervisorStrategy) PropsOption {
	return func(props *Props) {
		props.guardianStrategy = guardian
	}
}

func WithSupervisor(supervisor SupervisorStrategy) PropsOption {
	return func(props *Props) {
		props.supervisionStrategy = supervisor
	}
}

func WithReceiverMiddleware(middleware ...ReceiverMiddleware) PropsOption {
	return func(props *Props) {
		props.receiverMiddleware = append(props.receiverMiddleware, middleware...)

		// Construct the receiver middleware chain with the final receiver at the end
		props.receiverMiddlewareChain = makeReceiverMiddlewareChain(props.receiverMiddleware, func(ctx ReceiverContext, envelope *MessageEnvelope) {
			ctx.Receive(envelope)
		})
	}
}

func WithSenderMiddleware(middleware ...SenderMiddleware) PropsOption {
	return func(props *Props) {
		props.senderMiddleware = append(props.senderMiddleware, middleware...)

		// Construct the sender middleware chain with the final sender at the end
		props.senderMiddlewareChain = makeSenderMiddlewareChain(props.senderMiddleware, func(sender SenderContext, target *PID, envelope *MessageEnvelope) {
			target.sendUserMessage(sender.ActorSystem(), envelope)
		})
	}
}

func WithSpawnFunc(spawn SpawnFunc) PropsOption {
	return func(props *Props) {
		props.spawner = spawn
	}
}

func WithFunc(f ReceiveFunc) PropsOption {
	return func(props *Props) {
		props.producer = func() Actor { return f }
	}
}

func WithSpawnMiddleware(middleware ...SpawnMiddleware) PropsOption {
	return func(props *Props) {
		props.spawnMiddleware = append(props.spawnMiddleware, middleware...)

		// Construct the spawner middleware chain with the final spawner at the end
		props.spawnMiddlewareChain = makeSpawnMiddlewareChain(props.spawnMiddleware, func(actorSystem *ActorSystem, id string, props *Props, parentContext SpawnerContext) (pid *PID, e error) {
			if props.spawner == nil {
				return defaultSpawner(actorSystem, id, props, parentContext)
			}
			return props.spawner(actorSystem, id, props, parentContext)
		})
	}
}

// PropsFromProducer creates a props with the given actor producer assigned
func PropsFromProducer(producer Producer, opts ...PropsOption) *Props {
	p := &Props{
		producer:         producer,
		contextDecorator: make([]ContextDecorator, 0),
	}
	p.Configure(opts...)
	return p
}

// PropsFromFunc creates a props with the given receive func assigned as the actor producer
func PropsFromFunc(f ReceiveFunc, opts ...PropsOption) *Props {
	p := PropsFromProducer(func() Actor { return f }, opts...)
	return p
}

func (props *Props) Clone(opts ...PropsOption) *Props {
	cp :=
		PropsFromProducer(props.producer,
			WithDispatcher(props.dispatcher),
			WithMailbox(props.mailboxProducer),
			WithContextDecorator(props.contextDecorator...),
			WithGuardian(props.guardianStrategy),
			WithSupervisor(props.supervisionStrategy),
			WithReceiverMiddleware(props.receiverMiddleware...),
			WithSenderMiddleware(props.senderMiddleware...),
			WithSpawnFunc(props.spawner),
			WithSpawnMiddleware(props.spawnMiddleware...))

	cp.Configure(opts...)
	return cp
}
