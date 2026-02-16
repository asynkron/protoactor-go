package storage

import (
	"log/slog"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
)

// IdentityStorageLookup adapts a cluster.StorageLookup into a
// cluster.IdentityLookup. It uses the storage backend to manage identity
// activations and spawn locks, bridging the external storage to the
// cluster's identity resolution protocol.
type IdentityStorageLookup struct {
	storage  cluster.StorageLookup
	cluster  *cluster.Cluster
	memberID string
	isClient bool
}

// Compile-time check that IdentityStorageLookup implements cluster.IdentityLookup.
var _ cluster.IdentityLookup = (*IdentityStorageLookup)(nil)

// New creates a new IdentityStorageLookup that delegates storage operations
// to the provided StorageLookup backend.
func New(storage cluster.StorageLookup) *IdentityStorageLookup {
	return &IdentityStorageLookup{
		storage: storage,
	}
}

// Get resolves a cluster identity to an actor PID.
//
// The resolution protocol follows these steps:
//  1. Check for an existing activation in the storage backend.
//  2. If no activation exists, attempt to acquire a spawn lock.
//  3. If the lock is acquired (this node should spawn the actor), attempt to
//     spawn the actor and store the activation. If spawning fails, the lock
//     is removed so another node can retry.
//  4. If the lock could not be acquired (another node is spawning), wait for
//     that node to complete the activation and return the result.
//
// Returns nil if the identity could not be resolved (caller should retry).
func (l *IdentityStorageLookup) Get(ci *cluster.ClusterIdentity) *actor.PID {
	// Step 1: Check for an existing activation.
	existing := l.storage.TryGetExistingActivation(ci)
	if existing != nil {
		return l.pidFromStored(existing)
	}

	// Clients cannot spawn actors, so they must wait for a member to do it.
	if l.isClient {
		activation := l.storage.WaitForActivation(ci)
		if activation != nil {
			return l.pidFromStored(activation)
		}
		return nil
	}

	// Step 2: Try to acquire the spawn lock.
	lock := l.storage.TryAcquireLock(ci)
	if lock == nil {
		// Another node is spawning this actor. Wait for it.
		activation := l.storage.WaitForActivation(ci)
		if activation != nil {
			return l.pidFromStored(activation)
		}
		return nil
	}

	// Step 3: We hold the lock — spawn the actor.
	pid := l.spawnActivation(ci, lock)
	if pid == nil {
		// Spawn failed; release the lock so another node can try.
		l.storage.RemoveLock(*lock)
		return nil
	}

	return pid
}

// RemovePid removes the activation for a cluster identity from the storage
// backend. This is called when an actor terminates.
func (l *IdentityStorageLookup) RemovePid(ci *cluster.ClusterIdentity, pid *actor.PID) {
	l.storage.RemoveActivation(&cluster.SpawnLock{
		ClusterIdentity: ci,
	})
}

// Setup initializes the lookup with the cluster context. It subscribes to
// topology events to clean up activations when members leave.
func (l *IdentityStorageLookup) Setup(c *cluster.Cluster, kinds []string, isClient bool) {
	l.cluster = c
	l.isClient = isClient
	l.memberID = c.ActorSystem.ID

	// Subscribe to topology events to remove activations when members leave.
	c.ActorSystem.EventStream.Subscribe(func(evt any) {
		if topology, ok := evt.(*cluster.ClusterTopology); ok {
			for _, member := range topology.Left {
				l.storage.RemoveMemberId(member.Id)
			}
		}
	})
}

// Shutdown performs cleanup when the cluster is shutting down.
// It removes all activations belonging to this member from the storage backend.
func (l *IdentityStorageLookup) Shutdown() {
	if l.memberID != "" {
		l.storage.RemoveMemberId(l.memberID)
	}
}

// spawnActivation attempts to spawn an actor for the given cluster identity
// and store its activation in the backend.
//
// TODO: Full spawn integration requires invoking the cluster's kind-specific
// activator (placement actor). The current implementation uses a simplified
// approach that creates the actor via the cluster's registered kinds.
// For production use with remote placement, this should be integrated with
// the placement actor protocol.
func (l *IdentityStorageLookup) spawnActivation(ci *cluster.ClusterIdentity, lock *cluster.SpawnLock) *actor.PID {
	kind, ok := l.cluster.TryGetClusterKind(ci.Kind)
	if !ok {
		slog.Error("IdentityStorageLookup: unknown kind",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity))
		return nil
	}

	props := cluster.WithClusterIdentity(kind.Props, ci)
	pid, err := l.cluster.ActorSystem.Root.SpawnNamed(props, ci.Kind+"/"+ci.Identity)
	if err != nil {
		slog.Error("IdentityStorageLookup: failed to spawn actor",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity),
			slog.Any("error", err))
		return nil
	}

	// Store the activation and release the lock.
	l.storage.StoreActivation(l.memberID, lock, pid)

	// Also populate the local PID cache.
	l.cluster.PidCache.Set(ci.Identity, ci.Kind, pid)

	return pid
}

// pidFromStored converts a StoredActivation into an actor.PID.
// The StoredActivation.Pid field is formatted as "address/id".
func (l *IdentityStorageLookup) pidFromStored(stored *cluster.StoredActivation) *actor.PID {
	// Parse the PID string. The format is "address/id" as produced by PID.String().
	// We need to split on the first "/" since the address may not contain a slash
	// but the id might contain additional path segments.
	pidStr := stored.Pid
	for i := 0; i < len(pidStr); i++ {
		if pidStr[i] == '/' {
			return actor.NewPID(pidStr[:i], pidStr[i+1:])
		}
	}

	// If no slash found, treat the entire string as the ID with empty address.
	// This shouldn't happen with well-formed data, but we handle it gracefully.
	slog.Warn("IdentityStorageLookup: malformed PID in stored activation",
		slog.String("pid", pidStr))
	return actor.NewPID("", pidStr)
}
