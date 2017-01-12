package routing

import (
	"sync/atomic"

	"github.com/AsynkronIT/protoactor-go/actor"
)

type RoundRobinGroupRouter struct {
	GroupRouter
}

type RoundRobinPoolRouter struct {
	PoolRouter
}

type RoundRobinState struct {
	index   int32
	routees *actor.PIDSet
	values  []actor.PID
}

func (state *RoundRobinState) SetRoutees(routees *actor.PIDSet) {
	state.routees = routees
	state.values = routees.Values()
}

func (state *RoundRobinState) GetRoutees() *actor.PIDSet {
	return state.routees
}

func (state *RoundRobinState) RouteMessage(message interface{}, sender *actor.PID) {
	pid := roundRobinRoutee(&state.index, state.values)
	pid.Request(message, sender)
}

func NewRoundRobinGroup(routees ...*actor.PID) GroupRouterConfig {
	r := &RoundRobinGroupRouter{}
	r.Routees = actor.NewPIDSet(routees...)
	return r
}

func NewRoundRobinPool(poolSize int) PoolRouterConfig {
	r := &RoundRobinPoolRouter{}
	r.PoolSize = poolSize
	return r
}

func (config *RoundRobinPoolRouter) CreateRouterState() RouterState {
	return &RoundRobinState{}
}

func (config *RoundRobinGroupRouter) CreateRouterState() RouterState {
	return &RoundRobinState{}
}

func roundRobinRoutee(index *int32, routees []actor.PID) actor.PID {
	i := int(atomic.AddInt32(index, 1))
	mod := len(routees)
	routee := routees[i%mod]
	return routee
}
