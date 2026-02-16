# NATS Examples & Identity Lookup Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add five NATS-based examples plus two new StorageLookup implementations (Postgres, NATS JetStream KV) and a LocalAffinityStrategy for subject-based placement.

**Architecture:** Library packages go under `cluster/identitylookup/{postgres,nats}/` following the existing Redis pattern. Examples are self-contained Go modules under `examples/` with docker-compose infrastructure. The affinity strategy is demonstrated in the locality example's `shared/` package.

**Tech Stack:** Go 1.25, nats.go (with JetStream), pgx/v5, protobuf, Consul cluster provider, existing protoactor-go infrastructure.

---

### Task 1: Postgres StorageLookup — Failing Tests

**Files:**
- Create: `cluster/identitylookup/postgres/postgres_identity.go`
- Create: `cluster/identitylookup/postgres/options.go`
- Create: `cluster/identitylookup/postgres/postgres_identity_test.go`

**Context:**
- The `cluster.StorageLookup` interface is at `cluster/identity_lookup.go:19-33`
- The conformance suite is at `cluster/identitylookup/conformance.go` — all new backends must pass it
- Redis implementation at `cluster/identitylookup/redis/redis_identity.go` is the reference
- Redis test at `cluster/identitylookup/redis/redis_identity_test.go` shows the testcontainers pattern

**Step 1: Create options.go**

```go
// cluster/identitylookup/postgres/options.go
package postgres

import (
	"database/sql"
	"time"
)

type Config struct {
	DB             *sql.DB
	ClusterName    string
	LockTTL        time.Duration
	MaxConcurrency int
}

type Option func(*Config)

func WithLockTTL(ttl time.Duration) Option {
	return func(c *Config) { c.LockTTL = ttl }
}

func WithMaxConcurrency(n int) Option {
	return func(c *Config) { c.MaxConcurrency = n }
}

func defaultConfig(clusterName string, db *sql.DB) *Config {
	return &Config{
		DB:             db,
		ClusterName:    clusterName,
		LockTTL:        5 * time.Second,
		MaxConcurrency: 200,
	}
}
```

**Step 2: Create postgres_identity.go skeleton**

```go
// cluster/identitylookup/postgres/postgres_identity.go
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
)

type PostgresIdentityStorage struct {
	db        *sql.DB
	config    *Config
	tableName string
	semaphore chan struct{}
}

var _ cluster.StorageLookup = (*PostgresIdentityStorage)(nil)

func New(clusterName string, db *sql.DB, opts ...Option) *PostgresIdentityStorage {
	cfg := defaultConfig(clusterName, db)
	for _, opt := range opts {
		opt(cfg)
	}
	s := &PostgresIdentityStorage{
		db:        db,
		config:    cfg,
		tableName: clusterName + "_identities",
		semaphore: make(chan struct{}, cfg.MaxConcurrency),
	}
	return s
}

// EnsureSchema creates the identities table if it doesn't exist.
func (s *PostgresIdentityStorage) EnsureSchema(ctx context.Context) error {
	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		key TEXT PRIMARY KEY,
		lock_id TEXT NOT NULL DEFAULT '',
		pid_id TEXT NOT NULL DEFAULT '',
		pid_address TEXT NOT NULL DEFAULT '',
		member_id TEXT NOT NULL DEFAULT '',
		lock_expires_at TIMESTAMPTZ
	)`, s.tableName)
	_, err := s.db.ExecContext(ctx, query)
	return err
}

func (s *PostgresIdentityStorage) acquire() { s.semaphore <- struct{}{} }
func (s *PostgresIdentityStorage) release() { <-s.semaphore }

func (s *PostgresIdentityStorage) idKey(ci *cluster.ClusterIdentity) string {
	return ci.AsKey()
}
```

Stub all interface methods to panic with "not implemented" so the test file compiles.

**Step 3: Create test file**

```go
// cluster/identitylookup/postgres/postgres_identity_test.go
//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/identitylookup"
	pgidentity "github.com/asynkron/protoactor-go/cluster/identitylookup/postgres"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	dsn := os.Getenv("POSTGRES_DSN")

	if dsn == "" {
		ctx := context.Background()
		container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        "postgres:16-alpine",
				ExposedPorts: []string{"5432/tcp"},
				Env: map[string]string{
					"POSTGRES_USER":     "test",
					"POSTGRES_PASSWORD": "test",
					"POSTGRES_DB":       "test",
				},
				WaitingFor: wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).WithStartupTimeout(30 * time.Second),
			},
			Started: true,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to start postgres container: %v\n", err)
			os.Exit(1)
		}

		host, _ := container.Host(ctx)
		port, _ := container.MappedPort(ctx, "5432")
		dsn = fmt.Sprintf("postgres://test:test@%s:%s/test?sslmode=disable", host, port.Port())

		defer func() { _ = container.Terminate(ctx) }()
	}

	var err error
	testDB, err = sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to postgres: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func TestPostgresConformance(t *testing.T) {
	suite := &identitylookup.StorageConformanceSuite{
		NewStorage: func() cluster.StorageLookup {
			s := pgidentity.New("test-cluster", testDB)
			if err := s.EnsureSchema(context.Background()); err != nil {
				t.Fatalf("EnsureSchema: %v", err)
			}
			return s
		},
		Cleanup: func() {
			_, _ = testDB.Exec("DELETE FROM test_cluster_identities")
		},
	}
	suite.RunAll(t)
}
```

**Step 4: Add pgx dependency to go.mod**

Run: `cd /home/cchamplin/development/protoactor-go && go get github.com/jackc/pgx/v5`

**Step 5: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/protoactor-go && go test -tags integration -v ./cluster/identitylookup/postgres/...`
Expected: FAIL — stub methods panic with "not implemented"

**Step 6: Commit**

```
feat(identity): add Postgres StorageLookup test scaffold

Conformance test using testcontainers; stubs not yet implemented.
```

---

### Task 2: Postgres StorageLookup — Implementation

**Files:**
- Modify: `cluster/identitylookup/postgres/postgres_identity.go`

**Context:**
- Redis Lua scripts at `cluster/identitylookup/redis/lua_scripts.go` show the atomic operation semantics
- `StorageLookup` interface at `cluster/identity_lookup.go:19-33`
- `SpawnLock` at `cluster/identity_lookup.go:36-39`, `StoredActivation` at `cluster/identity_lookup.go:42-45`

**Step 1: Implement TryGetExistingActivation**

```go
func (s *PostgresIdentityStorage) TryGetExistingActivation(ci *cluster.ClusterIdentity) *cluster.StoredActivation {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	key := s.idKey(ci)

	var pidID, pidAddr, memberID string
	err := s.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT pid_id, pid_address, member_id FROM %s WHERE key = $1 AND pid_id != ''`, s.tableName),
		key).Scan(&pidID, &pidAddr, &memberID)
	if err != nil {
		return nil
	}

	return &cluster.StoredActivation{
		Pid:      fmt.Sprintf("%s/%s", pidAddr, pidID),
		MemberID: memberID,
	}
}
```

**Step 2: Implement TryAcquireLock**

Uses `INSERT ... ON CONFLICT DO NOTHING` for atomicity. Returns nil if key already exists.

```go
func (s *PostgresIdentityStorage) TryAcquireLock(ci *cluster.ClusterIdentity) *cluster.SpawnLock {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	lockID := uuid.New().String()
	key := s.idKey(ci)
	expiresAt := time.Now().Add(s.config.LockTTL)

	// Clean up any expired lock first.
	_, _ = s.db.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE key = $1 AND lock_id != '' AND pid_id = '' AND lock_expires_at < NOW()`, s.tableName),
		key)

	// Attempt insert — fails silently if key exists.
	result, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO %s (key, lock_id, lock_expires_at) VALUES ($1, $2, $3) ON CONFLICT (key) DO NOTHING`, s.tableName),
		key, lockID, expiresAt)
	if err != nil {
		slog.Error("Postgres TryAcquireLock failed", slog.String("key", key), slog.Any("error", err))
		return nil
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil
	}

	return &cluster.SpawnLock{
		LockID:          lockID,
		ClusterIdentity: ci,
	}
}
```

**Step 3: Implement StoreActivation**

Conditional on lock ID matching (UPDATE WHERE lock_id = $lockID).

```go
func (s *PostgresIdentityStorage) StoreActivation(memberID string, spawnLock *cluster.SpawnLock, pid *actor.PID) {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	key := s.idKey(spawnLock.ClusterIdentity)

	result, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`UPDATE %s SET pid_id = $1, pid_address = $2, member_id = $3, lock_id = '', lock_expires_at = NULL WHERE key = $4 AND lock_id = $5`, s.tableName),
		pid.Id, pid.Address, memberID, key, spawnLock.LockID)
	if err != nil {
		slog.Error("Postgres StoreActivation failed", slog.String("key", key), slog.Any("error", err))
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		slog.Warn("Postgres StoreActivation lock mismatch",
			slog.String("key", key), slog.String("lockID", spawnLock.LockID))
	}
}
```

**Step 4: Implement WaitForActivation**

Polls with exponential backoff (same pattern as Redis implementation).

```go
func (s *PostgresIdentityStorage) WaitForActivation(ci *cluster.ClusterIdentity) *cluster.StoredActivation {
	ctx := context.Background()
	key := s.idKey(ci)
	deadline := time.Now().Add(s.config.LockTTL)

	// Read initial lock ID.
	s.acquire()
	var initialLockID string
	_ = s.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT lock_id FROM %s WHERE key = $1`, s.tableName), key).Scan(&initialLockID)
	s.release()

	iteration := 1
	for time.Now().Before(deadline) {
		backoff := time.Duration(20*iteration) * time.Millisecond
		if backoff > 500*time.Millisecond {
			backoff = 500 * time.Millisecond
		}
		time.Sleep(backoff)
		iteration++

		s.acquire()
		var lockID, pidID, pidAddr, memberID string
		err := s.db.QueryRowContext(ctx,
			fmt.Sprintf(`SELECT lock_id, pid_id, pid_address, member_id FROM %s WHERE key = $1`, s.tableName),
			key).Scan(&lockID, &pidID, &pidAddr, &memberID)
		s.release()

		if err != nil {
			if initialLockID != "" {
				return nil
			}
			continue
		}

		if lockID == "" && pidID != "" {
			return &cluster.StoredActivation{
				Pid:      fmt.Sprintf("%s/%s", pidAddr, pidID),
				MemberID: memberID,
			}
		}

		if initialLockID == "" {
			initialLockID = lockID
			continue
		}

		if lockID != initialLockID {
			return nil
		}
	}

	// Stale lock cleanup.
	s.RemoveLock(cluster.SpawnLock{LockID: initialLockID, ClusterIdentity: ci})
	return nil
}
```

**Step 5: Implement RemoveLock**

```go
func (s *PostgresIdentityStorage) RemoveLock(spawnLock cluster.SpawnLock) {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	key := s.idKey(spawnLock.ClusterIdentity)

	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE key = $1 AND lock_id = $2`, s.tableName),
		key, spawnLock.LockID)
	if err != nil {
		slog.Error("Postgres RemoveLock failed", slog.String("key", key), slog.Any("error", err))
	}
}
```

**Step 6: Implement RemoveActivation**

```go
func (s *PostgresIdentityStorage) RemoveActivation(spawnLock *cluster.SpawnLock) {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	key := s.idKey(spawnLock.ClusterIdentity)

	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE key = $1`, s.tableName), key)
	if err != nil {
		slog.Error("Postgres RemoveActivation failed", slog.String("key", key), slog.Any("error", err))
	}
}
```

**Step 7: Implement RemoveMemberId**

```go
func (s *PostgresIdentityStorage) RemoveMemberId(memberID string) {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE member_id = $1`, s.tableName), memberID)
	if err != nil {
		slog.Error("Postgres RemoveMemberId failed", slog.String("memberID", memberID), slog.Any("error", err))
	}
}
```

**Step 8: Run conformance tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test -tags integration -v ./cluster/identitylookup/postgres/...`
Expected: ALL PASS

**Step 9: Commit**

```
feat(identity): implement Postgres StorageLookup

Passes full conformance suite. Uses INSERT ON CONFLICT for atomic
locking, standard SQL for all operations.
```

---

### Task 3: NATS JetStream StorageLookup — Failing Tests

**Files:**
- Create: `cluster/identitylookup/nats/nats_identity.go`
- Create: `cluster/identitylookup/nats/options.go`
- Create: `cluster/identitylookup/nats/nats_identity_test.go`

**Context:**
- Same conformance suite as Postgres/Redis
- NATS JetStream KV API: `jetstream.JetStream` → `CreateOrUpdateKeyValue()` → `jetstream.KeyValue`
- KV operations: `Create` (atomic if-not-exists), `Get`, `Put`, `Delete`, `Watch`
- `Create` returns `jetstream.ErrKeyExists` if key already exists — perfect for locking

**Step 1: Create options.go**

```go
// cluster/identitylookup/nats/options.go
package nats

import (
	"time"
)

type Config struct {
	ClusterName    string
	LockTTL        time.Duration
	MaxConcurrency int
}

type Option func(*Config)

func WithLockTTL(ttl time.Duration) Option {
	return func(c *Config) { c.LockTTL = ttl }
}

func WithMaxConcurrency(n int) Option {
	return func(c *Config) { c.MaxConcurrency = n }
}

func defaultConfig(clusterName string) *Config {
	return &Config{
		ClusterName:    clusterName,
		LockTTL:        5 * time.Second,
		MaxConcurrency: 200,
	}
}
```

**Step 2: Create nats_identity.go skeleton**

```go
// cluster/identitylookup/nats/nats_identity.go
package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
)

// activationRecord is the JSON-encoded value stored in the KV bucket.
type activationRecord struct {
	LockID     string `json:"lid,omitempty"`
	PidID      string `json:"pid,omitempty"`
	PidAddress string `json:"adr,omitempty"`
	MemberID   string `json:"mid,omitempty"`
}

// memberRecord tracks which identity keys belong to a member.
type memberRecord struct {
	Keys []string `json:"keys"`
}

type NatsIdentityStorage struct {
	js         jetstream.JetStream
	config     *Config
	identities jetstream.KeyValue // {clusterName}_identities
	members    jetstream.KeyValue // {clusterName}_members
	semaphore  chan struct{}
}

var _ cluster.StorageLookup = (*NatsIdentityStorage)(nil)

func New(clusterName string, js jetstream.JetStream, opts ...Option) (*NatsIdentityStorage, error) {
	cfg := defaultConfig(clusterName)
	for _, opt := range opts {
		opt(cfg)
	}

	ctx := context.Background()

	identities, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: clusterName + "_identities",
		TTL:    0, // no global TTL; per-key TTL via lock expiry
	})
	if err != nil {
		return nil, fmt.Errorf("create identities KV bucket: %w", err)
	}

	members, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: clusterName + "_members",
	})
	if err != nil {
		return nil, fmt.Errorf("create members KV bucket: %w", err)
	}

	return &NatsIdentityStorage{
		js:         js,
		config:     cfg,
		identities: identities,
		members:    members,
		semaphore:  make(chan struct{}, cfg.MaxConcurrency),
	}, nil
}

func (s *NatsIdentityStorage) acquire() { s.semaphore <- struct{}{} }
func (s *NatsIdentityStorage) release() { <-s.semaphore }

func (s *NatsIdentityStorage) idKey(ci *cluster.ClusterIdentity) string {
	// NATS KV keys cannot contain '/' so use '.' separator
	return ci.Kind + "." + ci.Identity
}
```

Stub all interface methods to panic with "not implemented".

**Step 3: Create test file**

```go
// cluster/identitylookup/nats/nats_identity_test.go
//go:build integration

package nats_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/identitylookup"
	natsidentity "github.com/asynkron/protoactor-go/cluster/identitylookup/nats"
)

var testJS jetstream.JetStream
var testNC *nats.Conn

func TestMain(m *testing.M) {
	natsURL := os.Getenv("NATS_URL")

	if natsURL == "" {
		ctx := context.Background()
		container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        "nats:2-alpine",
				ExposedPorts: []string{"4222/tcp"},
				Cmd:          []string{"-js"},
				WaitingFor:   wait.ForLog("Server is ready").WithStartupTimeout(30 * time.Second),
			},
			Started: true,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to start nats container: %v\n", err)
			os.Exit(1)
		}

		host, _ := container.Host(ctx)
		port, _ := container.MappedPort(ctx, "4222")
		natsURL = fmt.Sprintf("nats://%s:%s", host, port.Port())

		defer func() { _ = container.Terminate(ctx) }()
	}

	var err error
	testNC, err = nats.Connect(natsURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to nats: %v\n", err)
		os.Exit(1)
	}

	testJS, err = jetstream.New(testNC)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create jetstream context: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func TestNatsConformance(t *testing.T) {
	suite := &identitylookup.StorageConformanceSuite{
		NewStorage: func() cluster.StorageLookup {
			s, err := natsidentity.New("test-cluster", testJS)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			return s
		},
		Cleanup: func() {
			ctx := context.Background()
			// Delete and recreate the buckets.
			_ = testJS.DeleteKeyValue(ctx, "test-cluster_identities")
			_ = testJS.DeleteKeyValue(ctx, "test-cluster_members")
		},
	}
	suite.RunAll(t)
}
```

**Step 4: Add nats.go dependency**

Run: `cd /home/cchamplin/development/protoactor-go && go get github.com/nats-io/nats.go`

**Step 5: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/protoactor-go && go test -tags integration -v ./cluster/identitylookup/nats/...`
Expected: FAIL

**Step 6: Commit**

```
feat(identity): add NATS JetStream StorageLookup test scaffold

Conformance test using testcontainers; stubs not yet implemented.
```

---

### Task 4: NATS JetStream StorageLookup — Implementation

**Files:**
- Modify: `cluster/identitylookup/nats/nats_identity.go`

**Context:**
- NATS KV `Create(key, value)` fails with `jetstream.ErrKeyExists` if key exists — use for locking
- NATS KV `Update(key, value, revision)` provides CAS semantics via revision number
- NATS KV `Watch(key)` provides event-driven notifications (not polling)
- KV keys cannot contain `/` — use `.` separator for `{kind}.{identity}`

**Step 1: Implement TryGetExistingActivation**

```go
func (s *NatsIdentityStorage) TryGetExistingActivation(ci *cluster.ClusterIdentity) *cluster.StoredActivation {
	s.acquire()
	defer s.release()

	entry, err := s.identities.Get(context.Background(), s.idKey(ci))
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
```

**Step 2: Implement TryAcquireLock**

```go
func (s *NatsIdentityStorage) TryAcquireLock(ci *cluster.ClusterIdentity) *cluster.SpawnLock {
	s.acquire()
	defer s.release()

	lockID := uuid.New().String()
	key := s.idKey(ci)

	rec := activationRecord{LockID: lockID}
	data, _ := json.Marshal(rec)

	// Create fails if key already exists.
	_, err := s.identities.Create(context.Background(), key, data)
	if err != nil {
		return nil
	}

	return &cluster.SpawnLock{
		LockID:          lockID,
		ClusterIdentity: ci,
	}
}
```

**Step 3: Implement StoreActivation**

```go
func (s *NatsIdentityStorage) StoreActivation(memberID string, spawnLock *cluster.SpawnLock, pid *actor.PID) {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	key := s.idKey(spawnLock.ClusterIdentity)

	// Read current entry to verify lock and get revision.
	entry, err := s.identities.Get(ctx, key)
	if err != nil {
		slog.Error("NATS StoreActivation Get failed", slog.String("key", key), slog.Any("error", err))
		return
	}

	var rec activationRecord
	if err := json.Unmarshal(entry.Value(), &rec); err != nil {
		slog.Error("NATS StoreActivation unmarshal failed", slog.String("key", key), slog.Any("error", err))
		return
	}

	if rec.LockID != spawnLock.LockID {
		slog.Warn("NATS StoreActivation lock mismatch",
			slog.String("key", key), slog.String("lockID", spawnLock.LockID))
		return
	}

	// Update with CAS using revision.
	rec.LockID = ""
	rec.PidID = pid.Id
	rec.PidAddress = pid.Address
	rec.MemberID = memberID
	data, _ := json.Marshal(rec)

	_, err = s.identities.Update(ctx, key, data, entry.Revision())
	if err != nil {
		slog.Error("NATS StoreActivation Update failed", slog.String("key", key), slog.Any("error", err))
		return
	}

	// Add to member tracking.
	s.addToMember(ctx, memberID, key)
}

func (s *NatsIdentityStorage) addToMember(ctx context.Context, memberID, identityKey string) {
	// Read existing member record, append key, write back.
	var rec memberRecord
	entry, err := s.members.Get(ctx, memberID)
	if err == nil {
		_ = json.Unmarshal(entry.Value(), &rec)
	}

	rec.Keys = append(rec.Keys, identityKey)
	data, _ := json.Marshal(rec)

	if entry != nil {
		_, _ = s.members.Update(ctx, memberID, data, entry.Revision())
	} else {
		_, _ = s.members.Create(ctx, memberID, data)
	}
}
```

**Step 4: Implement WaitForActivation**

Uses KV Watch for event-driven notification instead of polling.

```go
func (s *NatsIdentityStorage) WaitForActivation(ci *cluster.ClusterIdentity) *cluster.StoredActivation {
	ctx, cancel := context.WithTimeout(context.Background(), s.config.LockTTL)
	defer cancel()

	key := s.idKey(ci)

	watcher, err := s.identities.Watch(ctx, key)
	if err != nil {
		return nil
	}
	defer func() { _ = watcher.Stop() }()

	for {
		select {
		case entry := <-watcher.Updates():
			if entry == nil {
				// Watcher initialization complete, continue waiting.
				continue
			}

			if entry.Operation() == jetstream.KeyValueDelete {
				return nil
			}

			var rec activationRecord
			if err := json.Unmarshal(entry.Value(), &rec); err != nil {
				continue
			}

			if rec.LockID == "" && rec.PidID != "" {
				return &cluster.StoredActivation{
					Pid:      fmt.Sprintf("%s/%s", rec.PidAddress, rec.PidID),
					MemberID: rec.MemberID,
				}
			}
		case <-ctx.Done():
			return nil
		}
	}
}
```

**Step 5: Implement RemoveLock**

```go
func (s *NatsIdentityStorage) RemoveLock(spawnLock cluster.SpawnLock) {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	key := s.idKey(spawnLock.ClusterIdentity)

	entry, err := s.identities.Get(ctx, key)
	if err != nil {
		return
	}

	var rec activationRecord
	if err := json.Unmarshal(entry.Value(), &rec); err != nil {
		return
	}

	if rec.LockID != spawnLock.LockID {
		return
	}

	_ = s.identities.Delete(ctx, key)
}
```

**Step 6: Implement RemoveActivation**

```go
func (s *NatsIdentityStorage) RemoveActivation(spawnLock *cluster.SpawnLock) {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	key := s.idKey(spawnLock.ClusterIdentity)

	// Read to find member ID for cleanup.
	entry, err := s.identities.Get(ctx, key)
	if err != nil {
		return
	}

	var rec activationRecord
	if err := json.Unmarshal(entry.Value(), &rec); err == nil && rec.MemberID != "" {
		s.removeFromMember(ctx, rec.MemberID, key)
	}

	_ = s.identities.Delete(ctx, key)
}

func (s *NatsIdentityStorage) removeFromMember(ctx context.Context, memberID, identityKey string) {
	entry, err := s.members.Get(ctx, memberID)
	if err != nil {
		return
	}

	var rec memberRecord
	if err := json.Unmarshal(entry.Value(), &rec); err != nil {
		return
	}

	filtered := rec.Keys[:0]
	for _, k := range rec.Keys {
		if k != identityKey {
			filtered = append(filtered, k)
		}
	}
	rec.Keys = filtered

	data, _ := json.Marshal(rec)
	_, _ = s.members.Update(ctx, memberID, data, entry.Revision())
}
```

**Step 7: Implement RemoveMemberId**

```go
func (s *NatsIdentityStorage) RemoveMemberId(memberID string) {
	s.acquire()
	defer s.release()

	ctx := context.Background()

	entry, err := s.members.Get(ctx, memberID)
	if err != nil {
		return
	}

	var rec memberRecord
	if err := json.Unmarshal(entry.Value(), &rec); err != nil {
		slog.Error("NATS RemoveMemberId unmarshal failed", slog.String("memberID", memberID), slog.Any("error", err))
		return
	}

	for _, key := range rec.Keys {
		_ = s.identities.Delete(ctx, key)
	}

	_ = s.members.Delete(ctx, memberID)
}
```

**Step 8: Run conformance tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test -tags integration -v ./cluster/identitylookup/nats/...`
Expected: ALL PASS

**Step 9: Commit**

```
feat(identity): implement NATS JetStream StorageLookup

Uses JetStream KV Create for atomic locking, Watch for
event-driven WaitForActivation. Passes conformance suite.
```

---

### Task 5: Example 1 — NATS Virtual Actor Ingress

**Files:**
- Create: `examples/nats-virtual-actor-ingress/docker-compose.yml`
- Create: `examples/nats-virtual-actor-ingress/go.mod`
- Create: `examples/nats-virtual-actor-ingress/shared/protos.proto`
- Create: `examples/nats-virtual-actor-ingress/shared/build.sh`
- Create: `examples/nats-virtual-actor-ingress/node/main.go`
- Create: `examples/nats-virtual-actor-ingress/ingress/main.go`

**Context:**
- Reference: C# Kafka example at `agent-vendored/protoactor-dotnet/examples/cluster.kafka-virtual-actor-ingress/Program.cs`
- Go grain pattern: `examples/cluster-grain/` (node1=client, node2=host, shared=proto)
- Go module pattern: `replace github.com/asynkron/protoactor-go => ../..` in go.mod
- Protoc build: `protoc --go_out=. --go_opt=paths=source_relative --plugin=protoc-gen-go-grain=../../../protobuf/protoc-gen-go-grain/protoc-gen-go-grain.sh --go-grain_out=. --go-grain_opt=paths=source_relative -I../../ -I. protos.proto`

**Step 1: Create docker-compose.yml**

```yaml
version: "3"
services:
  nats:
    image: nats:2-alpine
    ports:
      - "4222:4222"
    command: ["-js"]

  consul:
    image: hashicorp/consul:1.19
    ports:
      - "8500:8500"
    command: ["agent", "-dev", "-client=0.0.0.0"]
```

**Step 2: Create proto file**

```protobuf
// examples/nats-virtual-actor-ingress/shared/protos.proto
syntax = "proto3";
package shared;
option go_package = "github.com/asynkron/protoactor-go/examples/nats-virtual-actor-ingress/shared";

message DeviceMessage {
  string data = 1;
}

message Ack {}

message DeviceState {
  string data = 1;
  int32 message_count = 2;
}

service Device {
  rpc HandleMessage (DeviceMessage) returns (Ack) {}
}
```

**Step 3: Create build.sh and generate proto**

```bash
#!/bin/bash
protoc --go_out=. --go_opt=paths=source_relative \
    --plugin=protoc-gen-go-grain=../../../protobuf/protoc-gen-go-grain/protoc-gen-go-grain.sh \
    --go-grain_out=. --go-grain_opt=paths=source_relative \
    -I../../ -I. protos.proto
```

Run: `cd examples/nats-virtual-actor-ingress/shared && chmod +x build.sh && ./build.sh`

**Step 4: Create go.mod**

```
module nats-virtual-actor-ingress

go 1.25.3

replace github.com/asynkron/protoactor-go => ../..

require (
	github.com/asynkron/protoactor-go v0.0.0
	github.com/nats-io/nats.go v1.39.1
	google.golang.org/protobuf v1.36.10
)
```

Then run `go mod tidy` to resolve all transitive dependencies.

**Step 5: Create node/main.go**

```go
package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"nats-virtual-actor-ingress/shared"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/consul"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/remote"
)

type DeviceGrain struct {
	state shared.DeviceState
}

func (d *DeviceGrain) Init(ctx cluster.GrainContext)           {}
func (d *DeviceGrain) Terminate(ctx cluster.GrainContext)      {}
func (d *DeviceGrain) ReceiveDefault(ctx cluster.GrainContext) {}

func (d *DeviceGrain) HandleMessage(msg *shared.DeviceMessage, ctx cluster.GrainContext) (*shared.Ack, error) {
	d.state.MessageCount++
	d.state.Data = msg.Data
	ctx.Logger().Info("HandleMessage",
		slog.String("identity", ctx.Identity()),
		slog.String("data", msg.Data),
		slog.Int("count", int(d.state.MessageCount)))
	return &shared.Ack{}, nil
}

func main() {
	system := actor.NewActorSystem()
	provider, err := consul.New()
	if err != nil {
		log.Fatal(err)
	}
	lookup := disthash.New()
	remoteConfig := remote.Configure("localhost", 0)

	deviceKind := shared.NewDeviceKind(func() shared.Device {
		return &DeviceGrain{}
	}, 0)

	clusterConfig := cluster.Configure("nats-ingress-example", provider, lookup, remoteConfig,
		cluster.WithKinds(deviceKind))

	c := cluster.NewCluster(system, clusterConfig)
	if err := c.StartMember(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Node started. Press Ctrl+C to stop.")
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	c.Shutdown(true)
}
```

**Step 6: Create ingress/main.go**

```go
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"nats-virtual-actor-ingress/shared"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/consul"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/nats-io/nats.go"
)

func main() {
	// Connect to NATS.
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	// Start cluster client.
	system := actor.NewActorSystem()
	provider, err := consul.New()
	if err != nil {
		log.Fatal(err)
	}
	lookup := disthash.New()
	remoteConfig := remote.Configure("localhost", 0)
	clusterConfig := cluster.Configure("nats-ingress-example", provider, lookup, remoteConfig)

	c := cluster.NewCluster(system, clusterConfig)
	if err := c.StartClient(); err != nil {
		log.Fatal(err)
	}

	// Start publishing simulated device messages.
	go publishSimulatedMessages(nc)

	// Subscribe to devices.> and forward to virtual actors.
	sub, err := nc.Subscribe("devices.>", func(msg *nats.Msg) {
		// Extract device ID from subject: devices.{deviceID}
		parts := strings.SplitN(msg.Subject, ".", 2)
		if len(parts) < 2 {
			return
		}
		deviceID := parts[1]

		client := shared.GetDeviceGrainClient(c, deviceID)
		resp, err := client.HandleMessage(&shared.DeviceMessage{
			Data: string(msg.Data),
		})
		if err != nil {
			fmt.Printf("Error forwarding to device %s: %v\n", deviceID, err)
			return
		}
		_ = resp
	})
	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	fmt.Println("Ingress started. Forwarding NATS messages to virtual actors...")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	c.Shutdown(true)
}

func publishSimulatedMessages(nc *nats.Conn) {
	time.Sleep(2 * time.Second) // Wait for cluster to stabilize.

	for i := 0; ; i++ {
		deviceID := fmt.Sprintf("device-%d", rand.Intn(100))
		data := fmt.Sprintf("reading-%d", i)
		_ = nc.Publish("devices."+deviceID, []byte(data))

		time.Sleep(100 * time.Millisecond)
	}
}
```

**Step 7: Run go mod tidy**

Run: `cd examples/nats-virtual-actor-ingress && go mod tidy`

**Step 8: Verify compilation**

Run: `cd examples/nats-virtual-actor-ingress && go build ./...`
Expected: Builds without errors

**Step 9: Commit**

```
feat(examples): add NATS virtual actor ingress example

Demonstrates NATS core subscriber routing messages to virtual actors
using subject-based device identity mapping.
```

---

### Task 6: Example 2 — NATS JetStream Virtual Actor Ingress

**Files:**
- Create: `examples/nats-jetstream-virtual-actor-ingress/docker-compose.yml`
- Create: `examples/nats-jetstream-virtual-actor-ingress/go.mod`
- Create: `examples/nats-jetstream-virtual-actor-ingress/shared/protos.proto` (same Device service)
- Create: `examples/nats-jetstream-virtual-actor-ingress/shared/build.sh`
- Create: `examples/nats-jetstream-virtual-actor-ingress/node/main.go`
- Create: `examples/nats-jetstream-virtual-actor-ingress/ingress/main.go`

**Context:**
- Same proto as Example 1
- Key difference: uses JetStream pull consumer with batch fetch and ack-based flow control
- Reference: C# Kafka example's `RunKafkaConsumeLoop` pattern with batch→fan-out→ack

**Step 1: Create docker-compose.yml** (same as Example 1)

**Step 2: Create shared proto + build.sh** (same as Example 1)

**Step 3: Create go.mod** (same pattern, different module name: `nats-jetstream-virtual-actor-ingress`)

**Step 4: Create node/main.go** (same as Example 1 — identical DeviceGrain)

**Step 5: Create ingress/main.go**

```go
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"nats-jetstream-virtual-actor-ingress/shared"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/consul"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	streamName   = "DEVICES"
	consumerName = "device-ingress"
	batchSize    = 50
)

func main() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		log.Fatalf("Failed to create JetStream context: %v", err)
	}

	ctx := context.Background()

	// Create/ensure stream.
	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     streamName,
		Subjects: []string{"devices.>"},
	})
	if err != nil {
		log.Fatalf("Failed to create stream: %v", err)
	}

	// Create durable pull consumer.
	consumer, err := js.CreateOrUpdateConsumer(ctx, streamName, jetstream.ConsumerConfig{
		Durable:   consumerName,
		AckPolicy: jetstream.AckExplicitPolicy,
	})
	if err != nil {
		log.Fatalf("Failed to create consumer: %v", err)
	}

	// Start cluster client.
	system := actor.NewActorSystem()
	provider, err := consul.New()
	if err != nil {
		log.Fatal(err)
	}
	lookup := disthash.New()
	remoteConfig := remote.Configure("localhost", 0)
	clusterConfig := cluster.Configure("nats-js-ingress-example", provider, lookup, remoteConfig)

	c := cluster.NewCluster(system, clusterConfig)
	if err := c.StartClient(); err != nil {
		log.Fatal(err)
	}

	// Start publishing simulated device messages.
	go publishSimulatedMessages(nc)

	// Consume loop.
	done := make(chan struct{})
	go func() {
		defer close(done)
		runConsumeLoop(ctx, consumer, c)
	}()

	fmt.Println("JetStream ingress started. Fetching batches and forwarding to virtual actors...")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	c.Shutdown(true)
}

func runConsumeLoop(ctx context.Context, consumer jetstream.Consumer, c *cluster.Cluster) {
	for {
		batch, err := consumer.Fetch(batchSize, jetstream.FetchMaxWait(2*time.Second))
		if err != nil {
			continue
		}

		start := time.Now()
		var msgs []jetstream.Msg
		var wg sync.WaitGroup
		var mu sync.Mutex
		allOK := true

		for msg := range batch.Messages() {
			msgs = append(msgs, msg)
			wg.Add(1)

			go func(m jetstream.Msg) {
				defer wg.Done()

				parts := strings.SplitN(m.Subject(), ".", 2)
				if len(parts) < 2 {
					return
				}
				deviceID := parts[1]

				client := shared.GetDeviceGrainClient(c, deviceID)
				_, err := client.HandleMessage(&shared.DeviceMessage{
					Data: string(m.Data()),
				})
				if err != nil {
					mu.Lock()
					allOK = false
					mu.Unlock()
					fmt.Printf("Error: device %s: %v\n", deviceID, err)
				}
			}(msg)
		}

		wg.Wait()

		// Only ack if all succeeded — at-least-once semantics.
		if allOK {
			for _, m := range msgs {
				_ = m.Ack()
			}
		}

		elapsed := time.Since(start)
		if len(msgs) > 0 {
			tps := float64(len(msgs)) / elapsed.Seconds()
			fmt.Printf("Batch: %d msgs in %v (%.0f msgs/sec)\n", len(msgs), elapsed, tps)
		}
	}
}

func publishSimulatedMessages(nc *nats.Conn) {
	time.Sleep(2 * time.Second)
	for i := 0; ; i++ {
		deviceID := fmt.Sprintf("device-%d", rand.Intn(100))
		data := fmt.Sprintf("reading-%d", i)
		_ = nc.Publish("devices."+deviceID, []byte(data))
		time.Sleep(50 * time.Millisecond)
	}
}
```

**Step 6: Run go mod tidy and verify**

Run: `cd examples/nats-jetstream-virtual-actor-ingress && go mod tidy && go build ./...`

**Step 7: Commit**

```
feat(examples): add NATS JetStream virtual actor ingress example

Demonstrates JetStream pull consumer with batch fetch, fan-out to
virtual actors, and ack-based at-least-once delivery semantics.
```

---

### Task 7: Example 3 — NATS Forwarder

**Files:**
- Create: `examples/nats-forwarder/docker-compose.yml`
- Create: `examples/nats-forwarder/go.mod`
- Create: `examples/nats-forwarder/shared/protos.proto`
- Create: `examples/nats-forwarder/shared/build.sh`
- Create: `examples/nats-forwarder/forwarder/main.go`
- Create: `examples/nats-forwarder/worker/main.go`
- Create: `examples/nats-forwarder/publisher/main.go`

**Context:**
- Not grain-based — uses regular actors resolved by name
- Subject routing: `commands.{workerName}.{action}` maps to actor
- Bidirectional: results published back to NATS
- NATS request-reply for synchronous command/response

**Step 1: Create docker-compose.yml** (NATS + Consul)

**Step 2: Create proto**

```protobuf
syntax = "proto3";
package shared;
option go_package = "github.com/asynkron/protoactor-go/examples/nats-forwarder/shared";

message Command {
  string action = 1;
  bytes payload = 2;
  string reply_subject = 3;
}

message CommandResult {
  string worker = 1;
  string action = 2;
  bool success = 3;
  string result = 4;
}
```

Note: This example does NOT need protoc-gen-go-grain since it uses regular actors, not grains. Only need standard protoc for message types.

**Step 3: Create build.sh**

```bash
#!/bin/bash
protoc --go_out=. --go_opt=paths=source_relative -I. protos.proto
```

**Step 4: Create worker/main.go**

```go
package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"nats-forwarder/shared"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/consul"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/remote"
)

type WorkerActor struct {
	name string
}

func (w *WorkerActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *shared.Command:
		ctx.Logger().Info("Processing command",
			slog.String("worker", w.name),
			slog.String("action", msg.Action))

		result := &shared.CommandResult{
			Worker:  w.name,
			Action:  msg.Action,
			Success: true,
			Result:  fmt.Sprintf("Processed by %s: %s", w.name, msg.Action),
		}
		ctx.Respond(result)
	}
}

func main() {
	workerName := "worker-1"
	if len(os.Args) > 1 {
		workerName = os.Args[1]
	}

	system := actor.NewActorSystem()
	provider, err := consul.New()
	if err != nil {
		log.Fatal(err)
	}
	lookup := disthash.New()
	remoteConfig := remote.Configure("localhost", 0)

	workerKind := cluster.NewKind(workerName, actor.PropsFromProducer(func() actor.Actor {
		return &WorkerActor{name: workerName}
	}))

	clusterConfig := cluster.Configure("nats-forwarder-example", provider, lookup, remoteConfig,
		cluster.WithKinds(workerKind))

	c := cluster.NewCluster(system, clusterConfig)
	if err := c.StartMember(); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Worker %s started. Press Ctrl+C to stop.\n", workerName)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	c.Shutdown(true)
}
```

**Step 5: Create forwarder/main.go**

```go
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"nats-forwarder/shared"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/consul"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

func main() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	// Start cluster client.
	system := actor.NewActorSystem()
	provider, err := consul.New()
	if err != nil {
		log.Fatal(err)
	}
	lookup := disthash.New()
	remoteConfig := remote.Configure("localhost", 0)
	clusterConfig := cluster.Configure("nats-forwarder-example", provider, lookup, remoteConfig)

	c := cluster.NewCluster(system, clusterConfig)
	if err := c.StartClient(); err != nil {
		log.Fatal(err)
	}

	// Subscribe to commands.> and forward to actors.
	sub, err := nc.Subscribe("commands.>", func(msg *nats.Msg) {
		// Parse subject: commands.{workerName}.{action}
		parts := strings.SplitN(msg.Subject, ".", 3)
		if len(parts) < 3 {
			return
		}
		workerName := parts[1]
		action := parts[2]

		cmd := &shared.Command{
			Action:       action,
			Payload:      msg.Data,
			ReplySubject: msg.Reply,
		}

		// Forward to the worker via cluster.
		resp, err := c.Request(workerName, workerName, cmd)
		if err != nil {
			fmt.Printf("Error forwarding to %s: %v\n", workerName, err)
			return
		}

		// If there's a reply subject (NATS request-reply), send result back.
		if msg.Reply != "" {
			if result, ok := resp.(*shared.CommandResult); ok {
				data, _ := proto.Marshal(result)
				_ = nc.Publish(msg.Reply, data)
			}
		}

		// Also publish to results subject.
		if result, ok := resp.(*shared.CommandResult); ok {
			data, _ := proto.Marshal(result)
			_ = nc.Publish("results."+workerName, data)
		}
	})
	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	fmt.Println("Forwarder started. Routing NATS commands to actors...")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	c.Shutdown(true)
}
```

**Step 6: Create publisher/main.go**

```go
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

func main() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	workerName := "worker-1"
	if len(os.Args) > 1 {
		workerName = os.Args[1]
	}

	actions := []string{"process", "validate", "transform", "aggregate"}

	for i := 0; i < 20; i++ {
		action := actions[i%len(actions)]
		subject := fmt.Sprintf("commands.%s.%s", workerName, action)

		// Use NATS request-reply for synchronous response.
		msg, err := nc.Request(subject, []byte(fmt.Sprintf("payload-%d", i)), 5*time.Second)
		if err != nil {
			fmt.Printf("Request failed: %v\n", err)
			continue
		}
		fmt.Printf("Response from %s: %d bytes\n", subject, len(msg.Data))
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("Publisher done.")
}
```

**Step 7: Build and verify**

Run: `cd examples/nats-forwarder && go mod tidy && go build ./...`

**Step 8: Commit**

```
feat(examples): add NATS forwarder example

Demonstrates subject-based routing from NATS to actors with
bidirectional communication and request-reply pattern.
```

---

### Task 8: Example 4 — NATS Stream Locality

**Files:**
- Create: `examples/nats-stream-locality/docker-compose.yml`
- Create: `examples/nats-stream-locality/go.mod`
- Create: `examples/nats-stream-locality/shared/protos.proto`
- Create: `examples/nats-stream-locality/shared/build.sh`
- Create: `examples/nats-stream-locality/shared/affinity.go`
- Create: `examples/nats-stream-locality/node/main.go`
- Create: `examples/nats-stream-locality/publisher/main.go`

**Context:**
- `MemberStrategy` interface at `cluster/member_strategy.go:4-10`
- `Kind.WithMemberStrategy` at `cluster/kind.go:27-29`
- Gossip `SetState`/`GetState` at `cluster/gossiper.go:76-112`
- Uses `Rendezvous` for fallback hashing at `cluster/rendezvous.go`
- Member struct at `cluster/cluster.proto:107-112` (host, port, id, kinds — no metadata)

The affinity strategy uses gossip to propagate subject bindings since the Member proto has no metadata field.

**Step 1: Create docker-compose.yml** (NATS with JetStream + Consul)

**Step 2: Create proto**

```protobuf
syntax = "proto3";
package shared;
option go_package = "github.com/asynkron/protoactor-go/examples/nats-stream-locality/shared";

message SensorReading {
  string sensor_id = 1;
  string subject = 2;
  double value = 3;
  int64 timestamp = 4;
}

message Ack {}

service Sensor {
  rpc HandleReading (SensorReading) returns (Ack) {}
}
```

**Step 3: Create shared/affinity.go**

```go
package shared

import (
	"log/slog"
	"math/rand"
	"strings"

	"github.com/asynkron/protoactor-go/cluster"
)

const SubjectBindingsKey = "nats-subject-bindings"

// SubjectBindings is the gossip state value for a member's bound subjects.
// Stored as a comma-separated string in gossip.
type SubjectBindings struct {
	Subjects []string
}

// LocalAffinityStrategy implements cluster.MemberStrategy.
// It prefers members whose bound NATS subjects match the actor identity's
// subject prefix. Falls back to rendezvous hash on ties or no match.
type LocalAffinityStrategy struct {
	cluster *cluster.Cluster
	members cluster.Members
	rdv     *cluster.Rendezvous

	// subjectsByMember maps member address → list of bound subject patterns
	subjectsByMember map[string][]string
}

func NewLocalAffinityStrategy(c *cluster.Cluster) cluster.MemberStrategy {
	return &LocalAffinityStrategy{
		cluster:          c,
		members:          make(cluster.Members, 0),
		rdv:              cluster.NewRendezvous(),
		subjectsByMember: make(map[string][]string),
	}
}

func (s *LocalAffinityStrategy) GetAllMembers() cluster.Members {
	return s.members
}

func (s *LocalAffinityStrategy) AddMember(member *cluster.Member) {
	s.members = append(s.members, member)
	s.rdv.UpdateMembers(s.members)
}

func (s *LocalAffinityStrategy) RemoveMember(member *cluster.Member) {
	for i, m := range s.members {
		if m.Address() == member.Address() {
			s.members = append(s.members[:i], s.members[i+1:]...)
			s.rdv.UpdateMembers(s.members)
			delete(s.subjectsByMember, m.Address())
			return
		}
	}
}

// GetPartition selects the member for a given identity key.
// The identity key format is "kind/identity" where identity encodes the
// subject (e.g., "Sensor/sensors.building-a.floor-1.temp").
func (s *LocalAffinityStrategy) GetPartition(key string) string {
	// Refresh subject bindings from gossip.
	s.refreshBindings()

	// Extract the identity part (after "kind/")
	parts := strings.SplitN(key, "/", 2)
	identity := ""
	if len(parts) == 2 {
		identity = parts[1]
	}

	// Score each member by subject match.
	var bestMember *cluster.Member
	bestScore := -1

	for _, member := range s.members {
		subjects := s.subjectsByMember[member.Address()]
		score := matchScore(identity, subjects)
		if score > bestScore {
			bestScore = score
			bestMember = member
		}
	}

	if bestMember != nil && bestScore > 0 {
		return bestMember.Address()
	}

	// Fall back to rendezvous hash.
	return s.rdv.GetByIdentity(key)
}

func (s *LocalAffinityStrategy) GetActivator(senderAddress string) string {
	if len(s.members) == 0 {
		return ""
	}
	return s.members[rand.Intn(len(s.members))].Address()
}

func (s *LocalAffinityStrategy) refreshBindings() {
	state, err := s.cluster.Gossip.GetState(SubjectBindingsKey)
	if err != nil {
		return
	}

	for memberID, kv := range state {
		// Find the member address for this memberID.
		for _, m := range s.members {
			if m.Id == memberID {
				// Parse the gossip value (wrapperspb.StringValue containing comma-separated subjects).
				if kv.Value != nil {
					var sv wrapperspb.StringValue
					if err := kv.Value.UnmarshalTo(&sv); err == nil {
						s.subjectsByMember[m.Address()] = strings.Split(sv.Value, ",")
					}
				}
			}
		}
	}
}

// matchScore returns how well an identity matches the given subject patterns.
// Uses NATS-style wildcard matching (> for multi-level, * for single level).
func matchScore(identity string, patterns []string) int {
	for _, pattern := range patterns {
		// Strip trailing .> for prefix matching.
		prefix := strings.TrimSuffix(pattern, ".>")
		if strings.HasPrefix(identity, prefix) {
			return len(prefix) // Longer prefix = better match
		}
	}
	return 0
}
```

Note: The exact import for `wrapperspb` will need to be adjusted. The implementation may use a custom proto message or `wrapperspb.StringValue` depending on what works best with the gossip system. This will be refined during implementation.

**Step 4: Create node/main.go**

```go
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"nats-stream-locality/shared"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/consul"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var subjects = flag.String("subjects", "sensors.>", "Comma-separated NATS subject patterns to bind to")

type SensorGrain struct {
	readingCount int
}

func (s *SensorGrain) Init(ctx cluster.GrainContext)           {}
func (s *SensorGrain) Terminate(ctx cluster.GrainContext)      {}
func (s *SensorGrain) ReceiveDefault(ctx cluster.GrainContext) {}

func (s *SensorGrain) HandleReading(msg *shared.SensorReading, ctx cluster.GrainContext) (*shared.Ack, error) {
	s.readingCount++
	ctx.Logger().Info("HandleReading",
		slog.String("identity", ctx.Identity()),
		slog.String("subject", msg.Subject),
		slog.Float64("value", msg.Value),
		slog.Int("total", s.readingCount))
	return &shared.Ack{}, nil
}

func main() {
	flag.Parse()

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		log.Fatalf("Failed to create JetStream: %v", err)
	}

	ctx := context.Background()

	// Create/ensure stream.
	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     "SENSORS",
		Subjects: []string{"sensors.>"},
	})
	if err != nil {
		log.Fatalf("Failed to create stream: %v", err)
	}

	// Start cluster.
	system := actor.NewActorSystem()
	provider, err := consul.New()
	if err != nil {
		log.Fatal(err)
	}
	lookup := disthash.New()
	remoteConfig := remote.Configure("localhost", 0)

	sensorKind := shared.NewSensorKind(func() shared.Sensor {
		return &SensorGrain{}
	}, 0)

	// Set custom affinity strategy for the sensor kind.
	sensorKind.WithMemberStrategy(func(c *cluster.Cluster) cluster.MemberStrategy {
		return shared.NewLocalAffinityStrategy(c)
	})

	clusterConfig := cluster.Configure("nats-locality-example", provider, lookup, remoteConfig,
		cluster.WithKinds(sensorKind))

	c := cluster.NewCluster(system, clusterConfig)
	if err := c.StartMember(); err != nil {
		log.Fatal(err)
	}

	// Advertise our bound subjects via gossip.
	c.Gossip.SetState(shared.SubjectBindingsKey, wrapperspb.String(*subjects))

	subjectList := strings.Split(*subjects, ",")
	fmt.Printf("Node started with subjects: %v\n", subjectList)

	// Create a filtered consumer for our subjects.
	consumerName := fmt.Sprintf("node-%s", system.ID)
	consumer, err := js.CreateOrUpdateConsumer(ctx, "SENSORS", jetstream.ConsumerConfig{
		Durable:        consumerName,
		AckPolicy:      jetstream.AckExplicitPolicy,
		FilterSubjects: subjectList,
	})
	if err != nil {
		log.Fatalf("Failed to create consumer: %v", err)
	}

	// Consume and forward to grains.
	go func() {
		for {
			batch, err := consumer.Fetch(20, jetstream.FetchMaxWait(2*time.Second))
			if err != nil {
				continue
			}

			for msg := range batch.Messages() {
				// Use subject as sensor identity for locality.
				sensorID := msg.Subject()

				client := shared.GetSensorGrainClient(c, sensorID)
				_, err := client.HandleReading(&shared.SensorReading{
					SensorId:  sensorID,
					Subject:   msg.Subject(),
					Value:     0, // Would parse from msg.Data() in production
					Timestamp: time.Now().UnixMilli(),
				})
				if err != nil {
					fmt.Printf("Error: %v\n", err)
				}
				_ = msg.Ack()
			}
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	c.Shutdown(true)
}
```

**Step 5: Create publisher/main.go**

```go
package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

func main() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	buildings := []string{"building-a", "building-b"}
	floors := []string{"floor-1", "floor-2", "floor-3"}
	types := []string{"temp", "humidity", "pressure"}

	for i := 0; i < 200; i++ {
		building := buildings[rand.Intn(len(buildings))]
		floor := floors[rand.Intn(len(floors))]
		sensorType := types[rand.Intn(len(types))]

		subject := fmt.Sprintf("sensors.%s.%s.%s", building, floor, sensorType)
		data := fmt.Sprintf("%.2f", rand.Float64()*100)

		_ = nc.Publish(subject, []byte(data))
		fmt.Printf("Published: %s = %s\n", subject, data)

		time.Sleep(50 * time.Millisecond)
	}

	fmt.Println("Publisher done.")
}
```

**Step 6: Build and verify**

Run: `cd examples/nats-stream-locality && go mod tidy && go build ./...`

**Step 7: Commit**

```
feat(examples): add NATS stream locality example

Demonstrates local affinity placement where cluster nodes bind to
NATS subject patterns and actors preferentially spawn on matching
nodes via a custom MemberStrategy.
```

---

### Task 9: Example 5 — Duplicate Activation Prevention

**Files:**
- Create: `examples/nats-identity-prevention/docker-compose.yml`
- Create: `examples/nats-identity-prevention/go.mod`
- Create: `examples/nats-identity-prevention/shared/protos.proto`
- Create: `examples/nats-identity-prevention/shared/build.sh`
- Create: `examples/nats-identity-prevention/postgres-node/main.go`
- Create: `examples/nats-identity-prevention/nats-node/main.go`
- Create: `examples/nats-identity-prevention/client/main.go`

**Context:**
- `IdentityStorageLookup` adapter at `cluster/identitylookup/storage/identity_storage_lookup.go`
- Wraps any `StorageLookup` into an `IdentityLookup`
- Uses: `storage.New(postgres.New(...))` or `storage.New(nats.New(...))`
- Key demo: concurrent requests to same identity → only one activation

**Step 1: Create docker-compose.yml**

```yaml
version: "3"
services:
  nats:
    image: nats:2-alpine
    ports:
      - "4222:4222"
    command: ["-js"]

  postgres:
    image: postgres:16-alpine
    ports:
      - "5432:5432"
    environment:
      POSTGRES_USER: test
      POSTGRES_PASSWORD: test
      POSTGRES_DB: test

  consul:
    image: hashicorp/consul:1.19
    ports:
      - "8500:8500"
    command: ["agent", "-dev", "-client=0.0.0.0"]
```

**Step 2: Create proto**

```protobuf
syntax = "proto3";
package shared;
option go_package = "github.com/asynkron/protoactor-go/examples/nats-identity-prevention/shared";

message IncrementRequest {
  int32 amount = 1;
}

message GetCountRequest {}

message CountResponse {
  int32 count = 1;
}

service Counter {
  rpc Increment (IncrementRequest) returns (CountResponse) {}
  rpc GetCount (GetCountRequest) returns (CountResponse) {}
}
```

**Step 3: Create postgres-node/main.go**

```go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"nats-identity-prevention/shared"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/consul"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/postgres"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/storage"
	"github.com/asynkron/protoactor-go/remote"
)

type CounterGrain struct {
	count int32
}

func (c *CounterGrain) Init(ctx cluster.GrainContext) {
	ctx.Logger().Info("CounterGrain activated", slog.String("identity", ctx.Identity()))
}
func (c *CounterGrain) Terminate(ctx cluster.GrainContext) {}
func (c *CounterGrain) ReceiveDefault(ctx cluster.GrainContext) {}

func (c *CounterGrain) Increment(req *shared.IncrementRequest, ctx cluster.GrainContext) (*shared.CountResponse, error) {
	c.count += req.Amount
	ctx.Logger().Info("Increment",
		slog.String("identity", ctx.Identity()),
		slog.Int("amount", int(req.Amount)),
		slog.Int("count", int(c.count)))
	return &shared.CountResponse{Count: c.count}, nil
}

func (c *CounterGrain) GetCount(_ *shared.GetCountRequest, ctx cluster.GrainContext) (*shared.CountResponse, error) {
	return &shared.CountResponse{Count: c.count}, nil
}

func main() {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = "postgres://test:test@localhost:5432/test?sslmode=disable"
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to Postgres: %v", err)
	}

	pgStorage := postgres.New("identity-example", db)
	if err := pgStorage.EnsureSchema(context.Background()); err != nil {
		log.Fatalf("Failed to create schema: %v", err)
	}

	identityLookup := storage.New(pgStorage)

	system := actor.NewActorSystem()
	provider, err := consul.New()
	if err != nil {
		log.Fatal(err)
	}
	remoteConfig := remote.Configure("localhost", 0)

	counterKind := shared.NewCounterKind(func() shared.Counter {
		return &CounterGrain{}
	}, 0)

	clusterConfig := cluster.Configure("identity-example", provider, identityLookup, remoteConfig,
		cluster.WithKinds(counterKind))

	c := cluster.NewCluster(system, clusterConfig)
	if err := c.StartMember(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Postgres-backed node started. Press Ctrl+C to stop.")
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	c.Shutdown(true)
}
```

**Step 4: Create nats-node/main.go**

```go
package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"nats-identity-prevention/shared"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/consul"
	natsidentity "github.com/asynkron/protoactor-go/cluster/identitylookup/nats"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/storage"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type CounterGrain struct {
	count int32
}

func (c *CounterGrain) Init(ctx cluster.GrainContext) {
	ctx.Logger().Info("CounterGrain activated", slog.String("identity", ctx.Identity()))
}
func (c *CounterGrain) Terminate(ctx cluster.GrainContext)      {}
func (c *CounterGrain) ReceiveDefault(ctx cluster.GrainContext) {}

func (c *CounterGrain) Increment(req *shared.IncrementRequest, ctx cluster.GrainContext) (*shared.CountResponse, error) {
	c.count += req.Amount
	ctx.Logger().Info("Increment",
		slog.String("identity", ctx.Identity()),
		slog.Int("amount", int(req.Amount)),
		slog.Int("count", int(c.count)))
	return &shared.CountResponse{Count: c.count}, nil
}

func (c *CounterGrain) GetCount(_ *shared.GetCountRequest, ctx cluster.GrainContext) (*shared.CountResponse, error) {
	return &shared.CountResponse{Count: c.count}, nil
}

func main() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		log.Fatalf("Failed to create JetStream: %v", err)
	}

	natsStorage, err := natsidentity.New("identity-example", js)
	if err != nil {
		log.Fatalf("Failed to create NATS identity storage: %v", err)
	}

	identityLookup := storage.New(natsStorage)

	system := actor.NewActorSystem()
	provider, err := consul.New()
	if err != nil {
		log.Fatal(err)
	}
	remoteConfig := remote.Configure("localhost", 0)

	counterKind := shared.NewCounterKind(func() shared.Counter {
		return &CounterGrain{}
	}, 0)

	clusterConfig := cluster.Configure("identity-example", provider, identityLookup, remoteConfig,
		cluster.WithKinds(counterKind))

	c := cluster.NewCluster(system, clusterConfig)
	if err := c.StartMember(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("NATS-backed node started. Press Ctrl+C to stop.")
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	c.Shutdown(true)
}
```

**Step 5: Create client/main.go**

```go
package main

import (
	"flag"
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	"nats-identity-prevention/shared"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/consul"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/remote"
)

var concurrency = flag.Int("concurrency", 10, "Number of concurrent goroutines")

func main() {
	flag.Parse()

	system := actor.NewActorSystem()
	provider, err := consul.New()
	if err != nil {
		log.Fatal(err)
	}
	lookup := disthash.New()
	remoteConfig := remote.Configure("localhost", 0)
	clusterConfig := cluster.Configure("identity-example", provider, lookup, remoteConfig)

	c := cluster.NewCluster(system, clusterConfig)
	if err := c.StartClient(); err != nil {
		log.Fatal(err)
	}

	// Concurrent requests to the SAME counter identity.
	identity := "counter-1"
	var wg sync.WaitGroup
	var successCount atomic.Int32

	fmt.Printf("Sending %d concurrent Increment requests to %q...\n", *concurrency, identity)

	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			client := shared.GetCounterGrainClient(c, identity)
			resp, err := client.Increment(&shared.IncrementRequest{Amount: 1})
			if err != nil {
				fmt.Printf("  goroutine %d: ERROR: %v\n", n, err)
				return
			}
			successCount.Add(1)
			fmt.Printf("  goroutine %d: count=%d\n", n, resp.Count)
		}(i)
	}

	wg.Wait()

	// Final count check.
	client := shared.GetCounterGrainClient(c, identity)
	resp, err := client.GetCount(&shared.GetCountRequest{})
	if err != nil {
		fmt.Printf("GetCount error: %v\n", err)
	} else {
		fmt.Printf("\nFinal count: %d (expected: %d)\n", resp.Count, successCount.Load())
		if resp.Count == successCount.Load() {
			fmt.Println("SUCCESS: All increments were processed by a single actor instance!")
		} else {
			fmt.Println("WARNING: Count mismatch — possible duplicate activation!")
		}
	}

	c.Shutdown(true)
}
```

**Step 6: Build and verify**

Run: `cd examples/nats-identity-prevention && go mod tidy && go build ./...`

**Step 7: Commit**

```
feat(examples): add duplicate activation prevention example

Demonstrates Postgres and NATS JetStream StorageLookup backends
preventing duplicate virtual actor activations under concurrent load.
```

---

### Task 10: Final Verification and Cleanup

**Step 1: Verify all examples compile**

Run:
```bash
for dir in examples/nats-virtual-actor-ingress examples/nats-jetstream-virtual-actor-ingress examples/nats-forwarder examples/nats-stream-locality examples/nats-identity-prevention; do
  echo "Building $dir..."
  (cd "$dir" && go build ./...) || echo "FAIL: $dir"
done
```

Expected: All build successfully.

**Step 2: Verify library packages compile**

Run: `go build ./cluster/identitylookup/postgres/... ./cluster/identitylookup/nats/...`

**Step 3: Run library conformance tests (if infra available)**

Run: `go test -tags integration -v ./cluster/identitylookup/postgres/... ./cluster/identitylookup/nats/...`

**Step 4: Verify go vet passes**

Run: `go vet ./cluster/identitylookup/postgres/... ./cluster/identitylookup/nats/...`

**Step 5: Commit any final fixes**

```
chore: verify all NATS examples and identity lookups compile
```
