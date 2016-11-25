package actor

type RouterConfig interface {
	OnStarted(context Context, props Props, router RouterState)
	Create() RouterState
}

type GroupRouterConfig interface {
	RouterConfig
}

type PoolRouterConfig interface {
	RouterConfig
}

type GroupRouter struct {
	RouterConfig
	Routees []*PID
}

type PoolRouter struct {
	RouterConfig
	PoolSize int
}

func (config *GroupRouter) OnStarted(context Context, props Props, router RouterState) {
	for _, r := range config.Routees {
		context.Watch(r)
	}
	router.SetRoutees(config.Routees)
}

func (config *PoolRouter) OnStarted(context Context, props Props, router RouterState) {
	routees := make([]*PID, config.PoolSize)
	for i := 0; i < config.PoolSize; i++ {
		pid := context.Spawn(props)
		routees[i] = pid
	}
	router.SetRoutees(routees)
}

func spawnRouter(config RouterConfig, props Props, parent *PID) *PID {
	id := ProcessRegistry.getAutoId()
	routeeProps := props
	routeeProps.routerConfig = nil
	routerState := config.Create()

	routerProps := FromFunc(func(context Context) {
		switch context.Message().(type) {
		case *Started:
			config.OnStarted(context, routeeProps, routerState)
		}
	})
	router := spawn(id, routerProps, parent)

	ref := &RouterActorRef{
		router: router,
		state:  routerState,
	}
	proxyID := ProcessRegistry.getAutoId()
	proxy, _ := ProcessRegistry.add(ref, proxyID)
	return proxy
}

type RouterActorRef struct {
	router *PID
	state  RouterState
	ActorRef
}

func (ref *RouterActorRef) Tell(pid *PID, message interface{}) {
	ref.state.Route(message)
}

func (ref *RouterActorRef) SendSystemMessage(pid *PID, message SystemMessage) {
	r, _ := ProcessRegistry.get(ref.router)
	r.SendSystemMessage(pid, message)
}

func (ref *RouterActorRef) Stop(pid *PID) {
	ref.SendSystemMessage(pid, &Stop{})
}

type RouterState interface {
	Route(message interface{})
	SetRoutees(routees []*PID)
}
