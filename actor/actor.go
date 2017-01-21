package actor

// The Producer type is a function that creates a new actor
type Producer func() Actor

// Actor is the interface for actors, it defines the Receive method
type Actor interface {
	Receive(context Context)
}

// The ReceiveFunc type is an adapter to allow the use of ordinary functions as actors to process messages
type ReceiveFunc func(context Context)

// Receive calls f(context)
func (f ReceiveFunc) Receive(context Context) {
	f(context)
}
