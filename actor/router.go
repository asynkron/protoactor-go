package actor

type RouterConfig interface {
	OnStarted(context Context, props Props, router RouterState)

	Create() RouterState
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

	ref := &RouterActorRef{
		router: router,
		state:  routerState,
	}
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
