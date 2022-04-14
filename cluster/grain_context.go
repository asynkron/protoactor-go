package cluster

import (
	"github.com/asynkron/protoactor-go/actor"
)

type GrainContext interface {
	actor.Context

	Identity() string
	Kind() string
	Cluster() *Cluster
}

var _ actor.Context = GrainContext(&grainContextImpl{})

type grainContextImpl struct {
	actor.Context
	ci      *ClusterIdentity
	cluster *Cluster
}

func (g grainContextImpl) Identity() string {
	return g.ci.Identity
}

func (g grainContextImpl) Kind() string {
	return g.ci.Kind
}

func (g grainContextImpl) Cluster() *Cluster {
	return g.cluster
}

func NewGrainContext(context actor.Context, identity *ClusterIdentity, cluster *Cluster) GrainContext {
	return &grainContextImpl{
		Context: context,
		ci:      identity,
		cluster: cluster,
	}
}
