package router

import (
	"sync"

	"github.com/AsynkronIT/protoactor-go/actor"
)

type RouterType int

const (
	GroupRouterType RouterType = iota
	PoolRouterType
)

type RouterConfig interface {
	RouterType() RouterType
	OnStarted(context actor.Context, props *actor.Props, state RouterState)
	CreateRouterState() RouterState
}

type GroupRouter struct {
	Routees *actor.PIDSet
}

type PoolRouter struct {
	PoolSize int
}

func (config *GroupRouter) OnStarted(context actor.Context, props *actor.Props, state RouterState) {
	config.Routees.ForEach(func(i int, pid actor.PID) {
		context.Watch(&pid)
	})
	state.SetRoutees(config.Routees)
}

func (config *GroupRouter) RouterType() RouterType {
	return GroupRouterType
}

func (config *PoolRouter) OnStarted(context actor.Context, props *actor.Props, state RouterState) {
	var routees actor.PIDSet
	for i := 0; i < config.PoolSize; i++ {
		routees.Add(context.Spawn(props))
	}
	state.SetRoutees(&routees)
}

func (config *PoolRouter) RouterType() RouterType {
	return PoolRouterType
}

func spawner(config RouterConfig) actor.SpawnFunc {
	return func(id string, props *actor.Props, parentContext actor.SpawnerContext) (*actor.PID, error) {
		return spawn(id, config, props, parentContext)
	}
}

func spawn(id string, config RouterConfig, props *actor.Props, parentContext actor.SpawnerContext) (*actor.PID, error) {
	ref := &process{}
	proxy, absent := actor.ProcessRegistry.Add(ref, id)
	if !absent {
		return proxy, actor.ErrNameExists
	}

	var pc = *props
	pc.WithSpawnFunc(nil)
	ref.state = config.CreateRouterState()

	if config.RouterType() == GroupRouterType {
		wg := &sync.WaitGroup{}
		wg.Add(1)
		ref.router, _ = actor.DefaultSpawner(id+"/router", actor.PropsFromProducer(func() actor.Actor {
			return &groupRouterActor{
				props:  &pc,
				config: config,
				state:  ref.state,
				wg:     wg,
			}
		}), parentContext)
		wg.Wait() // wait for routerActor to start
	} else {
		wg := &sync.WaitGroup{}
		wg.Add(1)
		ref.router, _ = actor.DefaultSpawner(id+"/router", actor.PropsFromProducer(func() actor.Actor {
			return &poolRouterActor{
				props:  &pc,
				config: config,
				state:  ref.state,
				wg:     wg,
			}
		}), parentContext)
		wg.Wait() // wait for routerActor to start
	}

	ref.parent = parentContext.Self()
	return proxy, nil
}
