package router

import (
	"sync/atomic"

	"github.com/asynkron/protoactor-go/actor"
)

type broadcastGroupRouter struct {
	GroupRouter
}

type broadcastPoolRouter struct {
	PoolRouter
}

type broadcastRouterState struct {
	routees atomic.Pointer[actor.PIDSet]
	sender  actor.SenderContext
}

func (state *broadcastRouterState) SetSender(sender actor.SenderContext) {
	state.sender = sender
}

func (state *broadcastRouterState) SetRoutees(routees *actor.PIDSet) {
	state.routees.Store(routees)
}

func (state *broadcastRouterState) GetRoutees() *actor.PIDSet {
	r := state.routees.Load()
	if r == nil {
		return actor.NewPIDSet()
	}
	return r
}

func (state *broadcastRouterState) RouteMessage(message any) {
	r := state.routees.Load()
	if r == nil {
		return
	}
	r.ForEach(func(_ int, pid *actor.PID) {
		state.sender.Send(pid, message)
	})
}

func NewBroadcastPool(size int, opts ...actor.PropsOption) *actor.Props {
	return (&actor.Props{}).
		Configure(actor.WithSpawnFunc(spawner(&broadcastPoolRouter{PoolRouter{PoolSize: size}}))).
		Configure(opts...)
}

func NewBroadcastGroup(routees ...*actor.PID) *actor.Props {
	return (&actor.Props{}).Configure(actor.WithSpawnFunc(spawner(&broadcastGroupRouter{GroupRouter{Routees: actor.NewPIDSet(routees...)}})))
}

func (config *broadcastPoolRouter) CreateRouterState() State {
	return &broadcastRouterState{}
}

func (config *broadcastGroupRouter) CreateRouterState() State {
	return &broadcastRouterState{}
}
