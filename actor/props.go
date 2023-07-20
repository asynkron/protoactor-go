package actor

import (
	"context"
	"errors"
	"fmt"

	"github.com/asynkron/protoactor-go/log"
	"github.com/asynkron/protoactor-go/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

type (
	SpawnFunc          func(actorSystem *ActorSystem, id string, props *Props, parentContext SpawnerContext) (*PID, error)
	ReceiverMiddleware func(next ReceiverFunc) ReceiverFunc
	SenderMiddleware   func(next SenderFunc) SenderFunc
	ContextDecorator   func(next ContextDecoratorFunc) ContextDecoratorFunc
	SpawnMiddleware    func(next SpawnFunc) SpawnFunc
)

// Default values.
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
					sysMetrics.PrepareMailboxLengthGauge()
					meter := otel.Meter(metrics.LibName)

					if _, err := meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
						o.ObserveInt64(instruments.ActorMailboxLength, int64(mb.UserMessageCount()), metric.WithAttributes(sysMetrics.CommonLabels(ctx)...))
						return nil
					}); err != nil {
						err = fmt.Errorf("failed to instrument Actor Mailbox, %w", err)
						plog.Error(err.Error(), log.Error(err))
					}
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

// DefaultSpawner this is a hacking way to allow Proto.Router access default spawner func.
var DefaultSpawner SpawnFunc = defaultSpawner

// ErrNameExists is the error used when an existing name is used for spawning an actor.
var ErrNameExists = errors.New("spawn: name exists")

// Props represents configuration to define how an actor should be created.
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
