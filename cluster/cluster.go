package cluster

import (
	"log"

	"github.com/AsynkronIT/gonet"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/remoting"
)

func Start(clusterName, address string, provider ClusterProvider) {
	remoting.Start(address)
	h, p := gonet.GetAddress(address)
	log.Printf("[CLUSTER] Starting Proto.Actor cluster on on %v:%v", h, p)
	kinds := remoting.GetKnownKinds()
	kindPIDMap = make(map[string]*actor.PID)

	//for each known kind, spin up a partition-kind actor to handle all requests for that kind
	for _, kind := range kinds {
		kindPID := spawnPartitionActor(kind)
		kindPIDMap[kind] = kindPID
	}
	subscribePartitionKindsToEventStream()
	spawnMembershipActor()
	subscribeMembershipActorToEventStream()
	provider.RegisterMember(clusterName, h, p, kinds)
	provider.MonitorMemberStatusChanges()
}
