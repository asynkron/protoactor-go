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
	config  actor.RouterConfig
}

func (state *RandomRouterState) SetRoutees(routees []*actor.PID) {
	state.routees = routees
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

func randomRoutee(routees []*actor.PID) *actor.PID {
	routee := routees[rand.Intn(len(routees))]
	return routee
}
