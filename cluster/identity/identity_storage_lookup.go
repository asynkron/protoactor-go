package identity

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster"
)

const (
	placementActorName           = "placement-activator"
	pidClusterIdentityStartIndex = len(placementActorName) + 1
)

// IdentityStorageLookup contains
type IdentityStorageLookup struct {
	Storage        StorageLookup
	cluster        *cluster.Cluster
	isClient       bool
	placementActor *actor.PID
	system         *actor.ActorSystem
	router         *actor.PID
	memberID       string
}

func newIdentityStorageLookup(storage StorageLookup) *IdentityStorageLookup {
	this := &IdentityStorageLookup{
		Storage: storage,
	}
	return this
}

func (i *IdentityStorageLookup) RemoveMember(memberID string) {
	i.Storage.RemoveMemberId(memberID)
}

func RemotePlacementActor(address string) *actor.PID {
	return actor.NewPID(address, placementActorName)
}

//
// Interface: Lookup
//

/*
func (i *IdentityStorageLookup) Get(clusterIdentity *cluster.ClusterIdentity) *actor.PID {
	msg := newGetPid(clusterIdentity)

	i.system.Root.Request(i.router, msg)
}*/
