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

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"

	// Register the pgx stdlib driver for database/sql.
	_ "github.com/jackc/pgx/v5/stdlib"
)

// PostgresIdentityStorage implements cluster.StorageLookup backed by PostgreSQL.
type PostgresIdentityStorage struct {
	db        *sql.DB
	config    *Config
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
		tableName: clusterName + "_identities",
		semaphore: make(chan struct{}, cfg.MaxConcurrency),
	}
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
	panic("not implemented")
}

// TryAcquireLock attempts to acquire an exclusive spawn lock for the given
// cluster identity. Returns nil if the identity already has a lock or
// activation.
func (s *PostgresIdentityStorage) TryAcquireLock(clusterIdentity *cluster.ClusterIdentity) *cluster.SpawnLock {
	panic("not implemented")
}

// WaitForActivation polls Postgres until an activation appears for the given
// cluster identity, or the lock TTL expires. Returns nil if the activation
// is not found within the timeout period.
func (s *PostgresIdentityStorage) WaitForActivation(clusterIdentity *cluster.ClusterIdentity) *cluster.StoredActivation {
	panic("not implemented")
}

// RemoveLock removes a spawn lock, but only if the lock ID matches.
func (s *PostgresIdentityStorage) RemoveLock(spawnLock cluster.SpawnLock) {
	panic("not implemented")
}

// StoreActivation stores a completed activation, associating the PID with the
// cluster identity.
func (s *PostgresIdentityStorage) StoreActivation(memberID string, spawnLock *cluster.SpawnLock, pid *actor.PID) {
	panic("not implemented")
}

// RemoveActivation removes an activation from Postgres.
func (s *PostgresIdentityStorage) RemoveActivation(spawnLock *cluster.SpawnLock) {
	panic("not implemented")
}

// RemoveMemberId removes all activations belonging to the given member.
func (s *PostgresIdentityStorage) RemoveMemberId(memberID string) {
	panic("not implemented")
}
