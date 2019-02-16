package actor

import "time"

type RootContext struct {
	senderMiddleware SenderFunc
	headers          messageHeader
}

var emptyRootContext = &RootContext{
	senderMiddleware: nil,
	headers:          emptyMessageHeader,
}

func EmptyRootContext() *RootContext {
	return emptyRootContext
}

func NewRootContext(header map[string]string, middleware ...SenderMiddleware) *RootContext {
	return &RootContext{
		senderMiddleware: makeSenderMiddlewareChain(middleware, func(_ SenderContext, target *PID, envelope *MessageEnvelope) {
			target.sendUserMessage(envelope)
		}),
		headers: messageHeader(header),
	}
}

func (rc *RootContext) WithHeaders(headers map[string]string) *RootContext {
	rc.headers = headers
	return rc
}

func (rc *RootContext) WithSenderMiddleware(middleware ...SenderMiddleware) *RootContext {
	rc.senderMiddleware = makeSenderMiddlewareChain(middleware, func(_ SenderContext, target *PID, envelope *MessageEnvelope) {
		target.sendUserMessage(envelope)
	})
	return rc
}

//
// Interface: info
//

func (rc *RootContext) Parent() *PID {
	return nil
}

func (rc *RootContext) Self() *PID {
	return nil
}

func (rc *RootContext) Sender() *PID {
	return nil
}

func (rc *RootContext) Actor() Actor {
	return nil
}

//
// Interface: sender
//

func (rc *RootContext) Message() interface{} {
	return nil
}

func (rc *RootContext) MessageHeader() ReadonlyMessageHeader {
	return rc.headers
}

func (rc *RootContext) Send(pid *PID, message interface{}) {
	rc.sendUserMessage(pid, message)
}

func (rc *RootContext) Request(pid *PID, message interface{}) {
	rc.sendUserMessage(pid, message)
}

func (rc *RootContext) RequestWithCustomSender(pid *PID, message interface{}, sender *PID) {
	env := &MessageEnvelope{
		Header:  nil,
		Message: message,
		Sender:  sender,
	}
	rc.sendUserMessage(pid, env)
}

// RequestFuture sends a message to a given PID and returns a Future
func (rc *RootContext) RequestFuture(pid *PID, message interface{}, timeout time.Duration) *Future {
	future := NewFuture(timeout)
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
		if envelope, ok := message.(*MessageEnvelope); ok {
			// Request based middleware
			rc.senderMiddleware(rc, pid, envelope)
		} else {
			// tell based middleware
			rc.senderMiddleware(rc, pid, &MessageEnvelope{nil, message, nil})
		}
		return
	}
	// Default path
	pid.sendUserMessage(message)
}

//
// Interface: spawner
//

// Spawn starts a new actor based on props and named with a unique id
func (rc *RootContext) Spawn(props *Props) *PID {
	pid, err := rc.SpawnNamed(props, ProcessRegistry.NextId())
	if err != nil {
		panic(err)
	}
	return pid
}

// SpawnPrefix starts a new actor based on props and named using a prefix followed by a unique id
func (rc *RootContext) SpawnPrefix(props *Props, prefix string) *PID {
	pid, err := rc.SpawnNamed(props, prefix+ProcessRegistry.NextId())
	if err != nil {
		panic(err)
	}
	return pid
}

// SpawnNamed starts a new actor based on props and named using the specified name
//
// ErrNameExists will be returned if id already exists
//
// Please do not use name sharing same pattern with system actors, for example "YourPrefix$1", "Remote$1", "future$1"
func (rc *RootContext) SpawnNamed(props *Props, name string) (*PID, error) {
	var parent *PID
	if props.guardianStrategy != nil {
		parent = guardians.getGuardianPid(props.guardianStrategy)
	}
	return props.spawn(name, parent)
}
