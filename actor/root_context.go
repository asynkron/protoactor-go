package actor

import (
	"log/slog"
	"time"
)

// RootContext is the entry point used to interact with the actor system.
type RootContext struct {
	actorSystem      *ActorSystem
	senderMiddleware SenderFunc
	spawnMiddleware  SpawnFunc
	headers          messageHeader
	guardianStrategy SupervisorStrategy
}

var (
	_ SenderContext  = &RootContext{}
	_ SpawnerContext = &RootContext{}
	_ stopperPart    = &RootContext{}
)

func NewRootContext(actorSystem *ActorSystem, header map[string]string, middleware ...SenderMiddleware) *RootContext {
	if header == nil {
		header = make(map[string]string)
	}

	return &RootContext{
		actorSystem: actorSystem,
		senderMiddleware: makeSenderMiddlewareChain(middleware, func(_ SenderContext, target *PID, envelope *MessageEnvelope) {
			target.sendUserMessage(actorSystem, envelope)
		}),
		headers: header,
	}
}

func (rc RootContext) Copy() *RootContext {
	return &rc
}

func (rc *RootContext) ActorSystem() *ActorSystem {
	return rc.actorSystem
}

func (rc *RootContext) Logger() *slog.Logger {
	return rc.actorSystem.Logger()
}

// WithHeaders sets the message headers used for subsequent sends and requests.
func (rc *RootContext) WithHeaders(headers map[string]string) *RootContext {
	rc.headers = headers

	return rc
}

// WithSenderMiddleware applies sender middleware to outgoing messages.
func (rc *RootContext) WithSenderMiddleware(middleware ...SenderMiddleware) *RootContext {
	rc.senderMiddleware = makeSenderMiddlewareChain(middleware, func(_ SenderContext, target *PID, envelope *MessageEnvelope) {
		target.sendUserMessage(rc.actorSystem, envelope)
	})

	return rc
}

// WithSpawnMiddleware applies spawn middleware when creating child actors.
func (rc *RootContext) WithSpawnMiddleware(middleware ...SpawnMiddleware) *RootContext {
	rc.spawnMiddleware = makeSpawnMiddlewareChain(middleware, func(actorSystem *ActorSystem, id string, props *Props, _ SpawnerContext) (pid *PID, e error) {
		return props.spawn(actorSystem, id, rc)
	})

	return rc
}

// WithGuardian sets a guardian strategy used when spawning actors.
func (rc *RootContext) WithGuardian(guardian SupervisorStrategy) *RootContext {
	rc.guardianStrategy = guardian

	return rc
}

//
// Interface: info
//

// Parent returns the PID of the parent actor. RootContext has no parent and returns nil.
func (rc *RootContext) Parent() *PID {
	return nil
}

// Self returns the PID representing the root context.
func (rc *RootContext) Self() *PID {
	if rc.guardianStrategy != nil {
		return rc.actorSystem.Guardians.getGuardianPid(rc.guardianStrategy)
	}

	return nil
}

// Sender returns the PID of the message sender if available.
func (rc *RootContext) Sender() *PID {
	return nil
}

// Actor always returns nil for RootContext since it does not represent a real actor.
func (rc *RootContext) Actor() Actor {
	return nil
}

//
// Interface: sender
//

// Message always returns nil for RootContext since it is not processing a message.
func (rc *RootContext) Message() interface{} {
	return nil
}

// MessageHeader returns the headers attached to the current context.
func (rc *RootContext) MessageHeader() ReadonlyMessageHeader {
	return rc.headers
}

// Send delivers a message to the given PID using the configured middleware chain.
func (rc *RootContext) Send(pid *PID, message interface{}) {
	rc.sendUserMessage(pid, message)
}

// Request sends a message to the given PID expecting a response.
func (rc *RootContext) Request(pid *PID, message interface{}) {
	rc.sendUserMessage(pid, message)
}

// RequestWithCustomSender sends a message on behalf of the provided sender PID.
func (rc *RootContext) RequestWithCustomSender(pid *PID, message interface{}, sender *PID) {
	env := &MessageEnvelope{
		Header:  nil,
		Message: message,
		Sender:  sender,
	}
	rc.sendUserMessage(pid, env)
}

// RequestFuture sends a message to a given PID and returns a Future.
func (rc *RootContext) RequestFuture(pid *PID, message interface{}, timeout time.Duration) Future {
	future := NewFuture(rc.actorSystem, timeout)
	env := &MessageEnvelope{
		Header:  nil,
		Message: message,
		Sender:  future.PID(),
	}
	rc.sendUserMessage(pid, env)

	return future
}

func (rc *RootContext) sendUserMessage(pid *PID, message interface{}) {
	if rc.senderMiddleware != nil {
		// Request based middleware
		rc.senderMiddleware(rc, pid, WrapEnvelope(message))
	} else {
		// tell based middleware
		pid.sendUserMessage(rc.actorSystem, message)
	}
}

//
// Interface: spawner
//

// Spawn starts a new actor based on props and named with a unique id.
func (rc *RootContext) Spawn(props *Props) *PID {
	pid, err := rc.SpawnNamed(props, rc.actorSystem.ProcessRegistry.NextID())
	if err != nil {
		panic(err)
	}

	return pid
}

// SpawnPrefix starts a new actor based on props and named using a prefix followed by a unique id.
func (rc *RootContext) SpawnPrefix(props *Props, prefix string) *PID {
	pid, err := rc.SpawnNamed(props, prefix+rc.actorSystem.ProcessRegistry.NextID())
	if err != nil {
		panic(err)
	}

	return pid
}

// SpawnNamed starts a new actor based on props and named using the specified name
//
// # ErrNameExists will be returned if id already exists
//
// Please do not use name sharing same pattern with system actors, for example "YourPrefix$1", "Remote$1", "future$1".
func (rc *RootContext) SpawnNamed(props *Props, name string) (*PID, error) {
	rootContext := rc
	if props.guardianStrategy != nil {
		rootContext = rc.Copy().WithGuardian(props.guardianStrategy)
	}

	if rootContext.spawnMiddleware != nil {
		return rc.spawnMiddleware(rc.actorSystem, name, props, rootContext)
	}

	return props.spawn(rc.actorSystem, name, rootContext)
}

//
// Interface: StopperContext
//

// Stop will stop actor immediately regardless of existing user messages in mailbox.
func (rc *RootContext) Stop(pid *PID) {
	pid.ref(rc.actorSystem).Stop(pid)
}

// StopFuture will stop actor immediately regardless of existing user messages in mailbox, and return its future.
func (rc *RootContext) StopFuture(pid *PID) Future {
	future := newFuture(rc.actorSystem, 10*time.Second)

	pid.sendSystemMessage(rc.actorSystem, &Watch{Watcher: future.pid})
	rc.Stop(pid)

	return future
}

// Poison will tell actor to stop after processing current user messages in mailbox.
func (rc *RootContext) Poison(pid *PID) {
	pid.sendUserMessage(rc.actorSystem, poisonPillMessage)
}

// PoisonFuture will tell actor to stop after processing current user messages in mailbox, and return its future.
func (rc *RootContext) PoisonFuture(pid *PID) Future {
	future := newFuture(rc.actorSystem, 10*time.Second)

	pid.sendSystemMessage(rc.actorSystem, &Watch{Watcher: future.pid})
	rc.Poison(pid)

	return future
}
