package router

import (
	"sync/atomic"

	"github.com/AsynkronIT/gam/actor"
)

type RoundRobinGroupRouter struct {
	routees []*actor.PID
}

type RoundRobinPoolRouter struct {
	poolSize int
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
	return &RoundRobinGroupRouter{routees: routees}
}

func NewRoundRobinPool(poolSize int) actor.PoolRouterConfig {
	return &RoundRobinPoolRouter{poolSize: poolSize}
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

func (config *RoundRobinPoolRouter) PoolRouter()   {}
func (config *RoundRobinGroupRouter) GroupRouter() {}

func roundRobinRoutee(index *int32, routees []*actor.PID) *actor.PID {
	i := int(atomic.AddInt32(index, 1))
	mod := len(routees)
	routee := routees[i%mod]
	return routee
}

func (config *RoundRobinGroupRouter) OnStarted(context actor.Context, props actor.Props, router actor.RouterState) {
	for _, r := range config.routees {
		context.Watch(r)
	}
	router.SetRoutees(config.routees)
}

func (config *RoundRobinPoolRouter) OnStarted(context actor.Context, props actor.Props, router actor.RouterState) {
	routees := make([]*actor.PID, config.poolSize)
	for i := 0; i < config.poolSize; i++ {
		pid := context.Spawn(props)
		routees[i] = pid
	}
	router.SetRoutees(routees)
}
