package router

import (
	"github.com/AsynkronIT/protoactor-go/actor"
)

func spawn(id string, config RouterConfig, props *actor.Props, parent *actor.PID) (*actor.PID, error) {
	var pc = *props
	pc.WithSpawnFunc(nil)
	rs := config.CreateRouterState()

	ra := &routerActor{
		props:  &pc,
		config: config,
		state:  rs,
	}
	ra.wg.Add(1)
	rp := actor.FromInstance(ra)

	rid := actor.ProcessRegistry.NextId()
	router, _ := actor.DefaultSpawner(rid, rp, parent)
	ra.wg.Wait() // wait for routerActor to start

	ref := &process{
		router: router,
		state:  rs,
	}
	proxy, absent := actor.ProcessRegistry.Add(ref, id)
	if !absent {
		return proxy, actor.ErrNameExists
	}

	return proxy, nil
}
