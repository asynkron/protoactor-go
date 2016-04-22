package gam

type ActorProducer func() Actor
type Actor interface {
	Receive(message Context)
}
type Receive func(Context)
