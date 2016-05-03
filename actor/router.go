package actor

import "sync/atomic"

type RouterConfig interface {
	OnStarted(context Context, props Props)
	Route(message interface{})
}

type RoundRobinGroupRouter struct {
	index   int32
	routees []*PID
}

type RoundRobinPoolRouter struct {
	index    int32
	poolSize int
	routees  []*PID
}

func NewRoundRobinGroupRouter(routees ...*PID) RouterConfig {
	return &RoundRobinGroupRouter{routees: routees}
}

func NewRoundRobinPoolRouter(poolSize int) RouterConfig {
	return &RoundRobinPoolRouter{poolSize: poolSize}
}

func (config *RoundRobinPoolRouter) Route(message interface{}) {
	pid := roundRobinRoutee(&config.index, config.routees)
	pid.Tell(message)
}

func (config *RoundRobinGroupRouter) Route(message interface{}) {
	pid := roundRobinRoutee(&config.index, config.routees)
	pid.Tell(message)
}

func roundRobinRoutee(index *int32, routees []*PID) *PID {
	i := int(atomic.AddInt32(index, 1))
	mod := len(routees)
	routee := routees[i%mod]
	return routee
}

func (config *RoundRobinGroupRouter) OnStarted(context Context, props Props) {
	for _, r := range config.routees {
		context.Watch(r)
	}
}

func (config *RoundRobinPoolRouter) OnStarted(context Context, props Props) {
	config.routees = make([]*PID, config.poolSize)
	for i := 0; i < config.poolSize; i++ {
		pid := context.Spawn(props)
		config.routees[i] = pid
	}
}

var NoRouter RouterConfig = nil

func spawnRouter(config RouterConfig, props Props, parent *PID) *PID {
	id := ProcessRegistry.getAutoId()
	routeeProps := props.WithRouter(NoRouter)
	routerProps := FromFunc(func(context Context) {
		switch context.Message().(type) {
		case Started:
			config.OnStarted(context, routeeProps)
		}
	})
	router := spawn(id, routerProps, parent)
	ref := newRouterActorRef(router, config)
	proxyID := ProcessRegistry.getAutoId()
	proxy := ProcessRegistry.registerPID(ref, proxyID)
	return proxy
}

type RouterActorRef struct {
	router *PID
	config RouterConfig
	ActorRef
}

func (ref *RouterActorRef) Tell(message interface{}) {
	ref.config.Route(message)
}

func newRouterActorRef(router *PID, config RouterConfig) ActorRef {
	return &RouterActorRef{
		router: router,
		config: config,
	}
}
