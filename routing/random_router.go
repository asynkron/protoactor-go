package routing

import (
	"math/rand"

	"github.com/AsynkronIT/gam/actor"
)

type RandomGroupRouter struct {
	actor.GroupRouter
}

type RandomPoolRouter struct {
	actor.PoolRouter
}

type RandomRouterState struct {
	routees []*actor.PID
}

func (state *RandomRouterState) SetRoutees(routees []*actor.PID) {
	state.routees = routees
}

func (state *RandomRouterState) GetRoutees() []*actor.PID {
	return state.routees
}

func (state *RandomRouterState) RouteMessage(message interface{}, sender *actor.PID) {
	l := len(state.routees)
	r := rand.Intn(l)
	pid := state.routees[r]
	pid.Request(message, sender)
}

func NewRandomPool(poolSize int) actor.PoolRouterConfig {
	r := &RandomPoolRouter{}
	r.PoolSize = poolSize
	return r
}

func NewRandomGroup(routees ...*actor.PID) actor.GroupRouterConfig {
	r := &RandomGroupRouter{}
	r.Routees = routees
	return r
}

func (config *RandomPoolRouter) CreateRouterState() actor.RouterState {
	return &RandomRouterState{}
}

func (config *RandomGroupRouter) CreateRouterState() actor.RouterState {
	return &RandomRouterState{}
}
