package router

import (
	"math/rand"
	"sync/atomic"

	"github.com/asynkron/protoactor-go/actor"
)

type randomGroupRouter struct {
	GroupRouter
}

type randomPoolRouter struct {
	PoolRouter
}

type randomRouterState struct {
	routees atomic.Pointer[actor.PIDSet]
	sender  actor.SenderContext
}

func (state *randomRouterState) SetSender(sender actor.SenderContext) {
	state.sender = sender
}

func (state *randomRouterState) SetRoutees(routees *actor.PIDSet) {
	state.routees.Store(routees)
}

func (state *randomRouterState) GetRoutees() *actor.PIDSet {
	r := state.routees.Load()
	if r == nil {
		return actor.NewPIDSet()
	}
	return r
}

func (state *randomRouterState) RouteMessage(message any) {
	r := state.routees.Load()
	if r == nil || r.Len() == 0 {
		return
	}
	pid := randomRoutee(r)
	state.sender.Send(pid, message)
}

func NewRandomPool(size int, opts ...actor.PropsOption) *actor.Props {
	return (&actor.Props{}).
		Configure(actor.WithSpawnFunc(spawner(&randomPoolRouter{PoolRouter{PoolSize: size}}))).
		Configure(opts...)
}

func NewRandomGroup(routees ...*actor.PID) *actor.Props {
	return (&actor.Props{}).Configure(actor.WithSpawnFunc(spawner(&randomGroupRouter{GroupRouter{Routees: actor.NewPIDSet(routees...)}})))
}

func (config *randomPoolRouter) CreateRouterState() State {
	return &randomRouterState{}
}

func (config *randomGroupRouter) CreateRouterState() State {
	return &randomRouterState{}
}

func randomRoutee(routees *actor.PIDSet) *actor.PID {
	l := routees.Len()
	r := rand.Intn(l)
	pid := routees.Get(r)
	return pid
}
