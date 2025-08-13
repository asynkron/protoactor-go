package cluster

import (
	"time"

	"github.com/asynkron/protoactor-go/actor"
)

const (
	placementActorName           = "placement-activator"
	pidClusterIdentityStartIndex = len(placementActorName) + 1
)

// IdentityStorageLookup connects identity storage with the cluster for locating actors.
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
	isl := &IdentityStorageLookup{
		Storage: storage,
	}
	return isl
}

// RemoveMember from identity storage
func (isl *IdentityStorageLookup) RemoveMember(memberID string) {
	isl.Storage.RemoveMemberId(memberID)
}

// RemotePlacementActor returns the PID of the remote placement actor
func RemotePlacementActor(address string) *actor.PID {
	return actor.NewPID(address, placementActorName)
}

//
// Interface: IdentityLookup
//

// Get returns a PID for a given ClusterIdentity
func (isl *IdentityStorageLookup) Get(clusterIdentity *ClusterIdentity) *actor.PID {
	msg := newGetPid(clusterIdentity)
	timeout := 5 * time.Second

	res, _ := isl.system.Root.RequestFuture(isl.router, msg, timeout).Result()
	response := res.(actor.Future)

	return response.PID()
}

func (isl *IdentityStorageLookup) Setup(cluster *Cluster, _ []string, _ bool) {
	isl.cluster = cluster
	isl.system = cluster.ActorSystem
	isl.memberID = cluster.ActorSystem.ID

	// workerProps := actor.PropsFromProducer(func() actor.Actor { return newIdentityStorageWorker(identity) })

	// routerProps := identity.system.Root.(workerProps, 50);
}
