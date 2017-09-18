package cluster

import (
	"math/rand"
	"time"

	"github.com/AsynkronIT/gonet"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/AsynkronIT/protoactor-go/remote"
)

var(
	cp ClusterProvider
)

func Start(clusterName, address string, provider ClusterProvider) {
	//TODO: make it possible to become a cluster even if remoting is already started
	remote.Start(address)

	cp = provider
	address = actor.ProcessRegistry.Address
	h, p := gonet.GetAddress(address)
	plog.Info("Starting Proto.Actor cluster", log.String("address", address))
	kinds := remote.GetKnownKinds()

	//for each known kind, spin up a partition-kind actor to handle all requests for that kind
	spawnPartitionActors(kinds)
	subscribePartitionKindsToEventStream()
	spawnPidCacheActor()
	subscribePidCacheMemberStatusEventStream()
	spawnMembershipActor()
	subscribeMembershipActorToEventStream()
	cp.RegisterMember(clusterName, h, p, kinds)
	cp.MonitorMemberStatusChanges()
}

func SetUnavailable() {
	cp.DeregisterMember()
}

func Stop(graceful bool) {
	if graceful {
		cp.Shutdown()

		unsubMembershipActorToEventStream()
		stopMembershipActor()
		unsubPidCacheMemberStatusEventStream()
		stopPidCacheActor()
		unsubPartitionKindsToEventStream()
		stopPartitionActors()
	}

	remote.Stop(graceful)

	address := actor.ProcessRegistry.Address
	plog.Info("Stopped Proto.Actor cluster", log.String("address", address))	
}

func getRandomActivator(kind string) string {

	r := rand.Int()
	members := getMembers(kind)
	i := r % len(members)
	member := members[i]
	return member
}

//Get a PID to a virtual actor
func Get(name string, kind string) (*actor.PID, error) {

	req := &pidCacheRequest{
		kind: kind,
		name: name,
	}

	res, err := pidCacheActorPid.RequestFuture(req, 5*time.Second).Result()
	if err != nil {
		plog.Error("ActorPidRequest timed out", log.String("name", name), log.Error(err))
		return nil, err
	}
	typed, ok := res.(*remote.ActorPidResponse)
	if !ok {
		plog.Error("ActorPidRequest returned incorrect response", log.String("name", name))
	}
	return typed.Pid, nil
}
