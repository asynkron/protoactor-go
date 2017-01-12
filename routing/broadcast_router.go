package routing

import "github.com/AsynkronIT/protoactor-go/actor"

type BroadcastGroupRouter struct {
	GroupRouter
}

type BroadcastPoolRouter struct {
	PoolRouter
}

type BroadcastRouterState struct {
	routees *actor.PIDSet
}

func (state *BroadcastRouterState) SetRoutees(routees *actor.PIDSet) {
	state.routees = routees
}

func (state *BroadcastRouterState) GetRoutees() *actor.PIDSet {
	return state.routees
}

func (state *BroadcastRouterState) RouteMessage(message interface{}, sender *actor.PID) {
	state.routees.ForEach(func(i int, pid actor.PID) {
		pid.Request(message, sender)
	})
}

func NewBroadcastPool(poolSize int) PoolRouterConfig {
	r := &BroadcastPoolRouter{}
	r.PoolSize = poolSize
	return r
}

func NewBroadcastGroup(routees ...*actor.PID) GroupRouterConfig {
	r := &BroadcastGroupRouter{}
	r.Routees = actor.NewPIDSet(routees...)
	return r
}

func (config *BroadcastPoolRouter) CreateRouterState() RouterState {
	return &BroadcastRouterState{}
}

func (config *BroadcastGroupRouter) CreateRouterState() RouterState {
	return &BroadcastRouterState{}
}
