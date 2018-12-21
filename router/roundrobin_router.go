package router

import (
	"sync/atomic"
	"unsafe"

	"github.com/AsynkronIT/protoactor-go/actor"
)

type roundRobinGroupRouter struct {
	GroupRouter
}

type roundRobinPoolRouter struct {
	PoolRouter
}

type roundRobinState struct {
	index   int32
	routees *actor.PIDSet
	values  *[]actor.PID
}

func (state *roundRobinState) SetRoutees(routees *actor.PIDSet) {
	state.routees = routees
	values := routees.Values()
	atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&state.values)), unsafe.Pointer(&values))
}

func (state *roundRobinState) GetRoutees() *actor.PIDSet {
	return state.routees
}

func (state *roundRobinState) RouteMessage(message interface{}) {
	pid := roundRobinRoutee(&state.index, *state.values)
	rootContext.Send(&pid, message)
}

func NewRoundRobinPool(size int) *actor.Props {
	return (&actor.Props{}).WithSpawnFunc(spawner(&roundRobinPoolRouter{PoolRouter{PoolSize: size}}))
}

func NewRoundRobinGroup(routees ...*actor.PID) *actor.Props {
	return (&actor.Props{}).WithSpawnFunc(spawner(&roundRobinGroupRouter{GroupRouter{Routees: actor.NewPIDSet(routees...)}}))
}

func (config *roundRobinPoolRouter) CreateRouterState() RouterState {
	return &roundRobinState{}
}

func (config *roundRobinGroupRouter) CreateRouterState() RouterState {
	return &roundRobinState{}
}

func roundRobinRoutee(index *int32, routees []actor.PID) actor.PID {
	i := int(atomic.AddInt32(index, 1))
	if i < 0 {
		*index = 0
		i = 0
	}
	mod := len(routees)
	routee := routees[i%mod]
	return routee
}
