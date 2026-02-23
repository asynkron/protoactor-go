// Package nats implements cluster.StorageLookup backed by NATS JetStream KV.
//
// It uses two KV buckets per cluster:
//
//   - {clusterName}_identities: stores activation records keyed by {kind}.{identity}
//     (NATS KV keys cannot contain '/' so '.' is used as separator)
//   - {clusterName}_members: maps memberID to a JSON list of identity keys
//
// The atomic Create operation on NATS KV (which fails with ErrKeyExists if the
// key already exists) is used to implement lock acquisition. CAS updates via
// revision-based Update calls protect activation storage.
package nats

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

// activationRecord is the JSON-encoded value stored in the identities KV bucket.
type activationRecord struct {
	LockID     string `json:"lid,omitempty"`
	PidID      string `json:"pid,omitempty"`
	PidAddress string `json:"adr,omitempty"`
	MemberID   string `json:"mid,omitempty"`
}

// memberRecord is the JSON-encoded value stored in the members KV bucket.
type memberRecord struct {
	Keys []string `json:"keys"`
}

// NatsIdentityStorage implements cluster.StorageLookup using NATS JetStream KV.
type NatsIdentityStorage struct {
	identities jetstream.KeyValue
	members    jetstream.KeyValue
	config     *Config
	semaphore  chan struct{}
}

// Compile-time check that NatsIdentityStorage implements cluster.StorageLookup.
var _ cluster.StorageLookup = (*NatsIdentityStorage)(nil)

// New creates a new NatsIdentityStorage with the given cluster name,
// JetStream connection, and optional configuration overrides. It creates
// or ensures the two KV buckets needed for operation.
func New(clusterName string, js jetstream.JetStream, opts ...Option) (*NatsIdentityStorage, error) {
	cfg := defaultConfig(clusterName)
	for _, opt := range opts {
		opt(cfg)
	}

	ctx := context.Background()

	identities, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: clusterName + "_identities",
	})
	if err != nil {
		return nil, fmt.Errorf("nats identity: create identities bucket: %w", err)
	}

	members, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: clusterName + "_members",
	})
	if err != nil {
		return nil, fmt.Errorf("nats identity: create members bucket: %w", err)
	}

	return &NatsIdentityStorage{
		identities: identities,
		members:    members,
		config:     cfg,
		semaphore:  make(chan struct{}, cfg.MaxConcurrency),
	}, nil
}

// kvKey converts a ClusterIdentity to a NATS KV-safe key.
// ClusterIdentity.AsKey() returns "kind/identity" but NATS KV keys
// cannot contain '/', so we replace it with '.'.
func kvKey(ci *cluster.ClusterIdentity) string {
	return strings.ReplaceAll(ci.AsKey(), "/", ".")
}

// acquire acquires a slot from the concurrency semaphore.
func (s *NatsIdentityStorage) acquire() {
	s.semaphore <- struct{}{}
}

// release releases a slot back to the concurrency semaphore.
func (s *NatsIdentityStorage) release() {
	<-s.semaphore
}

// TryGetExistingActivation looks up the current activation for a cluster
// identity. Returns nil if no activation exists or if the key only contains
// a lock (no completed activation).
func (s *NatsIdentityStorage) TryGetExistingActivation(clusterIdentity *cluster.ClusterIdentity) *cluster.StoredActivation {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	key := kvKey(clusterIdentity)

	entry, err := s.identities.Get(ctx, key)
	if err != nil {
		return nil
	}

	var rec activationRecord
	if err := json.Unmarshal(entry.Value(), &rec); err != nil {
		return nil
	}

	if rec.PidID == "" || rec.PidAddress == "" || rec.MemberID == "" {
		return nil
	}

	return &cluster.StoredActivation{
		Pid:      fmt.Sprintf("%s/%s", rec.PidAddress, rec.PidID),
		MemberID: rec.MemberID,
	}
}

// TryAcquireLock attempts to acquire an exclusive spawn lock for the given
// cluster identity. Uses NATS KV Create which atomically fails if the key
// already exists. Returns nil if the identity already has a lock or activation.
func (s *NatsIdentityStorage) TryAcquireLock(clusterIdentity *cluster.ClusterIdentity) *cluster.SpawnLock {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	lockID := uuid.New().String()
	key := kvKey(clusterIdentity)

	rec := activationRecord{
		LockID: lockID,
	}
	data, err := json.Marshal(&rec)
	if err != nil {
		slog.Error("NATS TryAcquireLock marshal failed", slog.Any("error", err))
		return nil
	}

	_, err = s.identities.Create(ctx, key, data)
	if err != nil {
		// ErrKeyExists means key already exists (locked or activated).
		// Any other error is also treated as failure.
		if !errors.Is(err, jetstream.ErrKeyExists) {
			slog.Error("NATS TryAcquireLock failed",
				slog.String("key", key), slog.Any("error", err))
		}
		return nil
	}

	return &cluster.SpawnLock{
		LockID:          lockID,
		ClusterIdentity: clusterIdentity,
	}
}

// WaitForActivation watches the NATS KV key for the given cluster identity
// until an activation appears (LockID is empty and PidID is set), or the
// lock TTL timeout expires. Returns nil if no activation appears in time.
func (s *NatsIdentityStorage) WaitForActivation(clusterIdentity *cluster.ClusterIdentity) *cluster.StoredActivation {
	key := kvKey(clusterIdentity)
	ctx, cancel := context.WithTimeout(context.Background(), s.config.LockTTL)
	defer cancel()

	watcher, err := s.identities.Watch(ctx, key)
	if err != nil {
		slog.Error("NATS WaitForActivation watch failed",
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

		// Activation is complete when lock is empty and PID is set.
		if rec.LockID == "" && rec.PidID != "" {
			return &cluster.StoredActivation{
				Pid:      fmt.Sprintf("%s/%s", rec.PidAddress, rec.PidID),
				MemberID: rec.MemberID,
			}
		}
	}

	return nil
}

// RemoveLock removes a spawn lock, but only if the lock ID matches.
// This prevents accidentally removing a lock that was acquired by another node.
func (s *NatsIdentityStorage) RemoveLock(spawnLock cluster.SpawnLock) {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	key := kvKey(spawnLock.ClusterIdentity)

	entry, err := s.identities.Get(ctx, key)
	if err != nil {
		return
	}

	var rec activationRecord
	if err := json.Unmarshal(entry.Value(), &rec); err != nil {
		return
	}

	// Only remove if the lock ID matches.
	if rec.LockID != spawnLock.LockID {
		return
	}

	if err := s.identities.Delete(ctx, key); err != nil {
		slog.Error("NATS RemoveLock delete failed",
			slog.String("key", key), slog.Any("error", err))
	}
}

// StoreActivation stores a completed activation, associating the PID with the
// cluster identity. The operation is conditional on the spawn lock still being
// held (verified by reading the current record and checking the lock ID, then
// using a CAS update via revision). The identity key is also added to the
// member's tracking bucket.
func (s *NatsIdentityStorage) StoreActivation(memberID string, spawnLock *cluster.SpawnLock, pid *actor.PID) {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	key := kvKey(spawnLock.ClusterIdentity)

	// Read current entry to verify lock ownership and get revision for CAS.
	entry, err := s.identities.Get(ctx, key)
	if err != nil {
		slog.Error("NATS StoreActivation get failed",
			slog.String("key", key), slog.Any("error", err))
		return
	}

	var rec activationRecord
	if err := json.Unmarshal(entry.Value(), &rec); err != nil {
		slog.Error("NATS StoreActivation unmarshal failed",
			slog.String("key", key), slog.Any("error", err))
		return
	}

	if rec.LockID != spawnLock.LockID {
		slog.Warn("NATS StoreActivation lock mismatch -- lock was lost",
			slog.String("key", key), slog.String("lockID", spawnLock.LockID))
		return
	}

	// Build the updated record with PID info and cleared lock.
	updated := activationRecord{
		LockID:     "",
		PidID:      pid.Id,
		PidAddress: pid.Address,
		MemberID:   memberID,
	}
	data, err := json.Marshal(&updated)
	if err != nil {
		slog.Error("NATS StoreActivation marshal failed", slog.Any("error", err))
		return
	}

	// CAS update using the revision from the Get.
	_, err = s.identities.Update(ctx, key, data, entry.Revision())
	if err != nil {
		slog.Error("NATS StoreActivation update failed",
			slog.String("key", key), slog.Any("error", err))
		return
	}

	// Track this identity key under the member.
	s.addKeyToMember(ctx, memberID, key)
}

// RemoveActivation removes an activation from the identities bucket.
// Also cleans up the member tracking entry.
func (s *NatsIdentityStorage) RemoveActivation(spawnLock *cluster.SpawnLock) {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	key := kvKey(spawnLock.ClusterIdentity)

	// Read the entry to find the member ID for cleanup.
	entry, err := s.identities.Get(ctx, key)
	if err != nil {
		return
	}

	var rec activationRecord
	if err := json.Unmarshal(entry.Value(), &rec); err == nil && rec.MemberID != "" {
		s.removeKeyFromMember(ctx, rec.MemberID, key)
	}

	if err := s.identities.Delete(ctx, key); err != nil {
		slog.Error("NATS RemoveActivation delete failed",
			slog.String("key", key), slog.Any("error", err))
	}
}

// RemoveMemberId removes all activations belonging to the given member.
// It reads the member record, deletes each identity key, then deletes
// the member record itself.
func (s *NatsIdentityStorage) RemoveMemberId(memberID string) {
	s.acquire()
	defer s.release()

	ctx := context.Background()

	entry, err := s.members.Get(ctx, memberID)
	if err != nil {
		// No member record -- nothing to clean up.
		return
	}

	var mrec memberRecord
	if err := json.Unmarshal(entry.Value(), &mrec); err != nil {
		slog.Error("NATS RemoveMemberId unmarshal failed",
			slog.String("memberID", memberID), slog.Any("error", err))
		return
	}

	// Delete each identity key belonging to this member.
	for _, key := range mrec.Keys {
		if err := s.identities.Delete(ctx, key); err != nil && !errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.Error("NATS RemoveMemberId delete identity failed",
				slog.String("key", key), slog.Any("error", err))
		}
	}

	// Delete the member record itself.
	if err := s.members.Delete(ctx, memberID); err != nil && !errors.Is(err, jetstream.ErrKeyNotFound) {
		slog.Error("NATS RemoveMemberId delete member failed",
			slog.String("memberID", memberID), slog.Any("error", err))
	}
}

// addKeyToMember adds an identity key to a member's tracking record.
func (s *NatsIdentityStorage) addKeyToMember(ctx context.Context, memberID, key string) {
	for i := 0; i < 3; i++ {
		entry, err := s.members.Get(ctx, memberID)
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			// Create a new member record.
			mrec := memberRecord{Keys: []string{key}}
			data, err := json.Marshal(&mrec)
			if err != nil {
				slog.Error("NATS identity: addKeyToMember marshal failed",
					slog.String("memberID", memberID), slog.Any("error", err))
				return
			}
			_, err = s.members.Create(ctx, memberID, data)
			if err == nil {
				return
			}
			if errors.Is(err, jetstream.ErrKeyExists) {
				// Another goroutine created it first -- retry with update.
				continue
			}
			slog.Error("NATS addKeyToMember create failed",
				slog.String("memberID", memberID), slog.Any("error", err))
			return
		}
		if err != nil {
			slog.Error("NATS addKeyToMember get failed",
				slog.String("memberID", memberID), slog.Any("error", err))
			return
		}

		var mrec memberRecord
		if err := json.Unmarshal(entry.Value(), &mrec); err != nil {
			slog.Error("NATS addKeyToMember unmarshal failed",
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
			slog.Error("NATS identity: addKeyToMember marshal failed",
				slog.String("memberID", memberID), slog.Any("error", err))
			return
		}

		_, err = s.members.Update(ctx, memberID, data, entry.Revision())
		if err == nil {
			return
		}
		// CAS conflict -- retry.
		time.Sleep(10 * time.Millisecond)
	}
}

// removeKeyFromMember removes an identity key from a member's tracking record.
func (s *NatsIdentityStorage) removeKeyFromMember(ctx context.Context, memberID, key string) {
	entry, err := s.members.Get(ctx, memberID)
	if err != nil {
		return
	}

	var mrec memberRecord
	if err := json.Unmarshal(entry.Value(), &mrec); err != nil {
		return
	}

	// Filter out the key.
	filtered := mrec.Keys[:0]
	for _, k := range mrec.Keys {
		if k != key {
			filtered = append(filtered, k)
		}
	}
	mrec.Keys = filtered

	data, err := json.Marshal(&mrec)
	if err != nil {
		slog.Error("NATS identity: removeKeyFromMember marshal failed",
			slog.String("memberID", memberID), slog.Any("error", err))
		return
	}
	if _, err := s.members.Update(ctx, memberID, data, entry.Revision()); err != nil {
		slog.Warn("NATS identity: removeKeyFromMember CAS update failed, will be cleaned up on member leave",
			slog.String("memberID", memberID), slog.Any("error", err))
	}
}
