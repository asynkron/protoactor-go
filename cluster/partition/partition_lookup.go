package partition

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster"
)

type PartitionIdentityLookup struct {
	partitionManager *PartitionManager
}

func (p *PartitionIdentityLookup) Get(clusterIdentity *cluster.ClusterIdentity) *actor.PID {
	return p.partitionManager.Get(clusterIdentity)
}

func (p *PartitionIdentityLookup) RemovePid(clusterIdentity *cluster.ClusterIdentity, pid *actor.PID) {
	activationTerminated := &cluster.ActivationTerminated{
		Pid:             pid,
		ClusterIdentity: clusterIdentity,
	}
	p.partitionManager.cluster.MemberList.BroadcastEvent(activationTerminated, true)
}

func (p *PartitionIdentityLookup) Setup(cluster *cluster.Cluster, kinds []string, isClient bool) {
	p.partitionManager = newPartitionManager(cluster)
	p.partitionManager.Start()
}

func (p *PartitionIdentityLookup) Shutdown() {
	p.partitionManager.Stop()
}

func New() cluster.IdentityLookup {
	return &PartitionIdentityLookup{}
}
