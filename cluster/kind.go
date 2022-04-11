package cluster

import (
	"sync/atomic"

	"github.com/asynkron/protoactor-go/actor"
)

// Kind represents the kinds of actors a cluster can manage
type Kind struct {
	Kind            string
	Props           *actor.Props
	StrategyBuilder func(*Cluster) MemberStrategy
}

// NewKind creates a new instance of a kind
func NewKind(kind string, props *actor.Props) *Kind {
	// add cluster middleware
	p := props.Clone(withClusterReceiveMiddleware())
	return &Kind{
		Kind:            kind,
		Props:           p,
		StrategyBuilder: nil,
	}
}

func (k *Kind) WithMemberStrategy(strategyBuilder func(*Cluster) MemberStrategy) {
	k.StrategyBuilder = strategyBuilder
}

func (k *Kind) Build(cluster *Cluster) *ActivatedKind {
	var strategy MemberStrategy = nil
	if k.StrategyBuilder != nil {
		strategy = k.StrategyBuilder(cluster)
	}

	return &ActivatedKind{
		Kind:     k.Kind,
		Props:    k.Props,
		Strategy: strategy,
	}
}

type ActivatedKind struct {
	Kind     string
	Props    *actor.Props
	Strategy MemberStrategy
	count    int32
}

func (ak *ActivatedKind) Inc() {
	atomic.AddInt32(&ak.count, 1)
}

func (ak *ActivatedKind) Dev() {
	atomic.AddInt32(&ak.count, -1)
}
