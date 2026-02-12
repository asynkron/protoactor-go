package actor

type PropsOption func(props *Props)

// WithOnInit sets callback functions invoked when the actor is initialized, before processing messages.
func WithOnInit(init ...func(ctx Context)) PropsOption {
	return func(props *Props) {
		props.onInit = append(props.onInit, init...)
	}
}

// WithProducer assigns a producer function that creates new actor instances, called on initial spawn and restarts.
func WithProducer(p Producer) PropsOption {
	return func(props *Props) {
		props.producer = func(*ActorSystem) Actor { return p() }
	}
}

// WithDispatcher sets the dispatcher used for scheduling actor message processing.
func WithDispatcher(dispatcher Dispatcher) PropsOption {
	return func(props *Props) {
		props.dispatcher = dispatcher
	}
}

// WithMailbox sets the mailbox producer for the actor, controlling message buffering and delivery order.
func WithMailbox(mailbox MailboxProducer) PropsOption {
	return func(props *Props) {
		props.mailboxProducer = mailbox
	}
}

// WithContextDecorator wraps the actor context with custom decorators for cross-cutting concerns.
func WithContextDecorator(contextDecorator ...ContextDecorator) PropsOption {
	return func(props *Props) {
		props.contextDecorator = append(props.contextDecorator, contextDecorator...)

		props.contextDecoratorChain = makeContextDecoratorChain(props.contextDecorator, func(ctx Context) Context {
			return ctx
		})
	}
}

// WithGuardian sets the guardian strategy for actors spawned at the root level.
func WithGuardian(guardian SupervisorStrategy) PropsOption {
	return func(props *Props) {
		props.guardianStrategy = guardian
	}
}

// WithSupervisor sets the supervision strategy that determines how child actor failures are handled.
func WithSupervisor(supervisor SupervisorStrategy) PropsOption {
	return func(props *Props) {
		props.supervisionStrategy = supervisor
	}
}

// WithReceiverMiddleware adds middleware that intercepts incoming messages before the actor receives them.
func WithReceiverMiddleware(middleware ...ReceiverMiddleware) PropsOption {
	return func(props *Props) {
		props.receiverMiddleware = append(props.receiverMiddleware, middleware...)

		// Construct the receiver middleware chain with the final receiver at the end
		props.receiverMiddlewareChain = makeReceiverMiddlewareChain(props.receiverMiddleware, func(ctx ReceiverContext, envelope *MessageEnvelope) {
			ctx.Receive(envelope)
		})
	}
}

// WithSenderMiddleware adds middleware that intercepts outgoing messages before they are sent.
func WithSenderMiddleware(middleware ...SenderMiddleware) PropsOption {
	return func(props *Props) {
		props.senderMiddleware = append(props.senderMiddleware, middleware...)

		// Construct the sender middleware chain with the final sender at the end
		props.senderMiddlewareChain = makeSenderMiddlewareChain(props.senderMiddleware, func(sender SenderContext, target *PID, envelope *MessageEnvelope) {
			target.sendUserMessage(sender.ActorSystem(), envelope)
		})
	}
}

// WithSpawnFunc sets a custom function used to spawn the actor process.
func WithSpawnFunc(spawn SpawnFunc) PropsOption {
	return func(props *Props) {
		props.spawner = spawn
	}
}

// WithFunc creates Props from a simple receive function, shorthand for WithProducer wrapping a function.
func WithFunc(f ReceiveFunc) PropsOption {
	return func(props *Props) {
		props.producer = func(system *ActorSystem) Actor { return f }
	}
}

// WithSpawnMiddleware adds middleware that intercepts the actor spawn process.
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

// PropsFromProducer creates a props with the given actor producer assigned.
func PropsFromProducer(producer Producer, opts ...PropsOption) *Props {
	p := &Props{
		producer:         func(*ActorSystem) Actor { return producer() },
		contextDecorator: make([]ContextDecorator, 0),
	}
	p.Configure(opts...)

	return p
}

// PropsFromProducerWithActorSystem creates a props with the given actor producer that receives the ActorSystem.
func PropsFromProducerWithActorSystem(producer ProducerWithActorSystem, opts ...PropsOption) *Props {
	p := &Props{
		producer:         producer,
		contextDecorator: make([]ContextDecorator, 0),
	}
	p.Configure(opts...)

	return p
}

// PropsFromFunc creates a props with the given receive func assigned as the actor producer.
func PropsFromFunc(f ReceiveFunc, opts ...PropsOption) *Props {
	p := PropsFromProducer(func() Actor { return f }, opts...)

	return p
}

func (props *Props) Clone(opts ...PropsOption) *Props {
	cp := PropsFromProducerWithActorSystem(props.producer,
		WithDispatcher(props.dispatcher),
		WithMailbox(props.mailboxProducer),
		WithContextDecorator(props.contextDecorator...),
		WithGuardian(props.guardianStrategy),
		WithSupervisor(props.supervisionStrategy),
		WithReceiverMiddleware(props.receiverMiddleware...),
		WithSenderMiddleware(props.senderMiddleware...),
		WithSpawnFunc(props.spawner),
		WithSpawnMiddleware(props.spawnMiddleware...),
		WithOnInit(props.onInit...),
	)

	cp.Configure(opts...)

	return cp
}
