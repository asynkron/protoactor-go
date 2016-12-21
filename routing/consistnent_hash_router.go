package routing

import (
	"log"

	"github.com/AsynkronIT/gam/actor"
	"github.com/serialx/hashring"
)

type Hashable interface {
	HashBy() string
}

type ConsistentHashGroupRouter struct {
	actor.GroupRouter
}

type ConsistentHashPoolRouter struct {
	actor.PoolRouter
}

type ConsistentHashRouterState struct {
	hashring  *hashring.HashRing
	routeeMap map[string]*actor.PID
}

func (state *ConsistentHashRouterState) SetRoutees(routees []*actor.PID) {
	//lookup from node name to PID
	state.routeeMap = make(map[string]*actor.PID)
	nodes := make([]string, len(routees))
	for i, pid := range routees {
		nodeName := pid.Host + "@" + pid.Id
		nodes[i] = nodeName
		state.routeeMap[nodeName] = pid
	}
	//initialize hashring for mapping message keys to node names
	state.hashring = hashring.New(nodes)
}

func (state *ConsistentHashRouterState) GetRoutees() []*actor.PID {
	var routees []*actor.PID
	for _, v := range state.routeeMap {
		routees = append(routees, v)
	}
	return routees
}

func (state *ConsistentHashRouterState) RouteMessage(message interface{}, sender *actor.PID) {
	switch msg := message.(type) {
	case Hashable:
		key := msg.HashBy()

		node, ok := state.hashring.GetNode(key)
		if !ok {
			log.Printf("[ROUTING] Consistent has router failed to derminate routee: %v", key)
			return
		}
		if routee, ok := state.routeeMap[node]; ok {
			routee.Request(msg, sender)
		} else {
			log.Println("[ROUTING] Consisten router failed to resolve node", node)
		}
	default:
		log.Println("[ROUTING] Unknown message", msg)
	}
}

func (state *ConsistentHashRouterState) InvokeRouterManagementMessage(msg actor.RouterManagementMessage, sender *actor.PID) {

}


func NewConsistentHashPool(poolSize int) actor.PoolRouterConfig {
	r := &ConsistentHashPoolRouter{}
	r.PoolSize = poolSize
	return r
}

func NewConsistentHashGroup(routees ...*actor.PID) actor.GroupRouterConfig {
	r := &ConsistentHashGroupRouter{}
	r.Routees = routees
	return r
}

func (config *ConsistentHashPoolRouter) CreateRouterState() actor.RouterState {
	return &ConsistentHashRouterState{}
}

func (config *ConsistentHashGroupRouter) CreateRouterState() actor.RouterState {
	return &ConsistentHashRouterState{}
}
