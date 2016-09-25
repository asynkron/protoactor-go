package actor

type RouterState interface {
	Route(message interface{})
	SetRoutees(routees []*PID)
}
