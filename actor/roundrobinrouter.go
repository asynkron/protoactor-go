package actor

import "sync/atomic"

type RouterState interface {
	Route(message interface{})
	SetRoutees(routees []*PID)
}

type RoundRobinGroupRouter struct {
	routees []*PID
}

type RoundRobinPoolRouter struct {
	poolSize int
}

type RoundRobinState struct {
	index   int32
	routees []*PID
	config  RouterConfig
}

func (state *RoundRobinState) SetRoutees(routees []*PID) {
	state.routees = routees
}

func NewRoundRobinGroup(routees ...*PID) GroupRouterConfig {
	return &RoundRobinGroupRouter{routees: routees}
}

func NewRoundRobinPool(poolSize int) PoolRouterConfig {
	return &RoundRobinPoolRouter{poolSize: poolSize}
}

func (state *RoundRobinState) Route(message interface{}) {
	pid := roundRobinRoutee(&state.index, state.routees)
	pid.Tell(message)
}

func (config *RoundRobinPoolRouter) Create() RouterState {
	return &RoundRobinState{
		config: config,
	}
}

func (config *RoundRobinGroupRouter) Create() RouterState {
	return &RoundRobinState{
		config: config,
	}
}

func (config *RoundRobinPoolRouter) PoolRouter()   {}
func (config *RoundRobinGroupRouter) GroupRouter() {}

func roundRobinRoutee(index *int32, routees []*PID) *PID {
	i := int(atomic.AddInt32(index, 1))
	mod := len(routees)
	routee := routees[i%mod]
	return routee
}

func (config *RoundRobinGroupRouter) OnStarted(context Context, props Props, router RouterState) {
	for _, r := range config.routees {
		context.Watch(r)
	}
	router.SetRoutees(config.routees)
}

func (config *RoundRobinPoolRouter) OnStarted(context Context, props Props, router RouterState) {
	routees := make([]*PID, config.poolSize)
	for i := 0; i < config.poolSize; i++ {
		pid := context.Spawn(props)
		routees[i] = pid
	}
	router.SetRoutees(routees)
}
