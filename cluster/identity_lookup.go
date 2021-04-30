package cluster

import "github.com/AsynkronIT/protoactor-go/actor"

type IdentityLookup interface {
	Get(clusterIdentity *ClusterIdentity)
	RemovePid(clusterIdentity *ClusterIdentity, pid *actor.PID)
	Setup(cluster *Cluster, kinds []string, isClient bool)
	Shutdown()
}
