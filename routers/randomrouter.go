package routers

import (
	"math/rand"

	"github.com/AsynkronIT/gam/actor"
)

type RandomGroupRouter struct {
	routees []*actor.PID
}

type RandomPoolRouter struct {
	poolSize int
}

type RandomRouterState struct {
	routees []*actor.PID
	config  actor.RouterConfig
}

func (state *RandomRouterState) SetRoutees(routees []*actor.PID) {
	state.routees = routees
}

func NewRandomPool(poolSize int) actor.PoolRouterConfig {
	return &RandomPoolRouter{poolSize: poolSize}
}

func NewRandomGroup(routees ...*actor.PID) actor.GroupRouterConfig {
	return &RandomGroupRouter{routees: routees}
}

func (state *RandomRouterState) Route(message interface{}) {
	pid := randomRoutee(state.routees)
	pid.Tell(message)
}

func (config *RandomPoolRouter) Create() actor.RouterState {
	return &RandomRouterState{
		config: config,
	}
}

func (config *RandomGroupRouter) Create() actor.RouterState {
	return &RandomRouterState{
		config: config,
	}
}

func (config *RandomPoolRouter) PoolRouter()   {}
func (config *RandomGroupRouter) GroupRouter() {}

func randomRoutee(routees []*actor.PID) *actor.PID {
	routee := routees[rand.Intn(len(routees))]
	return routee
}

func (config *RandomGroupRouter) OnStarted(context actor.Context, props actor.Props, router actor.RouterState) {
	for _, r := range config.routees {
		context.Watch(r)
	}
	router.SetRoutees(config.routees)
}

func (config *RandomPoolRouter) OnStarted(context actor.Context, props actor.Props, router actor.RouterState) {
	routees := make([]*actor.PID, config.poolSize)
	for i := 0; i < config.poolSize; i++ {
		pid := context.Spawn(props)
		routees[i] = pid
	}
	router.SetRoutees(routees)
}
