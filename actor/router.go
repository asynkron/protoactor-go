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
	Routees *PIDSet
}

type PoolRouter struct {
	RouterConfig
	PoolSize int
}

func (config *GroupRouter) OnStarted(context Context, props Props, router RouterState) {
	config.Routees.ForEach(func(i int, pid PID) {
		context.Watch(&pid)
	})
	router.SetRoutees(config.Routees)
}

func (config *PoolRouter) OnStarted(context Context, props Props, router RouterState) {
	var routees PIDSet
	for i := 0; i < config.PoolSize; i++ {
		routees.Add(context.Spawn(props))
	}
	router.SetRoutees(&routees)
}

func spawnRouter(id string, config RouterConfig, props Props, parent *PID) *PID {
	routeeProps := props
	routeeProps.routerConfig = nil
	routerState := config.CreateRouterState()

	routerProps := FromInstance(&routerActor{
		props:  routeeProps,
		config: config,
		state:  routerState,
	})

	routerId := ProcessRegistry.getAutoId()
	router := spawn(routerId, routerProps, parent)

	ref := &RouterActorRef{
		router: router,
		state:  routerState,
	}
	proxy, _ := ProcessRegistry.add(ref, id)
	return proxy
}

type RouterActorRef struct {
	router *PID
	state  RouterState
}

func (ref *RouterActorRef) SendUserMessage(pid *PID, message interface{}, sender *PID) {
	if _, ok := message.(RouterManagementMessage); ok {
		r, _ := ProcessRegistry.get(ref.router)
		r.SendUserMessage(pid, message, sender)
	} else {
		ref.state.RouteMessage(message, sender)
	}
}

func (ref *RouterActorRef) Watch(pid *PID) {
	ref.SendSystemMessage(pid, &Watch{Watcher: pid})
}

func (ref *RouterActorRef) Unwatch(pid *PID) {
	ref.SendSystemMessage(pid, &Unwatch{Watcher: pid})
}

func (ref *RouterActorRef) SendSystemMessage(pid *PID, message SystemMessage) {
	r, _ := ProcessRegistry.get(ref.router)
	r.SendSystemMessage(pid, message)
}

func (ref *RouterActorRef) Stop(pid *PID) {
	ref.SendSystemMessage(pid, stopMessage)
}

type RouterState interface {
	RouteMessage(message interface{}, sender *PID)
	SetRoutees(routees *PIDSet)
	GetRoutees() *PIDSet
}
