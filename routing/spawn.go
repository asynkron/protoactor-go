package routing

import "github.com/AsynkronIT/protoactor-go/actor"

// SpawnPool spawns a pool router with an auto generated id
func SpawnPool(config PoolRouterConfig, props actor.Props) *actor.PID {
	id := actor.ProcessRegistry.NextId()
	pid := spawn(id, config, props, nil)
	return pid
}

// SpawnGroup spawns a pool router with an auto generated id
func SpawnGroup(config GroupRouterConfig) *actor.PID {
	id := actor.ProcessRegistry.NextId()
	pid := spawn(id, config, actor.Props{}, nil)
	return pid
}

// SpawnNamedPool spawns a named actor
func SpawnNamedPool(config RouterConfig, props actor.Props, name string) *actor.PID {
	pid := spawn(name, config, props, nil)
	return pid
}

// SpawnNamedPool spawns a named actor
func SpawnNamedGroup(config RouterConfig, name string) *actor.PID {
	pid := spawn(name, config, actor.Props{}, nil)
	return pid
}

func spawn(id string, config RouterConfig, props actor.Props, parent *actor.PID) *actor.PID {
	routerState := config.CreateRouterState()

	routerProps := actor.FromInstance(&routerActor{
		props:  props,
		config: config,
		state:  routerState,
	})

	routerId := actor.ProcessRegistry.NextId()
	router := actor.DefaultSpawner(routerId, routerProps, parent)

	ref := &routerProcess{
		router: router,
		state:  routerState,
	}
	proxy, _ := actor.ProcessRegistry.Add(ref, id)
	return proxy
}
