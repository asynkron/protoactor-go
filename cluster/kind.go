package cluster

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/awevoke/protoactor-go/actor"
)

// ValidateKindName checks that a Kind name is acceptable for use in a cluster.
//
// The kind name appears as the prefix (before the first '/') of every
// ClusterIdentity key — see ClusterIdentity.AsKey, which formats keys as
// "kind/identity". To keep that boundary unambiguous when the identity itself
// contains '/', kinds must not contain '/'. Identities are still free to
// contain '/' and round-trip correctly because key parsing splits on the
// first '/'.
//
// An empty kind name is also rejected because it produces a leading-'/'
// key that no parser can disambiguate from "kind=, identity=foo".
func ValidateKindName(name string) error {
	if name == "" {
		return fmt.Errorf("kind name must not be empty")
	}
	if strings.Contains(name, "/") {
		return fmt.Errorf("kind name %q must not contain '/' (reserved as kind/identity separator)", name)
	}
	return nil
}

// Kind represents the kinds of actors a cluster can manage
type Kind struct {
	Kind                     string
	Props                    *actor.Props
	StrategyBuilder          func(*Cluster) MemberStrategy
	CanSpawnIdentity         func(ctx context.Context, identity string) (bool, error)
	ActivatorStrategyBuilder func(*Cluster) ActivatorStrategy
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

func (k *Kind) WithMemberStrategy(strategyBuilder func(*Cluster) MemberStrategy) *Kind {
	k.StrategyBuilder = strategyBuilder
	return k
}

// WithCanSpawnIdentity sets an optional predicate that is called before
// spawning a grain. If it returns false, the activation is rejected with
// InvalidIdentity. The context allows async implementations (e.g., DB lookups).
func (k *Kind) WithCanSpawnIdentity(fn func(ctx context.Context, identity string) (bool, error)) *Kind {
	k.CanSpawnIdentity = fn
	return k
}

// WithActivatorStrategy sets the placement strategy builder for this kind.
// The builder receives the Cluster and returns an ActivatorStrategy that
// selects which member should host new grain activations for this kind.
func (k *Kind) WithActivatorStrategy(builder func(*Cluster) ActivatorStrategy) *Kind {
	k.ActivatorStrategyBuilder = builder
	return k
}

func (k *Kind) Build(cluster *Cluster) *ActivatedKind {
	var strategy MemberStrategy = nil
	if k.StrategyBuilder != nil {
		strategy = k.StrategyBuilder(cluster)
	}

	var activatorStrategy ActivatorStrategy = nil
	if k.ActivatorStrategyBuilder != nil && cluster != nil {
		activatorStrategy = k.ActivatorStrategyBuilder(cluster)
	}

	return &ActivatedKind{
		Kind:              k.Kind,
		Props:             k.Props,
		Strategy:          strategy,
		ActivatorStrategy: activatorStrategy,
		CanSpawnIdentity:  k.CanSpawnIdentity,
	}
}

type ActivatedKind struct {
	Kind              string
	Props             *actor.Props
	Strategy          MemberStrategy
	ActivatorStrategy ActivatorStrategy
	CanSpawnIdentity  func(ctx context.Context, identity string) (bool, error)
	count             int32
}

func (ak *ActivatedKind) Inc() {
	atomic.AddInt32(&ak.count, 1)
}

func (ak *ActivatedKind) Dec() {
	atomic.AddInt32(&ak.count, -1)
}

func (ak *ActivatedKind) Count() int32 {
	return atomic.LoadInt32(&ak.count)
}
