package redis

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

// RedisIdentityStorage implements cluster.StorageLookup backed by Redis.
//
// It uses Redis hashes to store identity activations and locks, and Redis sets
// to track which identities belong to each cluster member. Lua scripts ensure
// that multi-step operations (lock acquisition, activation storage, removal)
// are executed atomically.
//
// Key schema (compatible with the C# Proto.Cluster.Identity.Redis):
//
//   - Identity hash:  {clusterName}:ci:{kind}/{identity}
//     Fields: lid (lock ID), pid (PID ID), adr (PID address), mid (member ID)
//   - Member set:     {clusterName}:mb:{memberID}
//     Members: identity hash keys belonging to this member
type RedisIdentityStorage struct {
	client    *goredis.Client
	config    *Config
	ciPrefix  string // e.g. "mycluster:ci:"
	mbPrefix  string // e.g. "mycluster:mb:"
	semaphore chan struct{}

	// Lua script objects (loaded lazily by go-redis)
	acquireLockScript      *goredis.Script
	storeActivationScript  *goredis.Script
	removeActivationScript *goredis.Script
	removeMemberScript     *goredis.Script
	removeLockScript       *goredis.Script
}

// Compile-time check that RedisIdentityStorage implements cluster.StorageLookup.
var _ cluster.StorageLookup = (*RedisIdentityStorage)(nil)

// Compile-time check that RedisIdentityStorage implements cluster.StorageGrainEnumerator.
var _ cluster.StorageGrainEnumerator = (*RedisIdentityStorage)(nil)

// New creates a new RedisIdentityStorage with the given cluster name, Redis client,
// and optional configuration overrides.
func New(clusterName string, client *goredis.Client, opts ...Option) *RedisIdentityStorage {
	cfg := defaultConfig(clusterName, client)
	for _, opt := range opts {
		opt(cfg)
	}

	s := &RedisIdentityStorage{
		client:    client,
		config:    cfg,
		ciPrefix:  clusterName + ":ci:",
		mbPrefix:  clusterName + ":mb:",
		semaphore: make(chan struct{}, cfg.MaxConcurrency),

		acquireLockScript:      goredis.NewScript(tryAcquireLockScript),
		storeActivationScript:  goredis.NewScript(storeActivationScript),
		removeActivationScript: goredis.NewScript(removeActivationScript),
		removeMemberScript:     goredis.NewScript(removeMemberScript),
		removeLockScript:       goredis.NewScript(removeLockScript),
	}

	return s
}

// idKey returns the Redis key for a cluster identity hash.
func (s *RedisIdentityStorage) idKey(ci *cluster.ClusterIdentity) string {
	return s.ciPrefix + ci.AsKey()
}

// memberKey returns the Redis key for a member's identity set.
func (s *RedisIdentityStorage) memberKey(memberID string) string {
	return s.mbPrefix + memberID
}

// acquire acquires a slot from the concurrency semaphore.
func (s *RedisIdentityStorage) acquire() {
	s.semaphore <- struct{}{}
}

// release releases a slot back to the concurrency semaphore.
func (s *RedisIdentityStorage) release() {
	<-s.semaphore
}

// TryGetExistingActivation looks up the current activation for a cluster
// identity. Returns nil if no activation exists or if the key only contains
// a lock (no completed activation).
func (s *RedisIdentityStorage) TryGetExistingActivation(clusterIdentity *cluster.ClusterIdentity) *cluster.StoredActivation {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	key := s.idKey(clusterIdentity)

	result, err := s.client.HGetAll(ctx, key).Result()
	if err != nil || len(result) == 0 {
		return nil
	}

	return s.parseActivation(result)
}

// TryAcquireLock attempts to acquire an exclusive spawn lock for the given
// cluster identity. Returns nil if the identity already has a lock or
// activation.
func (s *RedisIdentityStorage) TryAcquireLock(clusterIdentity *cluster.ClusterIdentity) *cluster.SpawnLock {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	lockID := uuid.New().String()
	key := s.idKey(clusterIdentity)
	ttlMs := strconv.FormatInt(s.config.LockTTL.Milliseconds(), 10)

	result, err := s.acquireLockScript.Run(ctx, s.client, []string{key}, lockID, ttlMs).Int()
	if err != nil {
		slog.Error("Redis TryAcquireLock failed", slog.String("key", key), slog.Any("error", err))
		return nil
	}

	if result == 0 {
		return nil
	}

	return &cluster.SpawnLock{
		LockID:          lockID,
		ClusterIdentity: clusterIdentity,
	}
}

// WaitForActivation polls Redis until an activation appears for the given
// cluster identity, or the lock TTL expires. It uses exponential backoff
// while polling. Returns nil if the activation is not found within the
// timeout period.
func (s *RedisIdentityStorage) WaitForActivation(clusterIdentity *cluster.ClusterIdentity) *cluster.StoredActivation {
	ctx := context.Background()
	key := s.idKey(clusterIdentity)
	deadline := time.Now().Add(s.config.LockTTL)

	// Read the initial state to capture the current lock ID.
	s.acquire()
	result, err := s.client.HGetAll(ctx, key).Result()
	s.release()

	if err != nil {
		return nil
	}

	initialLockID := result["lid"]

	iteration := 1
	for time.Now().Before(deadline) {
		// Exponential backoff: 20ms, 40ms, 60ms, ...
		backoff := time.Duration(20*iteration) * time.Millisecond
		if backoff > 500*time.Millisecond {
			backoff = 500 * time.Millisecond
		}
		time.Sleep(backoff)
		iteration++

		s.acquire()
		result, err = s.client.HGetAll(ctx, key).Result()
		s.release()

		if err != nil {
			return nil
		}

		if len(result) == 0 {
			if initialLockID != "" {
				// Key was deleted (stale lock cleanup) — let caller retry.
				return nil
			}
			// Key doesn't exist yet — keep waiting for the lock to be acquired.
			continue
		}

		currentLockID := result["lid"]

		// If the lock field is empty, the activation is complete.
		if currentLockID == "" {
			return s.parseActivation(result)
		}

		// If we didn't have a lock ID before, capture the one we just saw.
		if initialLockID == "" {
			initialLockID = currentLockID
			continue
		}

		// If the lock ID changed, another node took over; let caller retry.
		if currentLockID != initialLockID {
			return nil
		}
	}

	// Lock TTL expired and still locked by the same request — stale lock.
	// Remove it so the cluster can retry.
	s.RemoveLock(cluster.SpawnLock{
		LockID:          initialLockID,
		ClusterIdentity: clusterIdentity,
	})

	return nil
}

// RemoveLock removes a spawn lock, but only if the lock ID matches.
// This prevents accidentally removing a lock that was acquired by another node.
func (s *RedisIdentityStorage) RemoveLock(spawnLock cluster.SpawnLock) {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	key := s.idKey(spawnLock.ClusterIdentity)

	_, err := s.removeLockScript.Run(ctx, s.client, []string{key}, spawnLock.LockID).Result()
	if err != nil && err != goredis.Nil {
		slog.Error("Redis RemoveLock failed", slog.String("key", key), slog.Any("error", err))
	}
}

// StoreActivation stores a completed activation, associating the PID with the
// cluster identity. The operation is conditional on the spawn lock still being
// held (verified atomically via Lua script). The identity key is also added
// to the member's set for tracking.
func (s *RedisIdentityStorage) StoreActivation(memberID string, spawnLock *cluster.SpawnLock, pid *actor.PID) {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	key := s.idKey(spawnLock.ClusterIdentity)
	mbKey := s.memberKey(memberID)

	result, err := s.storeActivationScript.Run(
		ctx, s.client,
		[]string{key, mbKey},
		spawnLock.LockID, pid.Id, pid.Address, memberID,
	).Int()

	if err != nil {
		slog.Error("Redis StoreActivation script failed",
			slog.String("key", key), slog.Any("error", err))
		return
	}

	if result == 0 {
		slog.Warn("Redis StoreActivation lock mismatch — lock was lost",
			slog.String("key", key), slog.String("lockID", spawnLock.LockID))
	}
}

// RemoveActivation removes an activation from Redis, but only if the stored
// PID matches (verified atomically). The identity key is also removed from
// the member's tracking set.
//
// Note: The spawnLock parameter carries the ClusterIdentity; the PID fields
// on the spawnLock are not used. Instead, the activation's stored PID is
// looked up by the cluster identity key.
func (s *RedisIdentityStorage) RemoveActivation(spawnLock *cluster.SpawnLock) {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	key := s.idKey(spawnLock.ClusterIdentity)

	// We need the PID to verify ownership. Read it first.
	result, err := s.client.HGetAll(ctx, key).Result()
	if err != nil || len(result) == 0 {
		return
	}

	pidID := result["pid"]
	pidAddr := result["adr"]

	if pidID == "" || pidAddr == "" {
		// No activation stored (maybe just a lock), just delete the key.
		s.client.Del(ctx, key)
		return
	}

	_, err = s.removeActivationScript.Run(
		ctx, s.client,
		[]string{key},
		pidID, pidAddr, s.mbPrefix,
	).Result()

	if err != nil && err != goredis.Nil {
		slog.Error("Redis RemoveActivation script failed",
			slog.String("key", key), slog.Any("error", err))
	}
}

// RemoveMemberId removes all activations belonging to the given member.
// It scans the member's set of identity keys, deletes each one, then
// deletes the member set itself. The entire operation is performed
// atomically via a Lua script.
func (s *RedisIdentityStorage) RemoveMemberId(memberID string) {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	mbKey := s.memberKey(memberID)

	_, err := s.removeMemberScript.Run(ctx, s.client, []string{mbKey}).Result()
	if err != nil && err != goredis.Nil {
		slog.Error("Redis RemoveMemberId script failed",
			slog.String("memberKey", mbKey), slog.Any("error", err))
	}
}

// parseActivation extracts a StoredActivation from a Redis hash result map.
// Returns nil if the required fields (pid, adr, mid) are not all present,
// which indicates the key only contains a lock.
func (s *RedisIdentityStorage) parseActivation(fields map[string]string) *cluster.StoredActivation {
	pidID := fields["pid"]
	pidAddr := fields["adr"]
	memberID := fields["mid"]

	if pidID == "" || pidAddr == "" || memberID == "" {
		return nil
	}

	// Format the PID as "address/id" to match actor.PID.String() output,
	// which is what StoredActivation.Pid stores.
	return &cluster.StoredActivation{
		Pid:      fmt.Sprintf("%s/%s", pidAddr, pidID),
		MemberID: memberID,
	}
}

// ListActivations returns all stored activations by scanning identity keys.
// Lock-only entries (no completed activation) are excluded.
func (s *RedisIdentityStorage) ListActivations() ([]*cluster.StoredActivationInfo, error) {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	pattern := s.ciPrefix + "*"

	var keys []string
	iter := s.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("redis scan: %w", err)
	}

	if len(keys) == 0 {
		return nil, nil
	}

	// Pipeline HGETALL on all keys.
	pipe := s.client.Pipeline()
	cmds := make([]*goredis.MapStringStringCmd, len(keys))
	for i, key := range keys {
		cmds[i] = pipe.HGetAll(ctx, key)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, fmt.Errorf("redis pipeline exec: %w", err)
	}

	var result []*cluster.StoredActivationInfo
	for i, cmd := range cmds {
		fields, err := cmd.Result()
		if err != nil || len(fields) == 0 {
			continue
		}

		info := s.parseActivationInfo(keys[i], fields)
		if info != nil {
			result = append(result, info)
		}
	}

	return result, nil
}

// ListActivationsByMember returns activations belonging to a specific member.
func (s *RedisIdentityStorage) ListActivationsByMember(memberID string) ([]*cluster.StoredActivationInfo, error) {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	mbKey := s.memberKey(memberID)

	keys, err := s.client.SMembers(ctx, mbKey).Result()
	if err != nil {
		return nil, fmt.Errorf("redis smembers: %w", err)
	}

	if len(keys) == 0 {
		return nil, nil
	}

	// Pipeline HGETALL on each identity key.
	pipe := s.client.Pipeline()
	cmds := make([]*goredis.MapStringStringCmd, len(keys))
	for i, key := range keys {
		cmds[i] = pipe.HGetAll(ctx, key)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, fmt.Errorf("redis pipeline exec: %w", err)
	}

	var result []*cluster.StoredActivationInfo
	for i, cmd := range cmds {
		fields, err := cmd.Result()
		if err != nil || len(fields) == 0 {
			continue
		}

		info := s.parseActivationInfo(keys[i], fields)
		if info != nil {
			result = append(result, info)
		}
	}

	return result, nil
}

// parseActivationInfo extracts a StoredActivationInfo from a Redis hash result
// map and the full key. Returns nil if the entry is lock-only (no activation).
func (s *RedisIdentityStorage) parseActivationInfo(key string, fields map[string]string) *cluster.StoredActivationInfo {
	pidID := fields["pid"]
	pidAddr := fields["adr"]
	memberID := fields["mid"]

	if pidID == "" || pidAddr == "" || memberID == "" {
		return nil
	}

	// Strip the ciPrefix to get "kind/identity".
	stripped := strings.TrimPrefix(key, s.ciPrefix)
	kind, identity := cluster.ParseStoredActivationInfoKey(stripped)

	return &cluster.StoredActivationInfo{
		Identity: identity,
		Kind:     kind,
		Pid:      fmt.Sprintf("%s/%s", pidAddr, pidID),
		MemberID: memberID,
	}
}
