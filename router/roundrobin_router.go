package router

import (
	"sync/atomic"

	"github.com/AsynkronIT/protoactor-go/actor"
)

type roundRobinGroupRouter struct {
	GroupRouter
}

type roundRobinPoolRouter struct {
	PoolRouter
}

type roundRobinState struct {
	index   int32
	routees *actor.PIDSet
	values  []actor.PID
}

func (state *roundRobinState) SetRoutees(routees *actor.PIDSet) {
	state.routees = routees
	state.values = routees.Values()
}

func (state *roundRobinState) GetRoutees() *actor.PIDSet {
	return state.routees
}

func (state *roundRobinState) RouteMessage(message interface{}, sender *actor.PID) {
	pid := roundRobinRoutee(&state.index, state.values)
	pid.Request(message, sender)
}

func NewRoundRobinPool(size int) *actor.Props {
	return actor.FromSpawnFunc(spawner(&roundRobinPoolRouter{PoolRouter{PoolSize: size}}))
}

func NewRoundRobinGroup(routees ...*actor.PID) *actor.Props {
	return actor.FromSpawnFunc(spawner(&roundRobinGroupRouter{GroupRouter{Routees: actor.NewPIDSet(routees...)}}))
}

func (config *roundRobinPoolRouter) CreateRouterState() Interface {
	return &roundRobinState{}
}

func (config *roundRobinGroupRouter) CreateRouterState() Interface {
	return &roundRobinState{}
}

func roundRobinRoutee(index *int32, routees []actor.PID) actor.PID {
	i := int(atomic.AddInt32(index, 1))
	if i < 0 {
		*index = 0
		i = 0
	}
	mod := len(routees)
	routee := routees[i%mod]
	return routee
}
