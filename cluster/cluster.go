package cluster

import (
	"github.com/AsynkronIT/gonet"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/remote"
)

func Start(clusterName, address string, provider ClusterProvider) {
	remote.Start(address)
	h, p := gonet.GetAddress(address)
	logdbg.Printf("Starting Proto.Actor cluster on on %v:%v", h, p)
	kinds := remote.GetKnownKinds()
	kindPIDMap = make(map[string]*actor.PID)

	//for each known kind, spin up a partition-kind actor to handle all requests for that kind
	for _, kind := range kinds {
		kindPID := spawnPartitionActor(kind)
		kindPIDMap[kind] = kindPID
	}
	subscribePartitionKindsToEventStream()
	spawnPidCacheActor()
	spawnMembershipActor()
	subscribeMembershipActorToEventStream()
	provider.RegisterMember(clusterName, h, p, kinds)
	provider.MonitorMemberStatusChanges()
}
