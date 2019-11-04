package router

import (
	"log"
	"sync/atomic"
	"unsafe"

	"github.com/otherview/protoactor-go/actor"
	"github.com/serialx/hashring"
)

type Hasher interface {
	Hash() string
}

type consistentHashGroupRouter struct {
	GroupRouter
}

type consistentHashPoolRouter struct {
	PoolRouter
}

type hashmapContainer struct {
	hashring  *hashring.HashRing
	routeeMap map[string]*actor.PID
}
type consistentHashRouterState struct {
	hmc *hashmapContainer
}

func (state *consistentHashRouterState) SetRoutees(routees *actor.PIDSet) {
	// lookup from node name to PID
	hmc := hashmapContainer{}
	hmc.routeeMap = make(map[string]*actor.PID)
	nodes := make([]string, routees.Len())
	routees.ForEach(func(i int, pid actor.PID) {
		nodeName := pid.Address + "@" + pid.Id
		nodes[i] = nodeName
		hmc.routeeMap[nodeName] = &pid
	})
	// initialize hashring for mapping message keys to node names
	hmc.hashring = hashring.New(nodes)
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&state.hmc)), unsafe.Pointer(&hmc))
}

func (state *consistentHashRouterState) GetRoutees() *actor.PIDSet {
	var routees actor.PIDSet
	hmc := (*hashmapContainer)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&state.hmc))))
	for _, v := range hmc.routeeMap {
		routees.Add(v)
	}
	return &routees
}

func (state *consistentHashRouterState) RouteMessage(message interface{}) {
	_, uwpMsg, _ := actor.UnwrapEnvelope(message)
	switch msg := uwpMsg.(type) {
	case Hasher:
		key := msg.Hash()
		hmc := (*hashmapContainer)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&state.hmc))))

		node, ok := hmc.hashring.GetNode(key)
		if !ok {
			log.Printf("[ROUTING] Consistent has router failed to derminate routee: %v", key)
			return
		}
		if routee, ok := hmc.routeeMap[node]; ok {
			rootContext.Send(routee, message)
		} else {
			log.Println("[ROUTING] Consistent router failed to resolve node", node)
		}
	default:
		log.Println("[ROUTING] Message must implement router.Hasher", msg)
	}
}

func (state *consistentHashRouterState) InvokeRouterManagementMessage(msg ManagementMessage, sender *actor.PID) {

}

func NewConsistentHashPool(size int) *actor.Props {
	return (&actor.Props{}).WithSpawnFunc(spawner(&consistentHashPoolRouter{PoolRouter{PoolSize: size}}))
}

func NewConsistentHashGroup(routees ...*actor.PID) *actor.Props {
	return (&actor.Props{}).WithSpawnFunc(spawner(&consistentHashGroupRouter{GroupRouter{Routees: actor.NewPIDSet(routees...)}}))
}

func (config *consistentHashPoolRouter) CreateRouterState() RouterState {
	return &consistentHashRouterState{}
}

func (config *consistentHashGroupRouter) CreateRouterState() RouterState {
	return &consistentHashRouterState{}
}
