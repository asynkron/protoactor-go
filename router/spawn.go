package router

import (
	"github.com/AsynkronIT/protoactor-go/actor"
)

func spawn(id string, config RouterConfig, props actor.Props, parent *actor.PID) *actor.PID {
	props = props.WithSpawn(nil)
	rs := config.CreateRouterState()

	ra := &routerActor{
		props:  props,
		config: config,
		state:  rs,
	}
	ra.wg.Add(1)
	rp := actor.FromInstance(ra)

	rid := actor.ProcessRegistry.NextId()
	router := actor.DefaultSpawner(rid, rp, parent)
	ra.wg.Wait() // wait for routerActor to start

	ref := &process{
		router: router,
		state:  rs,
	}
	proxy, _ := actor.ProcessRegistry.Add(ref, id)
	return proxy
}
