package actor

import "log"

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
		log.Println("got message")
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
	proxy, _ := ProcessRegistry.registerPID(ref, proxyID)
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

func (ref *RouterActorRef) SendSystemMessage(message SystemMessage) {
	r, _ := ProcessRegistry.fromPID(ref.router)
	r.SendSystemMessage(message)
}

func (ref *RouterActorRef) Stop() {
	ref.SendSystemMessage(&stop{})
}
