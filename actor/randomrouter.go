package actor

import "math/rand"

type RandomGroupRouter struct {
	routees []*PID
}

type RandomPoolRouter struct {
	poolSize int
}

type RandomRouterState struct {
	routees []*PID
	config  RouterConfig
}

func (state *RandomRouterState) SetRoutees(routees []*PID) {
	state.routees = routees
}

func NewRandomPool(poolSize int) PoolRouterConfig {
	return &RandomPoolRouter{poolSize: poolSize}
}

func NewRandomGroup(routees ...*PID) GroupRouterConfig {
	return &RandomGroupRouter{routees: routees}
}

func (state *RandomRouterState) Route(message interface{}) {
	pid := randomRoutee(state.routees)
	pid.Tell(message)
}

func (config *RandomPoolRouter) Create() RouterState {
	return &RandomRouterState{
		config: config,
	}
}

func (config *RandomGroupRouter) Create() RouterState {
	return &RandomRouterState{
		config: config,
	}
}

func (config *RandomPoolRouter) PoolRouter()   {}
func (config *RandomGroupRouter) GroupRouter() {}

func randomRoutee(routees []*PID) *PID {
	routee := routees[rand.Intn(len(routees))]
	return routee
}

func (config *RandomGroupRouter) OnStarted(context Context, props Props, router RouterState) {
	for _, r := range config.routees {
		context.Watch(r)
	}
	router.SetRoutees(config.routees)
}

func (config *RandomPoolRouter) OnStarted(context Context, props Props, router RouterState) {
	routees := make([]*PID, config.poolSize)
	for i := 0; i < config.poolSize; i++ {
		pid := context.Spawn(props)
		routees[i] = pid
	}
	router.SetRoutees(routees)
}
