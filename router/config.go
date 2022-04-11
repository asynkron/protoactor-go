package router

import (
	"sync"

	"github.com/asynkron/protoactor-go/actor"
)

type RouterType int

const (
	GroupRouterType RouterType = iota
	PoolRouterType
)

type RouterConfig interface {
	RouterType() RouterType
	OnStarted(context actor.Context, props *actor.Props, state State)
	CreateRouterState() State
}

type GroupRouter struct {
	Routees *actor.PIDSet
}

type PoolRouter struct {
	PoolSize int
}

func (config *GroupRouter) OnStarted(context actor.Context, props *actor.Props, state State) {
	config.Routees.ForEach(func(i int, pid *actor.PID) {
		context.Watch(pid)
	})
	state.SetSender(context)
	state.SetRoutees(config.Routees)
}

func (config *GroupRouter) RouterType() RouterType {
	return GroupRouterType
}

func (config *PoolRouter) OnStarted(context actor.Context, props *actor.Props, state State) {
	var routees actor.PIDSet
	for i := 0; i < config.PoolSize; i++ {
		routees.Add(context.Spawn(props))
	}
	state.SetSender(context)
	state.SetRoutees(&routees)
}

func (config *PoolRouter) RouterType() RouterType {
	return PoolRouterType
}

func spawner(config RouterConfig) actor.SpawnFunc {
	return func(actorSystem *actor.ActorSystem, id string, props *actor.Props, parentContext actor.SpawnerContext) (*actor.PID, error) {
		return spawn(actorSystem, id, config, props, parentContext)
	}
}

func spawn(actorSystem *actor.ActorSystem, id string, config RouterConfig, props *actor.Props, parentContext actor.SpawnerContext) (*actor.PID, error) {
	ref := &process{
		actorSystem: actorSystem,
	}
	proxy, absent := actorSystem.ProcessRegistry.Add(ref, id)
	if !absent {
		return proxy, actor.ErrNameExists
	}

	pc := *props
	pc.Configure(actor.WithSpawnFunc(nil))
	ref.state = config.CreateRouterState()

	if config.RouterType() == GroupRouterType {
		wg := &sync.WaitGroup{}
		wg.Add(1)
		ref.router, _ = actor.DefaultSpawner(actorSystem, id+"/router", actor.PropsFromProducer(func() actor.Actor {
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
		ref.router, _ = actor.DefaultSpawner(actorSystem, id+"/router", actor.PropsFromProducer(func() actor.Actor {
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
