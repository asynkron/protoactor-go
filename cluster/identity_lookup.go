package cluster

import (
	"time"

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

// GrainInfo describes an active grain activation.
type GrainInfo struct {
	Identity      string
	Kind          string
	PID           *actor.PID
	MemberID      string
	ActivatedAt   time.Time // zero if unknown (e.g. remote grains from storage)
	LastMessageAt time.Time // zero unless WithGrainMetrics() enabled
	MessageCount  int64     // zero unless WithGrainMetrics() enabled
}

// StoredActivationInfo describes a stored activation with parsed identity fields.
type StoredActivationInfo struct {
	Identity string
	Kind     string
	Pid      string // "address/id" format
	MemberID string
}

// GrainEnumerator is an optional interface that IdentityLookup implementations
// may implement to support listing active grain activations.
type GrainEnumerator interface {
	// ListGrains returns all known grain activations.
	ListGrains() ([]*GrainInfo, error)
	// ListGrainsByKind returns grains filtered by kind.
	ListGrainsByKind(kind string) ([]*GrainInfo, error)
	// ListGrainsByMember returns grains owned by a specific member.
	ListGrainsByMember(memberID string) ([]*GrainInfo, error)
}

// StorageGrainEnumerator is an optional interface that StorageLookup backends
// may implement to support listing stored activations.
type StorageGrainEnumerator interface {
	// ListActivations returns all stored activations.
	ListActivations() ([]*StoredActivationInfo, error)
	// ListActivationsByMember returns activations belonging to a specific member.
	ListActivationsByMember(memberID string) ([]*StoredActivationInfo, error)
}
