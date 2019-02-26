package router

import "github.com/AsynkronIT/protoactor-go/actor"

// router root context
var rootContext = actor.EmptyRootContext

// A type that satisfies router.Interface can be used as a router
type RouterState interface {
	RouteMessage(message interface{})
	SetRoutees(routees *actor.PIDSet)
	GetRoutees() *actor.PIDSet
}
