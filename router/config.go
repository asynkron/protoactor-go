package router

import (
	"github.com/AsynkronIT/protoactor-go/actor"
)

type RouterConfig interface {
	OnStarted(context actor.Context, props actor.Props, router Interface)
	CreateRouterState() Interface
}

type GroupRouter struct {
	Routees *actor.PIDSet
}

type PoolRouter struct {
	PoolSize int
}

func (config *GroupRouter) OnStarted(context actor.Context, props actor.Props, router Interface) {
	config.Routees.ForEach(func(i int, pid actor.PID) {
		context.Watch(&pid)
	})
	router.SetRoutees(config.Routees)
}

func (config *PoolRouter) OnStarted(context actor.Context, props actor.Props, router Interface) {
	var routees actor.PIDSet
	for i := 0; i < config.PoolSize; i++ {
		routees.Add(context.Spawn(props))
	}
	router.SetRoutees(&routees)
}

func spawner(config RouterConfig) actor.Spawner {
	return func(id string, props actor.Props, parent *actor.PID) *actor.PID {
		return spawn(id, config, props, parent)
	}
}
