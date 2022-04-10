package router

import "github.com/asynkron/protoactor-go/actor"

// A type that satisfies router.Interface can be used as a router
type State interface {
	RouteMessage(message interface{})
	SetRoutees(routees *actor.PIDSet)
	GetRoutees() *actor.PIDSet
	SetSender(sender actor.SenderContext)
}
