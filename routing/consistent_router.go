package routing

import (
	"errors"
	"log"

	"github.com/AsynkronIT/gam/actor"
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

type ConsistentRouterState struct {
	routees map[string]*actor.PID
	hasher  Hasher
	config  actor.RouterConfig
}

func NewConsistentRouter(config actor.RouterConfig, hasher Hasher) actor.RouterState {
	router := &ConsistentRouterState{
		config: config,
		hasher: hasher,
	}
	return router
}

func (state *ConsistentRouterState) SetRoutees(routees []*actor.PID) {
	nodes := make([]string, len(routees))
	routeesHash := make(map[string]*actor.PID)

	for i, r := range routees {
		routeesHash[r.Host] = r
		nodes[i] = r.Host
	}

	state.hasher.SetNodes(nodes)

	oldRoutees := state.routees

	state.routees = routeesHash

	for _, r := range oldRoutees {
		r.Stop()
	}
}

func (state *ConsistentRouterState) Route(message interface{}) {

	switch msg := message.(type) {
	case Hashable:
		node, err := state.hasher.GetNode(msg)
		if err != nil {
			log.Println("Consisten router failed to derminate node", err)
			return
		}
		if routee, ok := state.routees[node]; ok {
			routee.Tell(msg)
		} else {
			log.Println("Consisten router failed to resolve node", node)
		}
	default:
		log.Println("Unknown message", msg)
	}
}
