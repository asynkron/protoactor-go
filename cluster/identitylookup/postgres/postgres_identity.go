// Package postgres implements cluster.StorageLookup backed by PostgreSQL.
//
// It uses a single table per cluster to store identity activations and locks.
// Row-level locking and TIMESTAMPTZ-based TTL ensure that lock acquisition
// and activation storage are safe across concurrent cluster nodes.
//
// Table schema (created by EnsureSchema):
//
//	{clusterName}_identities (
//	    key            TEXT PRIMARY KEY,
//	    lock_id        TEXT DEFAULT '',
//	    pid_id         TEXT DEFAULT '',
//	    pid_address    TEXT DEFAULT '',
//	    member_id      TEXT DEFAULT '',
//	    lock_expires_at TIMESTAMPTZ
//	)
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/google/uuid"

	// Register the pgx stdlib driver for database/sql.
	_ "github.com/jackc/pgx/v5/stdlib"
)

// PostgresIdentityStorage implements cluster.StorageLookup backed by PostgreSQL.
type PostgresIdentityStorage struct {
	db        *sql.DB
	config    *Config
	logger    *slog.Logger
	tableName string
	semaphore chan struct{}
}

// Compile-time check that PostgresIdentityStorage implements cluster.StorageLookup.
var _ cluster.StorageLookup = (*PostgresIdentityStorage)(nil)

// New creates a new PostgresIdentityStorage with the given cluster name,
// database connection, and optional configuration overrides.
func New(clusterName string, db *sql.DB, opts ...Option) *PostgresIdentityStorage {
	cfg := defaultConfig(clusterName, db)
	for _, opt := range opts {
		opt(cfg)
	}

	return &PostgresIdentityStorage{
		db:        db,
		config:    cfg,
		logger:    cfg.Logger,
		tableName: clusterName + "_identities",
		semaphore: make(chan struct{}, cfg.MaxConcurrency),
	}
}

// SetLogger sets the logger for this storage backend.
func (s *PostgresIdentityStorage) SetLogger(logger *slog.Logger) {
	s.logger = logger
}

func (s *PostgresIdentityStorage) log() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

// EnsureSchema creates the identities table if it does not already exist.
func (s *PostgresIdentityStorage) EnsureSchema(ctx context.Context) error {
	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		key              TEXT PRIMARY KEY,
		lock_id          TEXT NOT NULL DEFAULT '',
		pid_id           TEXT NOT NULL DEFAULT '',
		pid_address      TEXT NOT NULL DEFAULT '',
		member_id        TEXT NOT NULL DEFAULT '',
		lock_expires_at  TIMESTAMPTZ
	)`, s.tableName)

	_, err := s.db.ExecContext(ctx, query)
	return err
}

// acquire acquires a slot from the concurrency semaphore.
func (s *PostgresIdentityStorage) acquire() {
	s.semaphore <- struct{}{}
}

// release releases a slot back to the concurrency semaphore.
func (s *PostgresIdentityStorage) release() {
	<-s.semaphore
}

// TryGetExistingActivation looks up the current activation for a cluster
// identity. Returns nil if no activation exists.
func (s *PostgresIdentityStorage) TryGetExistingActivation(clusterIdentity *cluster.ClusterIdentity) *cluster.StoredActivation {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	key := clusterIdentity.AsKey()

	query := fmt.Sprintf(
		`SELECT pid_id, pid_address, member_id FROM %s WHERE key = $1 AND pid_id != ''`,
		s.tableName,
	)

	var pidID, pidAddr, memberID string
	err := s.db.QueryRowContext(ctx, query, key).Scan(&pidID, &pidAddr, &memberID)
	if err != nil {
		return nil
	}

	return &cluster.StoredActivation{
		Pid:      fmt.Sprintf("%s/%s", pidAddr, pidID),
		MemberID: memberID,
	}
}

// TryAcquireLock attempts to acquire an exclusive spawn lock for the given
// cluster identity. Returns nil if the identity already has a lock or
// activation.
func (s *PostgresIdentityStorage) TryAcquireLock(clusterIdentity *cluster.ClusterIdentity) *cluster.SpawnLock {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	key := clusterIdentity.AsKey()
	lockID := uuid.New().String()
	lockExpires := time.Now().Add(s.config.LockTTL)

	// First, clean up any expired locks for this key (rows that have a lock but no activation).
	cleanQuery := fmt.Sprintf(
		`DELETE FROM %s WHERE key = $1 AND lock_id != '' AND pid_id = '' AND lock_expires_at < NOW()`,
		s.tableName,
	)
	if _, err := s.db.ExecContext(ctx, cleanQuery, key); err != nil {
		s.log().Warn("Postgres identity: cleanup query failed",
			slog.String("key", key), slog.Any("error", err))
	}

	// Try to insert a new lock row. ON CONFLICT DO NOTHING means if the key
	// already exists (locked or activated), the insert is a no-op.
	insertQuery := fmt.Sprintf(
		`INSERT INTO %s (key, lock_id, lock_expires_at) VALUES ($1, $2, $3) ON CONFLICT (key) DO NOTHING`,
		s.tableName,
	)
	result, err := s.db.ExecContext(ctx, insertQuery, key, lockID, lockExpires)
	if err != nil {
		s.log().Error("Postgres TryAcquireLock failed", slog.String("key", key), slog.Any("error", err))
		return nil
	}

	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		return nil
	}

	return &cluster.SpawnLock{
		LockID:          lockID,
		ClusterIdentity: clusterIdentity,
	}
}

// WaitForActivation polls Postgres until an activation appears for the given
// cluster identity, or the lock TTL expires. It uses exponential backoff
// while polling. Returns nil if the activation is not found within the
// timeout period.
func (s *PostgresIdentityStorage) WaitForActivation(clusterIdentity *cluster.ClusterIdentity) *cluster.StoredActivation {
	ctx := context.Background()
	key := clusterIdentity.AsKey()
	deadline := time.Now().Add(s.config.LockTTL)

	query := fmt.Sprintf(
		`SELECT lock_id, pid_id, pid_address, member_id FROM %s WHERE key = $1`,
		s.tableName,
	)

	// Read the initial state to capture the current lock ID.
	s.acquire()
	var lockID, pidID, pidAddr, memberID string
	err := s.db.QueryRowContext(ctx, query, key).Scan(&lockID, &pidID, &pidAddr, &memberID)
	s.release()

	var initialLockID string
	if err == nil {
		initialLockID = lockID
	}

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
		err = s.db.QueryRowContext(ctx, query, key).Scan(&lockID, &pidID, &pidAddr, &memberID)
		s.release()

		if err != nil {
			if initialLockID != "" {
				// Row was deleted (stale lock cleanup) -- let caller retry.
				return nil
			}
			// Row doesn't exist yet -- keep waiting.
			continue
		}

		// If lock_id is empty and pid_id is set, the activation is complete.
		if lockID == "" && pidID != "" {
			return &cluster.StoredActivation{
				Pid:      fmt.Sprintf("%s/%s", pidAddr, pidID),
				MemberID: memberID,
			}
		}

		// If we didn't have a lock ID before, capture the one we just saw.
		if initialLockID == "" {
			initialLockID = lockID
			continue
		}

		// If the lock ID changed, another node took over; let caller retry.
		if lockID != initialLockID {
			return nil
		}
	}

	// Lock TTL expired and still locked by the same request -- stale lock.
	// Remove it so the cluster can retry.
	if initialLockID != "" {
		s.RemoveLock(cluster.SpawnLock{
			LockID:          initialLockID,
			ClusterIdentity: clusterIdentity,
		})
	}

	return nil
}

// RemoveLock removes a spawn lock, but only if the lock ID matches.
// This prevents accidentally removing a lock that was acquired by another node.
func (s *PostgresIdentityStorage) RemoveLock(spawnLock cluster.SpawnLock) {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	key := spawnLock.ClusterIdentity.AsKey()

	query := fmt.Sprintf(
		`DELETE FROM %s WHERE key = $1 AND lock_id = $2`,
		s.tableName,
	)

	_, err := s.db.ExecContext(ctx, query, key, spawnLock.LockID)
	if err != nil {
		s.log().Error("Postgres RemoveLock failed", slog.String("key", key), slog.Any("error", err))
	}
}

// StoreActivation stores a completed activation, associating the PID with the
// cluster identity. The operation is conditional on the spawn lock still being
// held (verified by matching lock_id).
func (s *PostgresIdentityStorage) StoreActivation(memberID string, spawnLock *cluster.SpawnLock, pid *actor.PID) {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	key := spawnLock.ClusterIdentity.AsKey()

	query := fmt.Sprintf(
		`UPDATE %s SET pid_id=$1, pid_address=$2, member_id=$3, lock_id='', lock_expires_at=NULL WHERE key=$4 AND lock_id=$5`,
		s.tableName,
	)

	result, err := s.db.ExecContext(ctx, query, pid.Id, pid.Address, memberID, key, spawnLock.LockID)
	if err != nil {
		s.log().Error("Postgres StoreActivation failed",
			slog.String("key", key), slog.Any("error", err))
		return
	}

	rows, err := result.RowsAffected()
	if err != nil {
		s.log().Warn("Postgres identity: RowsAffected failed",
			slog.String("key", key), slog.Any("error", err))
	}
	if rows == 0 {
		s.log().Warn("Postgres StoreActivation lock mismatch -- lock was lost",
			slog.String("key", key), slog.String("lockID", spawnLock.LockID))
	}
}

// RemoveActivation removes an activation from Postgres.
func (s *PostgresIdentityStorage) RemoveActivation(spawnLock *cluster.SpawnLock) {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	key := spawnLock.ClusterIdentity.AsKey()

	query := fmt.Sprintf(`DELETE FROM %s WHERE key = $1`, s.tableName)

	_, err := s.db.ExecContext(ctx, query, key)
	if err != nil {
		s.log().Error("Postgres RemoveActivation failed",
			slog.String("key", key), slog.Any("error", err))
	}
}

// Compile-time check that PostgresIdentityStorage implements cluster.StorageGrainEnumerator.
var _ cluster.StorageGrainEnumerator = (*PostgresIdentityStorage)(nil)

// ListActivations returns all stored activations.
func (s *PostgresIdentityStorage) ListActivations() ([]*cluster.StoredActivationInfo, error) {
	s.acquire()
	defer s.release()

	ctx := context.Background()

	query := fmt.Sprintf(
		`SELECT key, pid_id, pid_address, member_id FROM %s WHERE pid_id != ''`,
		s.tableName,
	)

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("postgres ListActivations: %w", err)
	}
	defer rows.Close()

	var result []*cluster.StoredActivationInfo
	for rows.Next() {
		var key, pidID, pidAddr, memberID string
		if err := rows.Scan(&key, &pidID, &pidAddr, &memberID); err != nil {
			return nil, fmt.Errorf("postgres ListActivations scan: %w", err)
		}
		kind, identity := cluster.ParseStoredActivationInfoKey(key)
		result = append(result, &cluster.StoredActivationInfo{
			Identity: identity,
			Kind:     kind,
			Pid:      fmt.Sprintf("%s/%s", pidAddr, pidID),
			MemberID: memberID,
		})
	}
	return result, rows.Err()
}

// ListActivationsByMember returns activations belonging to a specific member.
func (s *PostgresIdentityStorage) ListActivationsByMember(memberID string) ([]*cluster.StoredActivationInfo, error) {
	s.acquire()
	defer s.release()

	ctx := context.Background()

	query := fmt.Sprintf(
		`SELECT key, pid_id, pid_address, member_id FROM %s WHERE pid_id != '' AND member_id = $1`,
		s.tableName,
	)

	rows, err := s.db.QueryContext(ctx, query, memberID)
	if err != nil {
		return nil, fmt.Errorf("postgres ListActivationsByMember: %w", err)
	}
	defer rows.Close()

	var result []*cluster.StoredActivationInfo
	for rows.Next() {
		var key, pidID, pidAddr, mID string
		if err := rows.Scan(&key, &pidID, &pidAddr, &mID); err != nil {
			return nil, fmt.Errorf("postgres ListActivationsByMember scan: %w", err)
		}
		kind, identity := cluster.ParseStoredActivationInfoKey(key)
		result = append(result, &cluster.StoredActivationInfo{
			Identity: identity,
			Kind:     kind,
			Pid:      fmt.Sprintf("%s/%s", pidAddr, pidID),
			MemberID: mID,
		})
	}
	return result, rows.Err()
}

// RemoveMemberId removes all activations belonging to the given member.
func (s *PostgresIdentityStorage) RemoveMemberId(memberID string) {
	s.acquire()
	defer s.release()

	ctx := context.Background()

	query := fmt.Sprintf(`DELETE FROM %s WHERE member_id = $1`, s.tableName)

	_, err := s.db.ExecContext(ctx, query, memberID)
	if err != nil {
		s.log().Error("Postgres RemoveMemberId failed",
			slog.String("memberID", memberID), slog.Any("error", err))
	}
}
