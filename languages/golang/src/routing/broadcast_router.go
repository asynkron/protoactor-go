package routing

import "github.com/AsynkronIT/gam/languages/golang/src/actor"

type BroadcastGroupRouter struct {
	actor.GroupRouter
}

type BroadcastPoolRouter struct {
	actor.PoolRouter
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

func NewBroadcastPool(poolSize int) actor.PoolRouterConfig {
	r := &BroadcastPoolRouter{}
	r.PoolSize = poolSize
	return r
}

func NewBroadcastGroup(routees ...*actor.PID) actor.GroupRouterConfig {
	r := &BroadcastGroupRouter{}
	r.Routees = actor.NewPIDSet(routees...)
	return r
}

func (config *BroadcastPoolRouter) CreateRouterState() actor.RouterState {
	return &BroadcastRouterState{}
}

func (config *BroadcastGroupRouter) CreateRouterState() actor.RouterState {
	return &BroadcastRouterState{}
}
