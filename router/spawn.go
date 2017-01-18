package router

import (
	"github.com/AsynkronIT/protoactor-go/actor"
)

func spawn(id string, config RouterConfig, props actor.Props, parent *actor.PID) *actor.PID {
	props = props.WithSpawn(nil)
	routerState := config.CreateRouterState()

	routerProps := actor.FromInstance(&routerActor{
		props:  props,
		config: config,
		state:  routerState,
	})

	routerID := actor.ProcessRegistry.NextId()
	router := actor.DefaultSpawner(routerID, routerProps, parent)

	ref := &process{
		router: router,
		state:  routerState,
	}
	proxy, _ := actor.ProcessRegistry.Add(ref, id)
	return proxy
}
