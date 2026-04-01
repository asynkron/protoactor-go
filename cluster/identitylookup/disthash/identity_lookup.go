// Package disthash implements a distributed hash-based identity lookup.
package disthash

import (
	"log/slog"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
)

// Compile-time check that IdentityLookup implements cluster.GrainEnumerator.
var _ cluster.GrainEnumerator = (*IdentityLookup)(nil)

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

// Peek checks whether a grain activation exists without triggering activation.
func (p *IdentityLookup) Peek(clusterIdentity *cluster.ClusterIdentity) (*cluster.PeekResult, error) {
	panic("not implemented")
}

// New creates a new distributed hash identity lookup implementation.
func New() cluster.IdentityLookup {
	return &IdentityLookup{}
}

// ListGrains returns all known grain activations across the cluster.
func (p *IdentityLookup) ListGrains() ([]*cluster.GrainInfo, error) {
	return p.fanOutListGrains("")
}

// ListGrainsByKind returns grain activations filtered by kind.
func (p *IdentityLookup) ListGrainsByKind(kind string) ([]*cluster.GrainInfo, error) {
	all, err := p.fanOutListGrains("")
	if err != nil {
		return nil, err
	}
	var result []*cluster.GrainInfo
	for _, g := range all {
		if g.Kind == kind {
			result = append(result, g)
		}
	}
	return result, nil
}

// ListGrainsByMember returns grain activations filtered by member ID.
func (p *IdentityLookup) ListGrainsByMember(memberID string) ([]*cluster.GrainInfo, error) {
	return p.fanOutListGrains(memberID)
}

func (p *IdentityLookup) fanOutListGrains(memberID string) ([]*cluster.GrainInfo, error) {
	c := p.partitionManager.cluster
	memberSet := c.MemberList.Members()
	if memberSet == nil {
		return nil, nil
	}

	members := memberSet.Members()
	if len(members) == 0 {
		return nil, nil
	}

	type futureEntry struct {
		future actor.Future
	}

	var futures []futureEntry
	for _, m := range members {
		if memberID != "" && m.Id != memberID {
			continue
		}
		placementPID := p.partitionManager.PidOfActivatorActor(m.Address())
		future := c.ActorSystem.Root.RequestFuture(placementPID, &cluster.ListGrainsRequest{}, 5*time.Second)
		futures = append(futures, futureEntry{future: future})
	}

	var result []*cluster.GrainInfo
	for _, fe := range futures {
		res, err := fe.future.Result()
		if err != nil {
			c.Logger().Warn("ListGrains: failed to query member placement actor", slog.Any("error", err))
			continue
		}
		typed, ok := res.(*cluster.ListGrainsResponse)
		if !ok {
			continue
		}
		result = append(result, typed.Grains...)
	}
	return result, nil
}
