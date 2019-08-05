package router

import (
	"log"
	"math/rand"
	"sync/atomic"
	"unsafe"

	"github.com/AsynkronIT/protoactor-go/actor"
)

type randomGroupRouter struct {
	GroupRouter
}

type randomPoolRouter struct {
	PoolRouter
}

type randomRouterState struct {
	routees *actor.PIDSet
	values  *[]actor.PID
}

func (state *randomRouterState) SetRoutees(routees *actor.PIDSet) {
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&state.routees)), unsafe.Pointer(routees))
	values := routees.Values()
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&state.values)), unsafe.Pointer(&values))
}

func (state *randomRouterState) GetRoutees() *actor.PIDSet {
	return (*actor.PIDSet)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&state.routees))))
}

func (state *randomRouterState) RouteMessage(message interface{}) {
	values := (*[]actor.PID)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&state.values))))
	if len(*values) <= 0 {
		log.Println("[ROUTING]RandomRouter route message failed, empty routees")
		return
	}
	pid := randomRoutee(*values)
	rootContext.Send(&pid, message)
}

func NewRandomPool(size int) *actor.Props {
	return (&actor.Props{}).WithSpawnFunc(spawner(&randomPoolRouter{PoolRouter{PoolSize: size}}))
}

func NewRandomGroup(routees ...*actor.PID) *actor.Props {
	return (&actor.Props{}).WithSpawnFunc(spawner(&randomGroupRouter{GroupRouter{Routees: actor.NewPIDSet(routees...)}}))
}

func (config *randomPoolRouter) CreateRouterState() RouterState {
	return &randomRouterState{}
}

func (config *randomGroupRouter) CreateRouterState() RouterState {
	return &randomRouterState{}
}

func randomRoutee(routees []actor.PID) actor.PID {
	l := len(routees)
	r := rand.Intn(l)
	pid := routees[r]
	return pid
}
