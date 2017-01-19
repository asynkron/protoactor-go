package actor

//Producer is a function that can create an actor
type Producer func() Actor

//Actor is the interface for actors, it defines the Receive method
type Actor interface {
	Receive(Context)
}

// The ReceiveFunc type is an adapter to allow the use of ordinary functions as actors to process messages
type ReceiveFunc func(Context)

// Receive calls f(context)
func (f ReceiveFunc) Receive(context Context) {
	f(context)
}
