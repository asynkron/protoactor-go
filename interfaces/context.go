package interfaces

type Context interface {
	Watch(ActorRef)
	Unwatch(ActorRef)
	Message() interface{}
	Become(Receive)
	BecomeStacked(Receive)
	UnbecomeStacked()
	Self() ActorRef
	Parent() ActorRef
	SpawnChild(Props) ActorRef
}
