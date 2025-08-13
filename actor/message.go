package actor

// Producer creates a new actor instance when invoked.
type Producer func() Actor

// ProducerWithActorSystem creates a new actor instance with access to the actor system.
type ProducerWithActorSystem func(system *ActorSystem) Actor

// Actor is the interface that defines the Receive method.
//
// Receive is sent messages to be processed from the mailbox associated with the instance of the actor
type Actor interface {
	Receive(c Context)
}

// The ReceiveFunc type is an adapter to allow the use of ordinary functions as actors to process messages
type ReceiveFunc func(c Context)

// Receive calls f(c)
func (f ReceiveFunc) Receive(c Context) {
	f(c)
}

// ReceiverFunc processes an incoming message envelope.
type ReceiverFunc func(c ReceiverContext, envelope *MessageEnvelope)

// SenderFunc processes an outgoing message envelope before delivery.
type SenderFunc func(c SenderContext, target *PID, envelope *MessageEnvelope)

// ContextDecoratorFunc modifies a context before it is used by an actor.
type ContextDecoratorFunc func(ctx Context) Context
