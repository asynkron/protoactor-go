package actor

type Context interface {
	Watch(ActorRef)
	Unwatch(ActorRef)
	Message() interface{}
	Become(Receive)
	BecomeStacked(Receive)
	UnbecomeStacked()
	Self() ActorRef
	Parent() ActorRef
	SpawnChild(Properties) ActorRef
}

type ContextValue struct {
	*ActorCell
	message interface{}
}

func (context *ContextValue) Message() interface{} {
	return context.message
}

func NewContext(cell *ActorCell, message interface{}) Context {
	res := &ContextValue{
		ActorCell: cell,
		message:   message,
	}
	return res
}
