package cluster

import (
	"time"

	"github.com/AsynkronIT/gonet"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/AsynkronIT/protoactor-go/remote"
)

var cfg *ClusterConfig

func Start(clusterName, address string, provider ClusterProvider) {
	StartWithConfig(NewClusterConfig(clusterName, address, provider))
}

func StartWithConfig(config *ClusterConfig) {
	cfg = config

	//TODO: make it possible to become a cluster even if remoting is already started
	remote.Start(cfg.Address, cfg.RemotingOption...)

	address := actor.ProcessRegistry.Address
	h, p := gonet.GetAddress(address)
	plog.Info("Starting Proto.Actor cluster", log.String("address", address))
	kinds := remote.GetKnownKinds()

	//for each known kind, spin up a partition-kind actor to handle all requests for that kind
	spawnPartitionActors(kinds)
	subscribePartitionKindsToEventStream()
	setupPidCache()
	subscribePidCacheMemberStatusEventStream()
	subscribeMemberlistToEventStream()

	cfg.ClusterProvider.RegisterMember(cfg.Name, h, p, kinds, cfg.InitialMemberStatusValue, cfg.MemberStatusValueSerializer)
	cfg.ClusterProvider.MonitorMemberStatusChanges()
}

func Shutdown(graceful bool) {
	if graceful {
		cfg.ClusterProvider.Shutdown()
		//This is to wait ownership transfering complete.
		time.Sleep(2000)
		unsubMemberlistToEventStream()
		unsubPidCacheMemberStatusEventStream()
		stopPidCache()
		unsubPartitionKindsToEventStream()
		stopPartitionActors()
	}

	remote.Shutdown(graceful)

	address := actor.ProcessRegistry.Address
	plog.Info("Stopped Proto.Actor cluster", log.String("address", address))
}

//Get a PID to a virtual actor
func Get(name string, kind string) (*actor.PID, remote.ResponseStatusCode) {
	return getPid(name, kind)
}

//RemoveCache at PidCache
func RemoveCache(name string) {
	pc.removeCacheByName(name)
}
