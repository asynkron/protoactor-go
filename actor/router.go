package actor

import "sync/atomic"

type RouterConfig interface {
	OnStarted(context Context, props Props, router RouterState)

	Create() RouterState
}

type RoundRobinGroupRouter struct {
	index   int32
	routees []*PID
}

type RoundRobinPoolRouter struct {
	index    int32
	poolSize int
}

type RouterState interface {
	Route(message interface{})
	SetRoutees(routees []*PID)
}

type RoundRobinState struct {
	index   int32
	routees []*PID
	config  RouterConfig
}

func (state *RoundRobinState) SetRoutees(routees []*PID) {
	state.routees = routees
}

func NewRoundRobinGroupRouter(routees ...*PID) RouterConfig {
	return &RoundRobinGroupRouter{routees: routees}
}

func NewRoundRobinPoolRouter(poolSize int) RouterConfig {
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

var NoRouter RouterConfig = nil

func spawnRouter(config RouterConfig, props Props, parent *PID) *PID {
	id := ProcessRegistry.getAutoId()
	routeeProps := props.WithRouter(NoRouter)
	routerState := config.Create()

	routerProps := FromFunc(func(context Context) {
		switch context.Message().(type) {
		case Started:
			config.OnStarted(context, routeeProps, routerState)
		}
	})
	router := spawn(id, routerProps, parent)
	ref := newRouterActorRef(router, routerState)
	proxyID := ProcessRegistry.getAutoId()
	proxy := ProcessRegistry.registerPID(ref, proxyID)
	return proxy
}

type RouterActorRef struct {
	router *PID
	state  RouterState
	ActorRef
}

func (ref *RouterActorRef) Tell(message interface{}) {
	ref.state.Route(message)
}

func newRouterActorRef(router *PID, state RouterState) ActorRef {
	return &RouterActorRef{
		router: router,
		state:  state,
	}
}
