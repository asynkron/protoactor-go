package actor

import (
	"errors"
	"log"
)

var (
	ErrorUnknownPartition = errors.New("Hasher doesn't return partition")
)

type Hashable interface {
	HashBy() string
}

type Hasher interface {
	Hash(message Hashable) (string, error)
	SetNodes(nodes []string)
}

type ConsistentRouterState struct {
	routees map[string]*PID
	hasher  Hasher
	config  RouterConfig
}

func NewConsistentRouter(config RouterConfig) *ConsistentRouterState {
	router := &ConsistentRouterState{
		config: config,
		hasher: NewHashring(),
	}
	return router
}

func (state *ConsistentRouterState) WithHasher(hasher Hasher) RouterState {
	state.hasher = hasher
	return state
}

func (state *ConsistentRouterState) ToRouter() RouterState {
	return state
}

func (state *ConsistentRouterState) SetRoutees(routees []*PID) {
	nodes := make([]string, len(routees))
	routeesHash := make(map[string]*PID)

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
		node, err := state.hasher.Hash(msg)
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
