package routing

import (
	"sync/atomic"

	"github.com/AsynkronIT/gam/actor"
)

type RoundRobinGroupRouter struct {
	actor.GroupRouter
}

type RoundRobinPoolRouter struct {
	actor.PoolRouter
}

type RoundRobinState struct {
	index   int32
	routees []*actor.PID
	config  actor.RouterConfig
}

func (state *RoundRobinState) SetRoutees(routees []*actor.PID) {
	state.routees = routees
}

func NewRoundRobinGroup(routees ...*actor.PID) actor.GroupRouterConfig {
	r := &RoundRobinGroupRouter{}
	r.Routees = routees
	return r
}

func NewRoundRobinPool(poolSize int) actor.PoolRouterConfig {
	r := &RoundRobinPoolRouter{}
	r.PoolSize = poolSize
	return r
}

func (state *RoundRobinState) Route(message interface{}) {
	pid := roundRobinRoutee(&state.index, state.routees)
	pid.Tell(message)
}

func (config *RoundRobinPoolRouter) Create() actor.RouterState {
	return &RoundRobinState{
		config: config,
	}
}

func (config *RoundRobinGroupRouter) Create() actor.RouterState {
	return &RoundRobinState{
		config: config,
	}
}

func roundRobinRoutee(index *int32, routees []*actor.PID) *actor.PID {
	i := int(atomic.AddInt32(index, 1))
	mod := len(routees)
	routee := routees[i%mod]
	return routee
}
