package actor

type RouterConfig interface {
	OnStarted(context Context, props Props, router RouterState)
	Create() RouterState
}

type GroupRouterConfig interface {
	RouterConfig
	GroupRouter()
}

type PoolRouterConfig interface {
	RouterConfig
	PoolRouter()
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
