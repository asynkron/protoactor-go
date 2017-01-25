package router

import (
	"github.com/AsynkronIT/protoactor-go/actor"
)

func spawn(id string, config RouterConfig, props *actor.Props, parent *actor.PID) (*actor.PID, error) {
	ref := &process{}
	proxy, absent := actor.ProcessRegistry.Add(ref, id)
	if !absent {
		return proxy, actor.ErrNameExists
	}

	var pc = *props
	pc.WithSpawnFunc(nil)
	ref.state = config.CreateRouterState()

	if config.RouterType() == GroupRouterType {
		ra := &groupRouterActor{
			props:  &pc,
			config: config,
			state:  ref.state,
		}
		ra.wg.Add(1)
		ref.router, _ = actor.DefaultSpawner(id+"/router", actor.FromInstance(ra), parent)
		ra.wg.Wait() // wait for routerActor to start
	} else {
		ra := &poolRouterActor{
			props:  &pc,
			config: config,
			state:  ref.state,
		}
		ra.wg.Add(1)
		ref.router, _ = actor.DefaultSpawner(id+"/router", actor.FromInstance(ra), parent)
		ra.wg.Wait() // wait for routerActor to start
	}

	return proxy, nil
}
