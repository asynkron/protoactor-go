package cluster

import (
	"github.com/AsynkronIT/gonet"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/AsynkronIT/protoactor-go/remote"
)

func Start(clusterName, address string, provider ClusterProvider) {
	//TODO: make it possible to become a cluster even if remoting is already started
	remote.Start(address)
	h, p := gonet.GetAddress(address)
	plog.Info("Starting Proto.Actor cluster", log.String("address", address))
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
