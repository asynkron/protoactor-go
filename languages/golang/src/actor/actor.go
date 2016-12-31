package actor

//Producer is a function that can create an actor
type Producer func() Actor

//Actor is the interface for actors, it defines the Receive method
type Actor interface {
	Receive(Context)
}

//Receive is a function that receives an actor context
type Receive func(Context)

func (f Receive) Receive(context Context) {
	f(context)
}
