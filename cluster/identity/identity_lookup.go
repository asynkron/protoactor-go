package identity

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster"
)

// Lookup contains
type Lookup interface {
	Get(clusterIdentity *cluster.ClusterIdentity) *actor.PID

	RemovePid(pid *actor.PID)

	Setup(cluster *cluster.Cluster, kinds []string, isClient bool)

	Shutdown()
}

// StorageLookup contains
type StorageLookup interface {
	TryGetExistingActivation(clusterIdentity *cluster.ClusterIdentity) *StoredActivation

	TryAcquireLock(clusterIdentity *cluster.ClusterIdentity) *SpawnLock

	WaitForActivation(clusterIdentity *cluster.ClusterIdentity) *StoredActivation

	RemoveLock(spawnLock SpawnLock)

	StoreActivation(memberID string, spawnLock *SpawnLock, pid *actor.PID)

	RemoveActivation(pid *SpawnLock)

	RemoveMemberId(memberID string)
}

// SpawnLock contains
type SpawnLock struct {
	LockID          string
	ClusterIdentity *cluster.ClusterIdentity
}

func newSpawnLock(lockID string, clusterIdentity *cluster.ClusterIdentity) *SpawnLock {
	this := &SpawnLock{
		LockID:          lockID,
		ClusterIdentity: clusterIdentity,
	}

	return this
}

// StoredActivation contains
type StoredActivation struct {
	Pid      string
	MemberID string
}

func newStoredActivation(pid string, memberID string) *StoredActivation {
	this := &StoredActivation{
		Pid:      pid,
		MemberID: memberID,
	}

	return this
}

// GetPid contains
type GetPid struct {
	ClusterIdentity *cluster.ClusterIdentity
}

func newGetPid(clusterIdentity *cluster.ClusterIdentity) *GetPid {
	this := &GetPid{
		ClusterIdentity: clusterIdentity,
	}
	return this
}
