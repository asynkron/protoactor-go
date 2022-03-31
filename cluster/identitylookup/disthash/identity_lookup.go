package disthash

import (
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
)

type IdentityLookup struct {
	partitionManager *Manager
}

func (p *IdentityLookup) Get(clusterIdentity *cluster.ClusterIdentity) *actor.PID {
	return p.partitionManager.Get(clusterIdentity)
}

func (p *IdentityLookup) RemovePid(clusterIdentity *cluster.ClusterIdentity, pid *actor.PID) {
	activationTerminated := &cluster.ActivationTerminated{
		Pid:             pid,
		ClusterIdentity: clusterIdentity,
	}
	p.partitionManager.cluster.MemberList.BroadcastEvent(activationTerminated, true)
}

func (p *IdentityLookup) Setup(cluster *cluster.Cluster, kinds []string, isClient bool) {
	p.partitionManager = newPartitionManager(cluster)
	p.partitionManager.Start()
}

func (p *IdentityLookup) Shutdown() {
	p.partitionManager.Stop()
}

func New() cluster.IdentityLookup {
	return &IdentityLookup{}
}
