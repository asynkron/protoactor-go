package identity

import (
	"time"

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

// RemoveMember
func (i *IdentityStorageLookup) RemoveMember(memberID string) {
	i.Storage.RemoveMemberId(memberID)
}

// RemotePlacementActor
func RemotePlacementActor(address string) *actor.PID {
	return actor.NewPID(address, placementActorName)
}

//
// Interface: Lookup
//

// Get
func (id *IdentityStorageLookup) Get(clusterIdentity *cluster.ClusterIdentity) *actor.PID {
	msg := newGetPid(clusterIdentity)
	timeout := 5 * time.Second

	res, _ := id.system.Root.RequestFuture(id.router, msg, timeout).Result()
	response := res.(*actor.Future)

	return response.PID()
}

func (id *IdentityStorageLookup) Setup(cluster *cluster.Cluster, kinds []string, isClient bool) {
	id.cluster = cluster
	id.system = cluster.ActorSystem
	id.memberID = string(cluster.Id())

	//workerProps := actor.PropsFromProducer(func() actor.Actor { return newIdentityStorageWorker(id) })

	//routerProps := id.system.Root.(workerProps, 50);

}
