// Package disthash implements a distributed hash-based identity lookup.
package disthash

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
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

// Peek checks if a grain activation exists without triggering activation.
// For disthash, this hashes to the owner member and sends a PeekRequest
// directly to the placement actor (not via the proxy). This matches how
// disthash's Get() sends ActivationRequest directly to the placement actor,
// unlike natskv/natsstream/storage which route through $proxy-activator.
func (p *IdentityLookup) Peek(clusterIdentity *cluster.ClusterIdentity) (*cluster.PeekResult, error) {
	pm := p.partitionManager
	notFound := &cluster.PeekResult{
		GrainInfo: &cluster.GrainInfo{
			Identity: clusterIdentity.Identity,
			Kind:     clusterIdentity.Kind,
		},
		Status: cluster.PeekStatusNotFound,
	}

	// Snapshot rendezvous under read lock.
	pm.rdvMutex.RLock()
	rdv := pm.rdv
	pm.rdvMutex.RUnlock()

	ownerAddress := rdv.GetByClusterIdentity(clusterIdentity)
	if ownerAddress == "" {
		return notFound, nil
	}

	// Check if the owning member is still in the cluster.
	memberSet := pm.cluster.MemberList.Members()
	if memberSet == nil {
		return notFound, nil
	}

	memberAlive := false
	var ownerMember *cluster.Member
	for _, m := range memberSet.Members() {
		if m.Address() == ownerAddress {
			memberAlive = true
			ownerMember = m
			break
		}
	}

	if !memberAlive {
		return &cluster.PeekResult{
			GrainInfo: &cluster.GrainInfo{
				Identity: clusterIdentity.Identity,
				Kind:     clusterIdentity.Kind,
			},
			Status: cluster.PeekStatusMemberDead,
		}, nil
	}

	// Send PeekRequest to the placement actor on the owning member.
	placementPID := pm.PidOfActivatorActor(ownerAddress)
	future := pm.cluster.ActorSystem.Root.RequestFuture(placementPID, &cluster.PeekRequest{
		ClusterIdentity: clusterIdentity,
	}, 5*time.Second)

	res, err := future.Result()
	if err != nil {
		return nil, fmt.Errorf("peek request to %s failed: %w", ownerAddress, err)
	}

	peekResp, ok := res.(*cluster.PeekResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type from placement actor: %T", res)
	}

	if peekResp.Found {
		return &cluster.PeekResult{
			GrainInfo: &cluster.GrainInfo{
				Identity: clusterIdentity.Identity,
				Kind:     clusterIdentity.Kind,
				PID:      peekResp.Pid,
				MemberID: ownerMember.Id,
			},
			Status: cluster.PeekStatusAlive,
		}, nil
	}

	// disthash has no persistent records — if placement says not found,
	// the grain simply doesn't exist.
	return notFound, nil
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
