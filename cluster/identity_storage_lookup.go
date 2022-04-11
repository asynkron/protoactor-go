package cluster

import (
	"time"

	"github.com/asynkron/protoactor-go/actor"
)

const (
	placementActorName           = "placement-activator"
	pidClusterIdentityStartIndex = len(placementActorName) + 1
)

// IdentityStorageLookup contains
type IdentityStorageLookup struct {
	Storage        StorageLookup
	cluster        *Cluster
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

// RemoveMember from identity storage
func (i *IdentityStorageLookup) RemoveMember(memberID string) {
	i.Storage.RemoveMemberId(memberID)
}

// RemotePlacementActor returns the PID of the remote placement actor
func RemotePlacementActor(address string) *actor.PID {
	return actor.NewPID(address, placementActorName)
}

//
// Interface: IdentityLookup
//

// Get returns a PID for a given ClusterIdentity
func (id *IdentityStorageLookup) Get(clusterIdentity *ClusterIdentity) *actor.PID {
	msg := newGetPid(clusterIdentity)
	timeout := 5 * time.Second

	res, _ := id.system.Root.RequestFuture(id.router, msg, timeout).Result()
	response := res.(*actor.Future)

	return response.PID()
}

func (id *IdentityStorageLookup) Setup(cluster *Cluster, kinds []string, isClient bool) {
	id.cluster = cluster
	id.system = cluster.ActorSystem
	id.memberID = cluster.ActorSystem.ID

	// workerProps := actor.PropsFromProducer(func() actor.Actor { return newIdentityStorageWorker(identity) })

	// routerProps := identity.system.Root.(workerProps, 50);
}
