package router

import (
	"log/slog"
	"sync/atomic"

	"github.com/asynkron/protoactor-go/actor"
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
	hmc    atomic.Pointer[hashmapContainer]
	sender actor.SenderContext
}

func (state *consistentHashRouterState) SetSender(sender actor.SenderContext) {
	state.sender = sender
}

func (state *consistentHashRouterState) SetRoutees(routees *actor.PIDSet) {
	// lookup from node name to PID
	hmc := hashmapContainer{}
	hmc.routeeMap = make(map[string]*actor.PID)
	nodes := make([]string, routees.Len())
	routees.ForEach(func(i int, pid *actor.PID) {
		nodeName := pid.Address + "@" + pid.Id
		nodes[i] = nodeName
		hmc.routeeMap[nodeName] = pid
	})
	// initialize hashring for mapping message keys to node names
	hmc.hashring = hashring.New(nodes)
	state.hmc.Store(&hmc)
}

func (state *consistentHashRouterState) GetRoutees() *actor.PIDSet {
	var routees actor.PIDSet
	hmc := state.hmc.Load()
	if hmc == nil {
		return &routees
	}
	for _, v := range hmc.routeeMap {
		routees.Add(v)
	}
	return &routees
}

func (state *consistentHashRouterState) RouteMessage(message any) {
	_, uwpMsg, _ := actor.UnwrapEnvelope(message)
	switch msg := uwpMsg.(type) {
	case Hasher:
		key := msg.Hash()
		hmc := state.hmc.Load()
		if hmc == nil {
			return
		}

		node, ok := hmc.hashring.GetNode(key)
		if !ok {
			slog.Warn("consistent hash router failed to determine routee", slog.String("key", key))
			return
		}
		if routee, ok := hmc.routeeMap[node]; ok {
			state.sender.Send(routee, message)
		} else {
			slog.Warn("consistent hash router failed to resolve node", slog.String("node", node))
		}
	default:
		slog.Warn("message must implement router.Hasher", slog.Any("message", msg))
	}
}

func (state *consistentHashRouterState) InvokeRouterManagementMessage(ManagementMessage, *actor.PID) {
}

func NewConsistentHashPool(size int, opts ...actor.PropsOption) *actor.Props {
	return (&actor.Props{}).
		Configure(actor.WithSpawnFunc(spawner(&consistentHashPoolRouter{PoolRouter{PoolSize: size}}))).
		Configure(opts...)
}

func NewConsistentHashGroup(routees ...*actor.PID) *actor.Props {
	return (&actor.Props{}).Configure(actor.WithSpawnFunc(spawner(&consistentHashGroupRouter{GroupRouter{Routees: actor.NewPIDSet(routees...)}})))
}

func (config *consistentHashPoolRouter) CreateRouterState() State {
	return &consistentHashRouterState{}
}

func (config *consistentHashGroupRouter) CreateRouterState() State {
	return &consistentHashRouterState{}
}
