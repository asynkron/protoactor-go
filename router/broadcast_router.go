package router

import (
	"sync/atomic"
	"unsafe"

	"github.com/AsynkronIT/protoactor-go/actor"
)

type broadcastGroupRouter struct {
	GroupRouter
}

type broadcastPoolRouter struct {
	PoolRouter
}

type broadcastRouterState struct {
	routees *actor.PIDSet
}

func (state *broadcastRouterState) SetRoutees(routees *actor.PIDSet) {
	rts := *routees
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&state.routees)), unsafe.Pointer(&rts))
}

func (state *broadcastRouterState) GetRoutees() *actor.PIDSet {
	return state.routees.Clone()
}

func (state *broadcastRouterState) RouteMessage(message interface{}) {
	state.routees.ForEach(func(i int, pid actor.PID) {
		pid.Tell(message)
	})
}

func NewBroadcastPool(size int) *actor.Props {
	return actor.FromSpawnFunc(spawner(&broadcastPoolRouter{PoolRouter{PoolSize: size}}))
}

func NewBroadcastGroup(routees ...*actor.PID) *actor.Props {
	return actor.FromSpawnFunc(spawner(&broadcastGroupRouter{GroupRouter{Routees: actor.NewPIDSet(routees...)}}))
}

func (config *broadcastPoolRouter) CreateRouterState() Interface {
	return &broadcastRouterState{}
}

func (config *broadcastGroupRouter) CreateRouterState() Interface {
	return &broadcastRouterState{}
}
