package cluster

import (
	"github.com/asynkron/protoactor-go/actor"
)

// IdentityLookup contains
type IdentityLookup interface {
	Get(clusterIdentity *ClusterIdentity) *actor.PID

	RemovePid(clusterIdentity *ClusterIdentity, pid *actor.PID)

	Setup(cluster *Cluster, kinds []string, isClient bool)

	Shutdown()
}

// StorageLookup contains
type StorageLookup interface {
	TryGetExistingActivation(clusterIdentity *ClusterIdentity) *StoredActivation

	TryAcquireLock(clusterIdentity *ClusterIdentity) *SpawnLock

	WaitForActivation(clusterIdentity *ClusterIdentity) *StoredActivation

	RemoveLock(spawnLock SpawnLock)

	StoreActivation(memberID string, spawnLock *SpawnLock, pid *actor.PID)

	RemoveActivation(pid *SpawnLock)

	RemoveMemberId(memberID string)
}

// SpawnLock contains
type SpawnLock struct {
	LockID          string
	ClusterIdentity *ClusterIdentity
}

// StoredActivation contains
type StoredActivation struct {
	Pid      string
	MemberID string
}

// GetPid contains
type GetPid struct {
	ClusterIdentity *ClusterIdentity
}

// PidResult contains
type PidResult struct {
	Pid *actor.PID
}
