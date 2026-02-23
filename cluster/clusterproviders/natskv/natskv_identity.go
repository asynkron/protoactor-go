package natskv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
)

// Compile-time check that IdentityLookup implements cluster.IdentityLookup.
var _ cluster.IdentityLookup = (*IdentityLookup)(nil)

// IdentityLookup implements cluster.IdentityLookup directly using NATS JetStream KV
// for lock acquisition, activation storage, and member tracking.
type IdentityLookup struct {
	provider      *Provider
	cluster       *cluster.Cluster
	memberID      string
	isClient      bool
	identities    jetstream.KeyValue
	memberTracker jetstream.KeyValue
	config        *config
	semaphore     chan struct{}
	setupErr      error
}

// activationRecord is the JSON-encoded value stored in the identities KV bucket.
type activationRecord struct {
	LockID     string `json:"lid,omitempty"`
	PidID      string `json:"pid,omitempty"`
	PidAddress string `json:"adr,omitempty"`
	MemberID   string `json:"mid,omitempty"`
}

// memberRecord is the JSON-encoded value stored in the member tracking KV bucket.
type memberRecord struct {
	Keys []string `json:"keys"`
}

// newIdentityLookup creates a new IdentityLookup associated with the given provider.
func newIdentityLookup(p *Provider) *IdentityLookup {
	return &IdentityLookup{
		provider:  p,
		config:    p.config,
		semaphore: make(chan struct{}, p.config.MaxConcurrency),
	}
}

// Setup initializes the identity lookup with the cluster context, creates the
// required KV buckets, and subscribes to topology events for member cleanup.
func (il *IdentityLookup) Setup(c *cluster.Cluster, kinds []string, isClient bool) {
	il.cluster = c
	il.memberID = c.ActorSystem.ID
	il.isClient = isClient

	ctx := context.Background()
	js := il.provider.js
	clusterName := c.Config.Name

	// Create the identities KV bucket (no TTL -- activations persist).
	identities, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:   il.config.identityBucketName(clusterName),
		Replicas: il.config.Replicas,
	})
	if err != nil {
		il.setupErr = fmt.Errorf("natskv identity setup failed: create identities bucket: %w", err)
		slog.Error("natskv identity: failed to create identities bucket",
			slog.Any("error", err))
		return
	}
	il.identities = identities

	// Create the member tracking KV bucket.
	tracker, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:   "protoactor_" + clusterName + "_identities_tracking",
		Replicas: il.config.Replicas,
	})
	if err != nil {
		il.setupErr = fmt.Errorf("natskv identity setup failed: create tracking bucket: %w", err)
		slog.Error("natskv identity: failed to create tracking bucket",
			slog.Any("error", err))
		return
	}
	il.memberTracker = tracker

	// Subscribe to ClusterTopology events to clean up when members leave.
	c.ActorSystem.EventStream.Subscribe(func(evt any) {
		if topology, ok := evt.(*cluster.ClusterTopology); ok {
			for _, member := range topology.Left {
				il.removeMemberID(context.Background(), member.Id)
			}
		}
	})
}

// Get resolves a cluster identity to an actor PID.
//
// The resolution protocol:
//  1. Check for an existing activation.
//  2. If client, wait for activation (cannot spawn).
//  3. Try to acquire a spawn lock.
//  4. If lock acquired, spawn the actor and store the activation.
//  5. If lock not acquired, wait for activation.
func (il *IdentityLookup) Get(ci *cluster.ClusterIdentity) *actor.PID {
	if il.setupErr != nil {
		slog.Error("natskv identity: cannot Get, setup failed", slog.Any("error", il.setupErr))
		return nil
	}
	il.acquire()
	defer il.release()

	ctx := context.Background()

	// Step 1: Check for existing activation.
	rec := il.getExistingActivation(ctx, ci)
	if rec != nil {
		return pidFromRecord(rec)
	}

	// Step 2: If client, wait for a member to spawn the actor.
	if il.isClient {
		rec = il.waitForActivation(ctx, ci)
		if rec != nil {
			return pidFromRecord(rec)
		}
		return nil
	}

	// Step 3: Try to acquire the spawn lock.
	lockID, revision, ok := il.tryAcquireLock(ctx, ci)
	if !ok {
		// Another node is spawning. Wait for it.
		rec = il.waitForActivation(ctx, ci)
		if rec != nil {
			return pidFromRecord(rec)
		}
		return nil
	}

	// Step 4: Lock acquired -- spawn and store activation.
	pid := il.spawnActivation(ci, lockID, revision)
	if pid == nil {
		// Spawn failed; delete the lock key so another node can try.
		_ = il.identities.Delete(ctx, kvKey(ci))
		return nil
	}

	return pid
}

// RemovePid removes the activation for a cluster identity.
func (il *IdentityLookup) RemovePid(ci *cluster.ClusterIdentity, pid *actor.PID) {
	if il.setupErr != nil {
		slog.Error("natskv identity: cannot RemovePid, setup failed", slog.Any("error", il.setupErr))
		return
	}
	ctx := context.Background()
	key := kvKey(ci)

	// Read the entry to find the member ID for tracking cleanup.
	entry, err := il.identities.Get(ctx, key)
	if err == nil {
		var rec activationRecord
		if err := json.Unmarshal(entry.Value(), &rec); err == nil && rec.MemberID != "" {
			il.removeKeyFromMember(ctx, rec.MemberID, key)
		}
	}

	if err := il.identities.Delete(ctx, key); err != nil && !errors.Is(err, jetstream.ErrKeyNotFound) {
		slog.Error("natskv identity: RemovePid delete failed",
			slog.String("key", key), slog.Any("error", err))
	}
}

// Shutdown performs cleanup when the cluster is shutting down.
// It removes all activations belonging to this member.
func (il *IdentityLookup) Shutdown() {
	if il.setupErr != nil {
		return
	}
	if il.memberID != "" {
		il.removeMemberID(context.Background(), il.memberID)
	}
}

// kvKey converts a ClusterIdentity to a NATS KV-safe key.
// ClusterIdentity.AsKey() returns "kind/identity" but NATS KV keys
// cannot contain '/', so we replace it with '.'.
func kvKey(ci *cluster.ClusterIdentity) string {
	return strings.ReplaceAll(ci.AsKey(), "/", ".")
}

// acquire acquires a slot from the concurrency semaphore.
func (il *IdentityLookup) acquire() {
	il.semaphore <- struct{}{}
}

// release releases a slot back to the concurrency semaphore.
func (il *IdentityLookup) release() {
	<-il.semaphore
}

// getExistingActivation looks up the current activation for a cluster identity.
// Returns nil if no activation exists or if the key only contains a lock (no PID).
func (il *IdentityLookup) getExistingActivation(ctx context.Context, ci *cluster.ClusterIdentity) *activationRecord {
	key := kvKey(ci)

	entry, err := il.identities.Get(ctx, key)
	if err != nil {
		return nil
	}

	var rec activationRecord
	if err := json.Unmarshal(entry.Value(), &rec); err != nil {
		return nil
	}

	// Only return if it's a completed activation (has PID info).
	if rec.PidID == "" || rec.PidAddress == "" {
		return nil
	}

	return &rec
}

// tryAcquireLock attempts to acquire an exclusive spawn lock for the given
// cluster identity using NATS KV Create (atomic, fails if key exists).
// Returns the lock ID, the revision for CAS, and whether the lock was acquired.
func (il *IdentityLookup) tryAcquireLock(ctx context.Context, ci *cluster.ClusterIdentity) (lockID string, revision uint64, ok bool) {
	lockID = uuid.New().String()
	key := kvKey(ci)

	rec := activationRecord{
		LockID: lockID,
	}
	data, err := json.Marshal(&rec)
	if err != nil {
		slog.Error("natskv identity: tryAcquireLock marshal failed", slog.Any("error", err))
		return "", 0, false
	}

	revision, err = il.identities.Create(ctx, key, data)
	if err != nil {
		if !errors.Is(err, jetstream.ErrKeyExists) {
			slog.Error("natskv identity: tryAcquireLock failed",
				slog.String("key", key), slog.Any("error", err))
		}
		return "", 0, false
	}

	return lockID, revision, true
}

// storeActivation stores a completed activation using CAS (revision-based Update),
// then tracks the key in the member's tracking record.
func (il *IdentityLookup) storeActivation(ctx context.Context, ci *cluster.ClusterIdentity, lockID string, revision uint64, memberID, pidAddress, pidID string) error {
	key := kvKey(ci)

	updated := activationRecord{
		LockID:     "", // Clear the lock.
		PidID:      pidID,
		PidAddress: pidAddress,
		MemberID:   memberID,
	}
	data, err := json.Marshal(&updated)
	if err != nil {
		return err
	}

	_, err = il.identities.Update(ctx, key, data, revision)
	if err != nil {
		return err
	}

	// Track this identity key under the member.
	il.addKeyToMember(ctx, memberID, key)
	return nil
}

// waitForActivation watches the NATS KV key for the given cluster identity
// until an activation appears (PID is set), or the lock TTL timeout expires.
func (il *IdentityLookup) waitForActivation(ctx context.Context, ci *cluster.ClusterIdentity) *activationRecord {
	key := kvKey(ci)
	watchCtx, cancel := context.WithTimeout(ctx, il.config.LockTTL)
	defer cancel()

	watcher, err := il.identities.Watch(watchCtx, key)
	if err != nil {
		slog.Error("natskv identity: waitForActivation watch failed",
			slog.String("key", key), slog.Any("error", err))
		return nil
	}
	defer watcher.Stop()

	for entry := range watcher.Updates() {
		if entry == nil {
			// Initial values done signal -- continue waiting for real updates.
			continue
		}

		if entry.Operation() == jetstream.KeyValueDelete || entry.Operation() == jetstream.KeyValuePurge {
			// Key was deleted -- lock was removed, let caller retry.
			return nil
		}

		var rec activationRecord
		if err := json.Unmarshal(entry.Value(), &rec); err != nil {
			continue
		}

		// Activation is complete when PID is set.
		if rec.PidID != "" && rec.PidAddress != "" {
			return &rec
		}
	}

	return nil
}

// removeMemberID removes all activations belonging to the given member.
// It reads the member tracking record, deletes each identity key, then
// deletes the member record itself.
func (il *IdentityLookup) removeMemberID(ctx context.Context, memberID string) {
	if il.memberTracker == nil {
		return
	}

	entry, err := il.memberTracker.Get(ctx, memberID)
	if err != nil {
		return
	}

	var mrec memberRecord
	if err := json.Unmarshal(entry.Value(), &mrec); err != nil {
		slog.Error("natskv identity: removeMemberID unmarshal failed",
			slog.String("memberID", memberID), slog.Any("error", err))
		return
	}

	// Delete each identity key belonging to this member.
	for _, key := range mrec.Keys {
		if err := il.identities.Delete(ctx, key); err != nil && !errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.Error("natskv identity: removeMemberID delete identity failed",
				slog.String("key", key), slog.Any("error", err))
		}
	}

	// Delete the member tracking record itself.
	if err := il.memberTracker.Delete(ctx, memberID); err != nil && !errors.Is(err, jetstream.ErrKeyNotFound) {
		slog.Error("natskv identity: removeMemberID delete member failed",
			slog.String("memberID", memberID), slog.Any("error", err))
	}
}

// addKeyToMember adds an identity key to a member's tracking record.
// Uses a CAS-loop with retry on conflict.
func (il *IdentityLookup) addKeyToMember(ctx context.Context, memberID, key string) {
	for i := 0; i < 3; i++ {
		entry, err := il.memberTracker.Get(ctx, memberID)
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			// Create a new member record.
			mrec := memberRecord{Keys: []string{key}}
			data, err := json.Marshal(&mrec)
			if err != nil {
				slog.Error("natskv identity: addKeyToMember marshal failed",
					slog.String("memberID", memberID), slog.Any("error", err))
				return
			}
			_, err = il.memberTracker.Create(ctx, memberID, data)
			if err == nil {
				return
			}
			if errors.Is(err, jetstream.ErrKeyExists) {
				// Another goroutine created it first -- retry with update.
				continue
			}
			slog.Error("natskv identity: addKeyToMember create failed",
				slog.String("memberID", memberID), slog.Any("error", err))
			return
		}
		if err != nil {
			slog.Error("natskv identity: addKeyToMember get failed",
				slog.String("memberID", memberID), slog.Any("error", err))
			return
		}

		var mrec memberRecord
		if err := json.Unmarshal(entry.Value(), &mrec); err != nil {
			slog.Error("natskv identity: addKeyToMember unmarshal failed",
				slog.String("memberID", memberID), slog.Any("error", err))
			return
		}

		// Check if key already exists.
		for _, k := range mrec.Keys {
			if k == key {
				return
			}
		}

		mrec.Keys = append(mrec.Keys, key)
		data, err := json.Marshal(&mrec)
		if err != nil {
			slog.Error("natskv identity: addKeyToMember marshal failed",
				slog.String("memberID", memberID), slog.Any("error", err))
			return
		}

		_, err = il.memberTracker.Update(ctx, memberID, data, entry.Revision())
		if err == nil {
			return
		}
		// CAS conflict -- retry.
		time.Sleep(10 * time.Millisecond)
	}
}

// removeKeyFromMember removes an identity key from a member's tracking record.
// Uses a CAS-loop with retry on conflict.
func (il *IdentityLookup) removeKeyFromMember(ctx context.Context, memberID, key string) {
	for i := 0; i < 3; i++ {
		entry, err := il.memberTracker.Get(ctx, memberID)
		if err != nil {
			return
		}

		var mrec memberRecord
		if err := json.Unmarshal(entry.Value(), &mrec); err != nil {
			slog.Error("natskv identity: removeKeyFromMember unmarshal failed",
				slog.String("memberID", memberID), slog.Any("error", err))
			return
		}

		// Filter out the key.
		filtered := make([]string, 0, len(mrec.Keys))
		for _, k := range mrec.Keys {
			if k != key {
				filtered = append(filtered, k)
			}
		}
		mrec.Keys = filtered

		data, err := json.Marshal(&mrec)
		if err != nil {
			slog.Error("natskv identity: removeKeyFromMember marshal failed",
				slog.String("memberID", memberID), slog.Any("error", err))
			return
		}

		_, err = il.memberTracker.Update(ctx, memberID, data, entry.Revision())
		if err == nil {
			return
		}
		// CAS conflict -- retry.
		time.Sleep(10 * time.Millisecond)
	}
}

// spawnActivation attempts to spawn an actor for the given cluster identity,
// stores the activation, and populates the PID cache.
func (il *IdentityLookup) spawnActivation(ci *cluster.ClusterIdentity, lockID string, revision uint64) *actor.PID {
	kind, ok := il.cluster.TryGetClusterKind(ci.Kind)
	if !ok {
		slog.Error("natskv identity: unknown kind",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity))
		return nil
	}

	props := cluster.WithClusterIdentity(kind.Props, ci)
	pid, err := il.cluster.ActorSystem.Root.SpawnNamed(props, ci.Kind+"/"+ci.Identity)
	if err != nil {
		slog.Error("natskv identity: failed to spawn actor",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity),
			slog.Any("error", err))
		return nil
	}

	// Store the activation (CAS update with revision from lock creation).
	ctx := context.Background()
	storeErr := il.storeActivation(ctx, ci, lockID, revision, il.memberID, pid.Address, pid.Id)
	if storeErr != nil {
		slog.Error("natskv identity: failed to store activation",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity),
			slog.Any("error", storeErr))
		// Poison the spawned actor to prevent orphaned processes.
		il.cluster.ActorSystem.Root.Poison(pid)
		return nil
	}

	// Populate the local PID cache.
	il.cluster.PidCache.Set(ci.Identity, ci.Kind, pid)

	return pid
}

// identityLogger returns the cluster logger if available, otherwise the default logger.
func (il *IdentityLookup) identityLogger() *slog.Logger {
	if il.cluster != nil {
		return il.cluster.Logger()
	}
	return slog.Default()
}

// pidFromRecord converts an activationRecord into an actor.PID.
func pidFromRecord(rec *activationRecord) *actor.PID {
	return actor.NewPID(rec.PidAddress, rec.PidID)
}
