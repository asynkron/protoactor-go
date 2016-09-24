package actor

type ActorProducer func() Actor
type Actor interface {
	Receive(message Context)
}
type Receive func(Context)
type ReceivePlugin func(Context) interface{}
