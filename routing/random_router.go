package routing

import (
	"math/rand"

	"github.com/AsynkronIT/protoactor-go/actor"
)

type RandomGroupRouter struct {
	GroupRouter
}

type RandomPoolRouter struct {
	PoolRouter
}

type RandomRouterState struct {
	routees *actor.PIDSet
	values  []actor.PID
}

func (state *RandomRouterState) SetRoutees(routees *actor.PIDSet) {
	state.routees = routees
	state.values = routees.Values()
}

func (state *RandomRouterState) GetRoutees() *actor.PIDSet {
	return state.routees
}

func (state *RandomRouterState) RouteMessage(message interface{}, sender *actor.PID) {
	l := len(state.values)
	r := rand.Intn(l)
	pid := state.values[r]
	pid.Request(message, sender)
}

func NewRandomPool(poolSize int) PoolRouterConfig {
	r := &RandomPoolRouter{}
	r.PoolSize = poolSize
	return r
}

func NewRandomGroup(routees ...*actor.PID) GroupRouterConfig {
	r := &RandomGroupRouter{}
	r.Routees = actor.NewPIDSet(routees...)
	return r
}

func (config *RandomPoolRouter) CreateRouterState() RouterState {
	return &RandomRouterState{}
}

func (config *RandomGroupRouter) CreateRouterState() RouterState {
	return &RandomRouterState{}
}
