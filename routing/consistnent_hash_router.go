package routing

import (
	"errors"
	"log"

	"github.com/AsynkronIT/gam/actor"
	"github.com/serialx/hashring"
)

var (
	ErrorUnknownPartition = errors.New("Hasher doesn't return partition")
)

type Hashable interface {
	HashBy() string
}

type Hasher interface {
	GetNode(message Hashable) (string, error)
	SetNodes(nodes []string)
}

type ConsistentHashGroupRouter struct {
	routees []*actor.PID
}

type ConsistentHashPoolRouter struct {
	poolSize int
}

type ConsistentHashRouterState struct {
	routees   []*actor.PID
	hashring  *hashring.HashRing
	routeeMap map[string]*actor.PID
	config    actor.RouterConfig
}

func (state *ConsistentHashRouterState) SetRoutees(routees []*actor.PID) {
	//add the PID's
	state.routees = routees
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

func NewConsistentHashPool(poolSize int) actor.PoolRouterConfig {
	return &ConsistentHashPoolRouter{poolSize: poolSize}
}

func NewConsistentHashGroup(routees ...*actor.PID) actor.GroupRouterConfig {
	return &ConsistentHashGroupRouter{routees: routees}
}

func (state *ConsistentHashRouterState) Route(message interface{}) {
	switch msg := message.(type) {
	case Hashable:
		key := msg.HashBy()

		node, ok := state.hashring.GetNode(key)
		if !ok {
			log.Printf("[ROUTING] Consistent has router failed to derminate routee: %v", key)
			return
		}
		if routee, ok := state.routeeMap[node]; ok {
			routee.Tell(msg)
		} else {
			log.Println("[ROUTING] Consisten router failed to resolve node", node)
		}
	default:
		log.Println("[ROUTING] Unknown message", msg)
	}
}

func (config *ConsistentHashPoolRouter) Create() actor.RouterState {
	return &ConsistentHashRouterState{
		config: config,
	}
}

func (config *ConsistentHashGroupRouter) Create() actor.RouterState {
	return &ConsistentHashRouterState{
		config: config,
	}
}

func (config *ConsistentHashPoolRouter) PoolRouter()   {}
func (config *ConsistentHashGroupRouter) GroupRouter() {}

func (config *ConsistentHashGroupRouter) OnStarted(context actor.Context, props actor.Props, router actor.RouterState) {
	for _, r := range config.routees {
		context.Watch(r)
	}
	router.SetRoutees(config.routees)
}

func (config *ConsistentHashPoolRouter) OnStarted(context actor.Context, props actor.Props, router actor.RouterState) {
	routees := make([]*actor.PID, config.poolSize)
	for i := 0; i < config.poolSize; i++ {
		pid := context.Spawn(props)
		routees[i] = pid
	}
	router.SetRoutees(routees)
}
