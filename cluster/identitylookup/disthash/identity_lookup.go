// Package disthash implements a distributed hash-based identity lookup.
package disthash

import (
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
)

// IdentityLookup resolves cluster identities to actor PIDs using a partition manager.
type IdentityLookup struct {
	partitionManager *Manager
}

// Get returns the PID for the given cluster identity if it exists.
func (p *IdentityLookup) Get(clusterIdentity *cluster.ClusterIdentity) *actor.PID {
	return p.partitionManager.Get(clusterIdentity)
}

// RemovePid removes a PID from the cluster identity registry.
func (p *IdentityLookup) RemovePid(clusterIdentity *cluster.ClusterIdentity, pid *actor.PID) {
	activationTerminated := &cluster.ActivationTerminated{
		Pid:             pid,
		ClusterIdentity: clusterIdentity,
	}
	p.partitionManager.cluster.MemberList.BroadcastEvent(activationTerminated, true)
}

// Setup initializes the identity lookup for the given cluster.
func (p *IdentityLookup) Setup(cluster *cluster.Cluster, _ []string, _ bool) {
	p.partitionManager = newPartitionManager(cluster)
	p.partitionManager.Start()
}

// Shutdown stops the underlying partition manager.
func (p *IdentityLookup) Shutdown() {
	p.partitionManager.Stop()
}

// New creates a new distributed hash identity lookup implementation.
func New() cluster.IdentityLookup {
	return &IdentityLookup{}
}
