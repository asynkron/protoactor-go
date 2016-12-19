package actor

type RouterConfig interface {
	OnStarted(context Context, props Props, router RouterState)
	CreateRouterState() RouterState
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

func spawnRouter(id string, config RouterConfig, props Props, parent *PID) *PID {
	routeeProps := props
	routeeProps.routerConfig = nil
	routerState := config.CreateRouterState()

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
}

func (ref *RouterActorRef) SendUserMessage(pid *PID, message interface{}, sender *PID) {
	ref.state.RouteMessage(message, sender)
}

func (ref *RouterActorRef) Watch(pid *PID) {
	ref.SendSystemMessage(pid, &Watch{Watcher: pid})
}

func (ref *RouterActorRef) UnWatch(pid *PID) {
	ref.SendSystemMessage(pid, &Unwatch{Watcher: pid})
}

func (ref *RouterActorRef) SendSystemMessage(pid *PID, message SystemMessage) {
	r, _ := ProcessRegistry.get(ref.router)
	r.SendSystemMessage(pid, message)
}

func (ref *RouterActorRef) Stop(pid *PID) {
	ref.SendSystemMessage(pid, &Stop{})
}

type RouterState interface {
	RouteMessage(message interface{}, sender *PID)
	SetRoutees(routees []*PID)
}
