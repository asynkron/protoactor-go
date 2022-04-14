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

func newSpawnLock(lockID string, clusterIdentity *ClusterIdentity) *SpawnLock {
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
	ClusterIdentity *ClusterIdentity
}

func newGetPid(clusterIdentity *ClusterIdentity) *GetPid {
	this := &GetPid{
		ClusterIdentity: clusterIdentity,
	}

	return this
}

// PidResult contains
type PidResult struct {
	Pid *actor.PID
}

func newPidResult(p *actor.PID) *PidResult {
	this := &PidResult{
		Pid: p,
	}

	return this
}
