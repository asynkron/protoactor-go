package routing

import "github.com/AsynkronIT/gam/actor"

type BroadcastGroupRouter struct {
	actor.GroupRouter
}

type BroadcastPoolRouter struct {
	actor.PoolRouter
}

type BroadcastRouterState struct {
	routees []*actor.PID
}

func (state *BroadcastRouterState) SetRoutees(routees []*actor.PID) {
	state.routees = routees
}

func (state *BroadcastRouterState) GetRoutees() []*actor.PID {
	return state.routees
}

func (state *BroadcastRouterState) RouteMessage(message interface{}, sender *actor.PID) {
	for _, m := range state.routees {
		m.Request(message, sender)
	}
}

func NewBroadcastPool(poolSize int) actor.PoolRouterConfig {
	r := &BroadcastPoolRouter{}
	r.PoolSize = poolSize
	return r
}

func NewBroadcastGroup(routees ...*actor.PID) actor.GroupRouterConfig {
	r := &BroadcastGroupRouter{}
	r.Routees = routees
	return r
}

func (config *BroadcastPoolRouter) CreateRouterState() actor.RouterState {
	return &BroadcastRouterState{}
}

func (config *BroadcastGroupRouter) CreateRouterState() actor.RouterState {
	return &BroadcastRouterState{}
}
