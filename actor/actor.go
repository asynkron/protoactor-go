package actor

// The Producer type is a function that creates a new actor
type Producer func() Actor

// Actor is the interface that defines the Receive method.
//
// Receive is sent messages to be processed from the mailbox associated with the instance of the actor
type Actor interface {
	Receive(c Context)
}

// The ActorFunc type is an adapter to allow the use of ordinary functions as actors to process messages
type ActorFunc func(c Context)

// Receive calls f(c)
func (f ActorFunc) Receive(c Context) {
	f(c)
}

type ReceiverFunc func(c ReceiverContext, envelope *MessageEnvelope)

type SenderFunc func(c SenderContext, target *PID, envelope *MessageEnvelope)

type ContextDecoratorFunc func(ctx Context) Context
