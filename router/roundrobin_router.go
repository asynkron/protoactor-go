package router

import (
	"sync/atomic"

	"github.com/awevoke/protoactor-go/actor"
)

type roundRobinGroupRouter struct {
	GroupRouter
}

type roundRobinPoolRouter struct {
	PoolRouter
}

type roundRobinState struct {
	index   int32
	routees atomic.Pointer[actor.PIDSet]
	sender  actor.SenderContext
}

func (state *roundRobinState) SetSender(sender actor.SenderContext) {
	state.sender = sender
}

func (state *roundRobinState) SetRoutees(routees *actor.PIDSet) {
	state.routees.Store(routees)
}

func (state *roundRobinState) GetRoutees() *actor.PIDSet {
	r := state.routees.Load()
	if r == nil {
		return actor.NewPIDSet()
	}
	return r
}

func (state *roundRobinState) RouteMessage(message any) {
	r := state.routees.Load()
	if r == nil || r.Len() == 0 {
		return
	}
	pid := roundRobinRoutee(&state.index, r)
	state.sender.Send(pid, message)
}

func NewRoundRobinPool(size int, opts ...actor.PropsOption) *actor.Props {
	return (&actor.Props{}).
		Configure(actor.WithSpawnFunc(spawner(&roundRobinPoolRouter{PoolRouter{PoolSize: size}}))).
		Configure(opts...)
}

func NewRoundRobinGroup(routees ...*actor.PID) *actor.Props {
	return (&actor.Props{}).Configure(actor.WithSpawnFunc(spawner(&roundRobinGroupRouter{GroupRouter{Routees: actor.NewPIDSet(routees...)}})))
}

func (config *roundRobinPoolRouter) CreateRouterState() State {
	return &roundRobinState{}
}

func (config *roundRobinGroupRouter) CreateRouterState() State {
	return &roundRobinState{}
}

func roundRobinRoutee(index *int32, routees *actor.PIDSet) *actor.PID {
	if routees.Len() == 0 {
		return nil
	}
	i := int(atomic.AddInt32(index, 1))
	if i < 0 {
		*index = 0
		i = 0
	}
	mod := routees.Len()
	routee := routees.Get(i % mod)
	return routee
}
