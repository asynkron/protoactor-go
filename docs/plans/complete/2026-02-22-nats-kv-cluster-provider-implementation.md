# NATS KV Cluster Provider Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a NATS KV-based cluster provider for protoactor-go that handles member discovery, health checking, leader election, and integrated identity lookup using NATS JetStream KeyValue buckets.

**Architecture:** Members register themselves as keys in a NATS KV bucket with short TTLs, refreshing periodically. A KV watcher detects joins, leaves, and crashes. Leader election uses atomic `Create` on a dedicated key. An integrated identity lookup stores virtual actor activations in a separate KV bucket, implementing `cluster.IdentityLookup` directly.

**Tech Stack:** Go 1.25.3, `github.com/nats-io/nats.go v1.48.0` (JetStream KV), `github.com/nats-io/nats-server/v2` (embedded test server), `github.com/testcontainers/testcontainers-go` (integration tests), `github.com/stretchr/testify` (assertions).

**Design Document:** `docs/plans/2026-02-22-nats-kv-cluster-provider-design.md`

---

## Task 1: Module and Directory Scaffolding

**Files:**
- Create: `cluster/clusterproviders/natskv/go.mod`
- Create: `cluster/clusterproviders/natskv/doc.go`

**Step 1: Create directory and go.mod**

```bash
mkdir -p cluster/clusterproviders/natskv
```

Write `cluster/clusterproviders/natskv/go.mod`:

```go
module github.com/asynkron/protoactor-go/cluster/clusterproviders/natskv

go 1.25.3

require (
	github.com/asynkron/protoactor-go v0.0.0
	github.com/google/uuid v1.6.0
	github.com/nats-io/nats-server/v2 v2.11.4
	github.com/nats-io/nats.go v1.48.0
	github.com/stretchr/testify v1.11.1
	github.com/testcontainers/testcontainers-go v0.40.0
)

replace github.com/asynkron/protoactor-go => ../../../
```

**Step 2: Create doc.go**

Write `cluster/clusterproviders/natskv/doc.go`:

```go
// Package natskv provides a NATS JetStream KV-based cluster provider
// for Proto.Actor. It uses KeyValue buckets for member registration,
// health checking via key TTL, leader election via atomic Create, and
// an integrated identity lookup for virtual actor activation.
package natskv
```

**Step 3: Run go mod tidy**

```bash
cd cluster/clusterproviders/natskv && go mod tidy
```

Expected: Module resolves, `go.sum` generated.

**Step 4: Commit**

```bash
git add cluster/clusterproviders/natskv/
git commit -m "feat(natskv): scaffold module and directory structure"
```

---

## Task 2: Configuration (Options Pattern)

**Files:**
- Create: `cluster/clusterproviders/natskv/config.go`
- Create: `cluster/clusterproviders/natskv/config_test.go`

**Reference:** `cluster/clusterproviders/etcd/config.go`, `cluster/identitylookup/nats/options.go`

**Step 1: Write config_test.go**

```go
package natskv

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	cfg := newDefaultConfig()

	assert.Equal(t, "cluster", cfg.KeyPrefix)
	assert.Equal(t, 5*time.Second, cfg.MemberTTL)
	assert.Equal(t, 2*time.Second, cfg.RefreshInterval)
	assert.Equal(t, 10*time.Second, cfg.LeaderTTL)
	assert.Equal(t, 5*time.Second, cfg.LockTTL)
	assert.Equal(t, 200, cfg.MaxConcurrency)
	assert.Equal(t, 1, cfg.Replicas)
	assert.Equal(t, 1*time.Second, cfg.RetryInterval)
	assert.Empty(t, cfg.BucketName)
	assert.Empty(t, cfg.IdentityBucket)
}

func TestWithBucketName(t *testing.T) {
	cfg := newDefaultConfig()
	WithBucketName("custom_members")(cfg)

	assert.Equal(t, "custom_members", cfg.BucketName)
}

func TestWithIdentityBucket(t *testing.T) {
	cfg := newDefaultConfig()
	WithIdentityBucket("custom_identities")(cfg)

	assert.Equal(t, "custom_identities", cfg.IdentityBucket)
}

func TestWithKeyPrefix(t *testing.T) {
	cfg := newDefaultConfig()
	WithKeyPrefix("myprefix")(cfg)

	assert.Equal(t, "myprefix", cfg.KeyPrefix)
}

func TestWithMemberTTL(t *testing.T) {
	cfg := newDefaultConfig()
	WithMemberTTL(10 * time.Second)(cfg)

	assert.Equal(t, 10*time.Second, cfg.MemberTTL)
}

func TestWithRefreshInterval(t *testing.T) {
	cfg := newDefaultConfig()
	WithRefreshInterval(3 * time.Second)(cfg)

	assert.Equal(t, 3*time.Second, cfg.RefreshInterval)
}

func TestWithLeaderTTL(t *testing.T) {
	cfg := newDefaultConfig()
	WithLeaderTTL(15 * time.Second)(cfg)

	assert.Equal(t, 15*time.Second, cfg.LeaderTTL)
}

func TestWithLockTTL(t *testing.T) {
	cfg := newDefaultConfig()
	WithLockTTL(10 * time.Second)(cfg)

	assert.Equal(t, 10*time.Second, cfg.LockTTL)
}

func TestWithMaxConcurrency(t *testing.T) {
	cfg := newDefaultConfig()
	WithMaxConcurrency(500)(cfg)

	assert.Equal(t, 500, cfg.MaxConcurrency)
}

func TestWithReplicas(t *testing.T) {
	cfg := newDefaultConfig()
	WithReplicas(3)(cfg)

	assert.Equal(t, 3, cfg.Replicas)
}

func TestWithRetryInterval(t *testing.T) {
	cfg := newDefaultConfig()
	WithRetryInterval(5 * time.Second)(cfg)

	assert.Equal(t, 5*time.Second, cfg.RetryInterval)
}

func TestMemberBucketName_Default(t *testing.T) {
	cfg := newDefaultConfig()
	assert.Equal(t, "protoactor_mycluster_members", cfg.memberBucketName("mycluster"))
}

func TestMemberBucketName_Custom(t *testing.T) {
	cfg := newDefaultConfig()
	WithBucketName("custom")(cfg)
	assert.Equal(t, "custom", cfg.memberBucketName("mycluster"))
}

func TestIdentityBucketName_Default(t *testing.T) {
	cfg := newDefaultConfig()
	assert.Equal(t, "protoactor_mycluster_identities", cfg.identityBucketName("mycluster"))
}

func TestIdentityBucketName_Custom(t *testing.T) {
	cfg := newDefaultConfig()
	WithIdentityBucket("custom")(cfg)
	assert.Equal(t, "custom", cfg.identityBucketName("mycluster"))
}
```

**Step 2: Run test to verify it fails**

```bash
cd cluster/clusterproviders/natskv && go test -run TestDefaultConfig -v
```

Expected: FAIL — types/functions not defined.

**Step 3: Write config.go**

```go
package natskv

import "time"

const (
	defaultMemberTTL       = 5 * time.Second
	defaultRefreshInterval = 2 * time.Second
	defaultLeaderTTL       = 10 * time.Second
	defaultLockTTL         = 5 * time.Second
	defaultMaxConcurrency  = 200
	defaultReplicas        = 1
	defaultKeyPrefix       = "cluster"
	defaultRetryInterval   = 1 * time.Second
)

// RoleChangedListener receives notifications when the node's leadership role changes.
type RoleChangedListener interface {
	OnRoleChanged(RoleType)
}

// config holds internal configuration for the NATS KV cluster provider.
type config struct {
	// BucketName overrides the default member KV bucket name.
	// Default: "protoactor_{clusterName}_members".
	BucketName string

	// IdentityBucket overrides the default identity KV bucket name.
	// Default: "protoactor_{clusterName}_identities".
	IdentityBucket string

	// KeyPrefix is the prefix for all keys within the KV bucket.
	// Default: "cluster".
	KeyPrefix string

	// Replicas is the number of KV bucket replicas. Default: 1.
	Replicas int

	// MemberTTL is the TTL applied to member keys. Default: 5s.
	MemberTTL time.Duration

	// RefreshInterval is how often the member key is refreshed. Default: 2s.
	RefreshInterval time.Duration

	// LeaderTTL is the TTL applied to the leader key. Default: 10s.
	LeaderTTL time.Duration

	// LockTTL is the maximum duration a spawn lock is held. Default: 5s.
	LockTTL time.Duration

	// MaxConcurrency limits concurrent identity operations. Default: 200.
	MaxConcurrency int

	// RetryInterval is the backoff between retries. Default: 1s.
	RetryInterval time.Duration

	// RoleChanged is an optional listener notified on leader/follower transitions.
	RoleChanged RoleChangedListener
}

// Option configures the NATS KV cluster provider.
type Option func(*config)

// WithBucketName sets the KV bucket name for member registration.
func WithBucketName(name string) Option {
	return func(c *config) { c.BucketName = name }
}

// WithIdentityBucket sets the KV bucket name for identity activations.
func WithIdentityBucket(name string) Option {
	return func(c *config) { c.IdentityBucket = name }
}

// WithKeyPrefix sets the key prefix used inside the KV bucket.
func WithKeyPrefix(prefix string) Option {
	return func(c *config) { c.KeyPrefix = prefix }
}

// WithReplicas sets the number of replicas for the KV bucket.
func WithReplicas(n int) Option {
	return func(c *config) { c.Replicas = n }
}

// WithMemberTTL sets the TTL for member keys.
func WithMemberTTL(ttl time.Duration) Option {
	return func(c *config) { c.MemberTTL = ttl }
}

// WithRefreshInterval sets the interval for member key refresh.
func WithRefreshInterval(interval time.Duration) Option {
	return func(c *config) { c.RefreshInterval = interval }
}

// WithLeaderTTL sets the TTL for the leader key.
func WithLeaderTTL(ttl time.Duration) Option {
	return func(c *config) { c.LeaderTTL = ttl }
}

// WithLockTTL sets the TTL for spawn locks.
func WithLockTTL(ttl time.Duration) Option {
	return func(c *config) { c.LockTTL = ttl }
}

// WithMaxConcurrency sets the maximum concurrent identity operations.
func WithMaxConcurrency(n int) Option {
	return func(c *config) { c.MaxConcurrency = n }
}

// WithRetryInterval sets the retry backoff interval.
func WithRetryInterval(interval time.Duration) Option {
	return func(c *config) { c.RetryInterval = interval }
}

// WithRoleChangedListener sets a callback for role changes.
func WithRoleChangedListener(l RoleChangedListener) Option {
	return func(c *config) { c.RoleChanged = l }
}

func newDefaultConfig() *config {
	return &config{
		KeyPrefix:       defaultKeyPrefix,
		Replicas:        defaultReplicas,
		MemberTTL:       defaultMemberTTL,
		RefreshInterval: defaultRefreshInterval,
		LeaderTTL:       defaultLeaderTTL,
		LockTTL:         defaultLockTTL,
		MaxConcurrency:  defaultMaxConcurrency,
		RetryInterval:   defaultRetryInterval,
	}
}

// memberBucketName returns the KV bucket name for member registration.
func (c *config) memberBucketName(clusterName string) string {
	if c.BucketName != "" {
		return c.BucketName
	}
	return "protoactor_" + clusterName + "_members"
}

// identityBucketName returns the KV bucket name for identity activations.
func (c *config) identityBucketName(clusterName string) string {
	if c.IdentityBucket != "" {
		return c.IdentityBucket
	}
	return "protoactor_" + clusterName + "_identities"
}
```

**Step 4: Run tests to verify they pass**

```bash
cd cluster/clusterproviders/natskv && go test -run TestDefault -v && go test -run TestWith -v && go test -run TestMember -v && go test -run TestIdentityBucket -v
```

Expected: All PASS.

**Step 5: Commit**

```bash
git add cluster/clusterproviders/natskv/config.go cluster/clusterproviders/natskv/config_test.go
git commit -m "feat(natskv): add configuration with options pattern"
```

---

## Task 3: Node Struct

**Files:**
- Create: `cluster/clusterproviders/natskv/node.go`
- Create: `cluster/clusterproviders/natskv/node_test.go`

**Reference:** `cluster/clusterproviders/etcd/node.go`

**Step 1: Write node_test.go**

```go
package natskv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNode(t *testing.T) {
	n := NewNode("node1", "localhost", 8080, []string{"kind1", "kind2"})

	assert.Equal(t, "node1", n.ID)
	assert.Equal(t, "node1", n.Name)
	assert.Equal(t, "localhost", n.Host)
	assert.Equal(t, 8080, n.Port)
	assert.Equal(t, []string{"kind1", "kind2"}, n.Kinds)
	assert.True(t, n.Alive)
}

func TestNode_Serialize_Deserialize(t *testing.T) {
	n := NewNode("node1", "192.168.1.1", 9000, []string{"grainA"})

	data, err := n.Serialize()
	require.NoError(t, err)

	n2, err := NewNodeFromBytes(data)
	require.NoError(t, err)

	assert.Equal(t, n.ID, n2.ID)
	assert.Equal(t, n.Host, n2.Host)
	assert.Equal(t, n.Port, n2.Port)
	assert.Equal(t, n.Kinds, n2.Kinds)
	assert.Equal(t, n.Alive, n2.Alive)
}

func TestNewNodeFromBytes_Invalid(t *testing.T) {
	_, err := NewNodeFromBytes([]byte("not json"))
	assert.Error(t, err)
}

func TestNode_MemberStatus(t *testing.T) {
	n := NewNode("node1", "10.0.0.1", 5000, []string{"kind1", "kind2"})
	m := n.MemberStatus()

	assert.Equal(t, "node1", m.Id)
	assert.Equal(t, "10.0.0.1", m.Host)
	assert.Equal(t, int32(5000), m.Port)
	assert.Equal(t, []string{"kind1", "kind2"}, m.Kinds)
}

func TestNode_MemberStatus_NilKinds(t *testing.T) {
	n := NewNode("node1", "10.0.0.1", 5000, nil)
	m := n.MemberStatus()

	assert.NotNil(t, m.Kinds)
	assert.Empty(t, m.Kinds)
}

func TestNode_Equal(t *testing.T) {
	n1 := NewNode("node1", "localhost", 8080, nil)
	n2 := NewNode("node1", "localhost", 9090, nil)
	n3 := NewNode("node2", "localhost", 8080, nil)

	assert.True(t, n1.Equal(n2))
	assert.False(t, n1.Equal(n3))
	assert.False(t, n1.Equal(nil))
	assert.True(t, n1.Equal(n1))
}

func TestNode_AliveFlag(t *testing.T) {
	n := NewNode("node1", "localhost", 8080, nil)
	assert.True(t, n.IsAlive())

	n.SetAlive(false)
	assert.False(t, n.IsAlive())
}
```

**Step 2: Run test to verify it fails**

```bash
cd cluster/clusterproviders/natskv && go test -run TestNewNode -v
```

Expected: FAIL — `NewNode` not defined.

**Step 3: Write node.go**

```go
package natskv

import (
	"encoding/json"

	"github.com/asynkron/protoactor-go/cluster"
)

// Node represents a cluster member stored in the NATS KV bucket.
type Node struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Host  string   `json:"host"`
	Port  int      `json:"port"`
	Kinds []string `json:"kinds"`
	Alive bool     `json:"alive"`
}

// NewNode constructs a new Node instance.
func NewNode(name, host string, port int, kinds []string) *Node {
	return &Node{
		ID:    name,
		Name:  name,
		Host:  host,
		Port:  port,
		Kinds: kinds,
		Alive: true,
	}
}

// NewNodeFromBytes decodes a Node from its JSON representation.
func NewNodeFromBytes(data []byte) (*Node, error) {
	n := &Node{}
	if err := json.Unmarshal(data, n); err != nil {
		return nil, err
	}
	return n, nil
}

// Equal compares two nodes by ID.
func (n *Node) Equal(other *Node) bool {
	if n == nil || other == nil {
		return false
	}
	if n == other {
		return true
	}
	return n.ID == other.ID
}

// MemberStatus converts the node into a cluster.Member description.
func (n *Node) MemberStatus() *cluster.Member {
	kinds := n.Kinds
	if kinds == nil {
		kinds = []string{}
	}
	return &cluster.Member{
		Id:    n.ID,
		Host:  n.Host,
		Port:  int32(n.Port),
		Kinds: kinds,
	}
}

// IsAlive reports whether the node is considered alive.
func (n *Node) IsAlive() bool {
	return n.Alive
}

// SetAlive updates the alive flag for the node.
func (n *Node) SetAlive(alive bool) {
	n.Alive = alive
}

// Serialize encodes the node to JSON.
func (n *Node) Serialize() ([]byte, error) {
	return json.Marshal(n)
}

// Deserialize populates the node from JSON data.
func (n *Node) Deserialize(data []byte) error {
	return json.Unmarshal(data, n)
}
```

**Step 4: Run tests to verify they pass**

```bash
cd cluster/clusterproviders/natskv && go test -run TestNode -v && go test -run TestNew -v
```

Expected: All PASS.

**Step 5: Commit**

```bash
git add cluster/clusterproviders/natskv/node.go cluster/clusterproviders/natskv/node_test.go
git commit -m "feat(natskv): add Node struct with serialization and member conversion"
```

---

## Task 4: Singleton Scheduler and Role Types

**Files:**
- Create: `cluster/clusterproviders/natskv/singleton.go`
- Create: `cluster/clusterproviders/natskv/singleton_test.go`

**Reference:** `cluster/clusterproviders/etcd/singleton.go` — copy this pattern exactly.

**Step 1: Write singleton_test.go**

```go
package natskv

import (
	"sync/atomic"
	"testing"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

func TestRoleType_String(t *testing.T) {
	assert.Equal(t, "Leader", Leader.String())
	assert.Equal(t, "Follower", Follower.String())
}

func TestSingletonScheduler_FromFunc(t *testing.T) {
	system := actor.NewActorSystem()
	s := NewSingletonScheduler(system.Root)

	s.FromFunc(func(ctx actor.Context) {})

	assert.Len(t, s.props, 1)
}

func TestSingletonScheduler_FromProducer(t *testing.T) {
	system := actor.NewActorSystem()
	s := NewSingletonScheduler(system.Root)

	s.FromProducer(func() actor.Actor {
		return &testActor{}
	})

	assert.Len(t, s.props, 1)
}

func TestSingletonScheduler_OnRoleChanged_Leader(t *testing.T) {
	system := actor.NewActorSystem()
	s := NewSingletonScheduler(system.Root)

	var spawned atomic.Int32
	s.FromFunc(func(ctx actor.Context) {
		if _, ok := ctx.Message().(*actor.Started); ok {
			spawned.Add(1)
		}
	})

	s.OnRoleChanged(Leader)

	assert.Len(t, s.pids, 1)
	assert.NotNil(t, s.pids[0])
}

func TestSingletonScheduler_OnRoleChanged_Follower(t *testing.T) {
	system := actor.NewActorSystem()
	s := NewSingletonScheduler(system.Root)

	s.FromFunc(func(ctx actor.Context) {})

	s.OnRoleChanged(Leader)
	assert.Len(t, s.pids, 1)

	s.OnRoleChanged(Follower)
	assert.Nil(t, s.pids)
}

type testActor struct{}

func (a *testActor) Receive(ctx actor.Context) {}
```

**Step 2: Run test to verify it fails**

```bash
cd cluster/clusterproviders/natskv && go test -run TestRoleType -v
```

Expected: FAIL — types not defined.

**Step 3: Write singleton.go**

```go
package natskv

import (
	"sync"

	"github.com/asynkron/protoactor-go/actor"
)

// RoleType represents the node's leadership role in the cluster.
type RoleType int

const (
	// Follower indicates the node is not the leader.
	Follower RoleType = iota
	// Leader indicates the node currently holds leadership.
	Leader
)

// String returns a human-readable representation of the role.
func (r RoleType) String() string {
	switch r {
	case Leader:
		return "Leader"
	default:
		return "Follower"
	}
}

// SingletonScheduler manages actors that should only run on the leader node.
type SingletonScheduler struct {
	sync.Mutex
	root  *actor.RootContext
	props []*actor.Props
	pids  []*actor.PID
}

// NewSingletonScheduler creates a new scheduler bound to the given root context.
func NewSingletonScheduler(rc *actor.RootContext) *SingletonScheduler {
	return &SingletonScheduler{root: rc}
}

// FromFunc registers an actor function to run when the node becomes leader.
func (s *SingletonScheduler) FromFunc(f actor.ReceiveFunc) *SingletonScheduler {
	s.Lock()
	defer s.Unlock()
	s.props = append(s.props, actor.PropsFromFunc(f))
	return s
}

// FromProducer registers an actor producer to run when the node becomes leader.
func (s *SingletonScheduler) FromProducer(f actor.Producer) *SingletonScheduler {
	s.Lock()
	defer s.Unlock()
	s.props = append(s.props, actor.PropsFromProducer(f))
	return s
}

// OnRoleChanged reacts to leadership changes and spawns or poisons actors accordingly.
func (s *SingletonScheduler) OnRoleChanged(rt RoleType) {
	s.Lock()
	defer s.Unlock()
	switch rt {
	case Follower:
		if len(s.pids) > 0 {
			s.root.Logger().Info("I am follower, poison singleton actors")
			for _, pid := range s.pids {
				s.root.Poison(pid)
			}
			s.pids = nil
		}
	case Leader:
		if len(s.props) > 0 {
			s.root.Logger().Info("I am leader now, start singleton actors")
			s.pids = make([]*actor.PID, len(s.props))
			for i, p := range s.props {
				s.pids[i] = s.root.Spawn(p)
			}
		}
	}
}
```

**Step 4: Run tests to verify they pass**

```bash
cd cluster/clusterproviders/natskv && go test -run TestRoleType -v && go test -run TestSingleton -v
```

Expected: All PASS.

**Step 5: Commit**

```bash
git add cluster/clusterproviders/natskv/singleton.go cluster/clusterproviders/natskv/singleton_test.go
git commit -m "feat(natskv): add SingletonScheduler and RoleType (etcd pattern)"
```

---

## Task 5: Test Helpers (Embedded NATS Server)

**Files:**
- Create: `cluster/clusterproviders/natskv/testhelpers_test.go`

**Step 1: Write testhelpers_test.go**

This file creates helper functions for spinning up embedded NATS servers in tests.

```go
package natskv

import (
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
)

// startEmbeddedNATS starts an embedded NATS server with JetStream enabled.
// The server is automatically shut down when the test completes.
func startEmbeddedNATS(t *testing.T) *server.Server {
	t.Helper()

	opts := &server.Options{
		JetStream: true,
		Port:      -1, // random port
		StoreDir:  t.TempDir(),
	}

	srv, err := server.NewServer(opts)
	require.NoError(t, err, "failed to create NATS server")

	srv.Start()
	t.Cleanup(func() {
		srv.Shutdown()
		srv.WaitForShutdown()
	})

	if !srv.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server not ready")
	}

	return srv
}

// connectNATS connects to the given embedded NATS server and returns a
// JetStream handle. The connection is closed when the test completes.
func connectNATS(t *testing.T, srv *server.Server) (*nats.Conn, jetstream.JetStream) {
	t.Helper()

	nc, err := nats.Connect(srv.ClientURL())
	require.NoError(t, err, "failed to connect to NATS")
	t.Cleanup(func() { nc.Close() })

	js, err := jetstream.New(nc)
	require.NoError(t, err, "failed to create JetStream context")

	return nc, js
}
```

**Step 2: Verify it compiles**

```bash
cd cluster/clusterproviders/natskv && go test -run NONE -v
```

Expected: compiles successfully (no tests match NONE, but compilation succeeds).

**Step 3: Commit**

```bash
git add cluster/clusterproviders/natskv/testhelpers_test.go
git commit -m "test(natskv): add embedded NATS server test helpers"
```

---

## Task 6: Provider Struct and Constructors

**Files:**
- Create: `cluster/clusterproviders/natskv/natskv_provider.go`
- Modify: `cluster/clusterproviders/natskv/natskv_provider_test.go` (create)

**Reference:** `cluster/clusterproviders/etcd/etcd_provider.go:46-85` (constructors), `cluster/cluster_provider.go` (interface)

**Step 1: Write the failing tests for constructors**

Create `cluster/clusterproviders/natskv/natskv_provider_test.go`:

```go
package natskv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_ReturnsProvider(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc, _ := connectNATS(t, srv)

	p, err := New(nc)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestNewFromJetStream_ReturnsProvider(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	p, err := NewFromJetStream(js)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestNew_WithOptions(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc, _ := connectNATS(t, srv)

	p, err := New(nc,
		WithBucketName("custom_bucket"),
		WithKeyPrefix("myprefix"),
	)
	require.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, "custom_bucket", p.config.BucketName)
	assert.Equal(t, "myprefix", p.config.KeyPrefix)
}

func TestProvider_IdentityLookup(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc, _ := connectNATS(t, srv)

	p, err := New(nc)
	require.NoError(t, err)

	lookup := p.IdentityLookup()
	assert.NotNil(t, lookup)
}

func TestProvider_GetHealthStatus_NilByDefault(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc, _ := connectNATS(t, srv)

	p, err := New(nc)
	require.NoError(t, err)

	assert.NoError(t, p.GetHealthStatus())
}
```

**Step 2: Run test to verify it fails**

```bash
cd cluster/clusterproviders/natskv && go test -run TestNew_ReturnsProvider -v
```

Expected: FAIL — `New` not defined.

**Step 3: Write the provider struct and constructors**

Write `cluster/clusterproviders/natskv/natskv_provider.go`:

```go
package natskv

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/asynkron/protoactor-go/cluster"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Compile-time interface check.
var _ cluster.ClusterProvider = (*Provider)(nil)

// Provider uses NATS JetStream KV for cluster membership discovery,
// health checking, and leader election.
type Provider struct {
	cluster      *cluster.Cluster
	clusterName  string
	config       *config
	js           jetstream.JetStream
	memberBucket jetstream.KeyValue
	self         *Node
	members      map[string]*Node
	membersMu    sync.RWMutex
	shutdown     atomic.Bool
	clusterError error
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc

	// Leader election
	role                RoleType
	roleChangedChan     chan RoleType
	roleChangedListener RoleChangedListener
	schedulers          []*SingletonScheduler
	isLeader            atomic.Bool

	// Integrated identity lookup
	identity *IdentityLookup
}

// New creates a Provider using a NATS connection.
func New(conn *nats.Conn, opts ...Option) (*Provider, error) {
	js, err := jetstream.New(conn)
	if err != nil {
		return nil, fmt.Errorf("natskv: create JetStream context: %w", err)
	}
	return NewFromJetStream(js, opts...)
}

// NewFromJetStream creates a Provider using an existing JetStream handle.
func NewFromJetStream(js jetstream.JetStream, opts ...Option) (*Provider, error) {
	cfg := newDefaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	p := &Provider{
		config:          cfg,
		js:              js,
		members:         make(map[string]*Node),
		role:            Follower,
		roleChangedChan: make(chan RoleType, 1),
		roleChangedListener: cfg.RoleChanged,
	}

	p.identity = newIdentityLookup(p)

	return p, nil
}

// IdentityLookup returns the integrated identity lookup.
func (p *Provider) IdentityLookup() *IdentityLookup {
	return p.identity
}

// GetHealthStatus returns an error if the cluster health status has problems.
func (p *Provider) GetHealthStatus() error {
	return p.clusterError
}

// RegisterSingletonScheduler adds a singleton scheduler to be notified on role changes.
func (p *Provider) RegisterSingletonScheduler(scheduler *SingletonScheduler) {
	p.schedulers = append(p.schedulers, scheduler)
}

// init extracts host, port, memberID, and kinds from the cluster and builds the self node.
func (p *Provider) init(c *cluster.Cluster) error {
	p.cluster = c
	p.clusterName = c.Config.Name
	addr := c.ActorSystem.Address()
	host, port, err := splitHostPort(addr)
	if err != nil {
		return err
	}

	memberID := c.ActorSystem.ID
	knownKinds := c.GetClusterKinds()
	nodeName := fmt.Sprintf("%v@%v", p.clusterName, memberID)
	p.self = NewNode(nodeName, host, port, knownKinds)
	return nil
}

// StartMember registers the node in NATS KV and starts watching for updates.
func (p *Provider) StartMember(c *cluster.Cluster) error {
	p.ctx, p.cancel = context.WithCancel(context.Background())

	if err := p.init(c); err != nil {
		return err
	}

	if err := p.createMemberBucket(); err != nil {
		return err
	}

	if err := p.registerSelf(); err != nil {
		return err
	}

	p.startRoleChangedNotifyLoop()

	// Load existing members
	if err := p.loadInitialMembers(); err != nil {
		return err
	}

	p.publishClusterTopologyEvent()
	p.startWatching()
	p.startRefresh()
	p.attemptLeaderElection()

	return nil
}

// StartClient initializes the provider without registering the node.
func (p *Provider) StartClient(c *cluster.Cluster) error {
	p.ctx, p.cancel = context.WithCancel(context.Background())

	if err := p.init(c); err != nil {
		return err
	}

	if err := p.createMemberBucket(); err != nil {
		return err
	}

	if err := p.loadInitialMembers(); err != nil {
		return err
	}

	p.publishClusterTopologyEvent()
	p.startWatching()

	return nil
}

// Shutdown deregisters the node and stops background tasks.
func (p *Provider) Shutdown(_ bool) error {
	if !p.shutdown.CompareAndSwap(false, true) {
		return nil
	}

	if p.cancel != nil {
		p.cancel()
	}

	// Delete own member key
	if p.self != nil && p.memberBucket != nil {
		key := p.memberKey(p.self.ID)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = p.memberBucket.Delete(ctx, key)

		// If leader, delete leader key
		if p.isLeader.Load() {
			_ = p.memberBucket.Delete(ctx, p.leaderKey())
		}
	}

	// Wait for all goroutines to finish
	p.wg.Wait()

	return nil
}

// createMemberBucket creates or binds the member KV bucket.
func (p *Provider) createMemberBucket() error {
	bucketName := p.config.memberBucketName(p.clusterName)

	kv, err := p.js.CreateOrUpdateKeyValue(p.ctx, jetstream.KeyValueConfig{
		Bucket:   bucketName,
		Replicas: p.config.Replicas,
		TTL:      p.config.MemberTTL * 3, // safety net, per-key TTL is the primary mechanism
	})
	if err != nil {
		return fmt.Errorf("natskv: create member bucket %q: %w", bucketName, err)
	}

	p.memberBucket = kv
	return nil
}

// memberKey returns the full KV key for a member.
func (p *Provider) memberKey(memberID string) string {
	return p.config.KeyPrefix + ".members." + memberID
}

// leaderKey returns the full KV key for the leader.
func (p *Provider) leaderKey() string {
	return p.config.KeyPrefix + ".leader"
}

// registerSelf writes the member key with TTL.
func (p *Provider) registerSelf() error {
	data, err := p.self.Serialize()
	if err != nil {
		return fmt.Errorf("natskv: serialize self: %w", err)
	}

	key := p.memberKey(p.self.ID)
	_, err = p.memberBucket.Put(p.ctx, key, data, jetstream.KeyTTL(p.config.MemberTTL))
	if err != nil {
		return fmt.Errorf("natskv: register self: %w", err)
	}

	return nil
}

// loadInitialMembers reads all existing member keys from the KV bucket.
func (p *Provider) loadInitialMembers() error {
	prefix := p.config.KeyPrefix + ".members."
	ctx := p.ctx

	watcher, err := p.memberBucket.Watch(ctx, prefix+">", jetstream.IncludeHistory())
	if err != nil {
		return fmt.Errorf("natskv: watch initial members: %w", err)
	}
	defer watcher.Stop()

	for entry := range watcher.Updates() {
		if entry == nil {
			// nil signals end of initial values
			break
		}
		if entry.Operation() == jetstream.KeyValueDelete || entry.Operation() == jetstream.KeyValuePurge {
			continue
		}
		p.handleMemberPut(entry)
	}

	return nil
}

// handleMemberPut processes a Put event for a member key.
func (p *Provider) handleMemberPut(entry jetstream.KeyValueEntry) {
	node, err := NewNodeFromBytes(entry.Value())
	if err != nil {
		if p.cluster != nil {
			p.cluster.Logger().Error("Invalid member data",
				slog.String("key", entry.Key()), slog.Any("error", err))
		}
		return
	}

	// Don't track self via the watcher
	if p.self != nil && node.Equal(p.self) {
		return
	}

	p.membersMu.Lock()
	p.members[node.ID] = node
	p.membersMu.Unlock()
}

// handleMemberDelete processes a Delete event for a member key.
func (p *Provider) handleMemberDelete(entry jetstream.KeyValueEntry) {
	memberID := extractMemberID(entry.Key(), p.config.KeyPrefix+".members.")
	if memberID == "" {
		return
	}

	p.membersMu.Lock()
	delete(p.members, memberID)
	p.membersMu.Unlock()
}

// publishClusterTopologyEvent converts internal state to cluster.Member list
// and notifies the cluster's MemberList.
func (p *Provider) publishClusterTopologyEvent() {
	p.membersMu.RLock()
	members := make([]*cluster.Member, 0, len(p.members)+1)
	for _, m := range p.members {
		if m.IsAlive() {
			members = append(members, m.MemberStatus())
		}
	}
	p.membersMu.RUnlock()

	// Include self if registered
	if p.self != nil {
		members = append(members, p.self.MemberStatus())
	}

	if p.cluster != nil {
		p.cluster.Logger().Info("Update cluster topology",
			slog.String("provider", "natskv"),
			slog.Int("members", len(members)))
		p.cluster.MemberList.UpdateClusterTopology(members)
	}
}

// startWatching starts the KV watcher goroutine.
func (p *Provider) startWatching() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				if p.cluster != nil {
					p.cluster.Logger().Error("Recovered from panic in watcher",
						slog.String("provider", "natskv"),
						slog.Any("error", r))
				}
				p.clusterError = fmt.Errorf("watcher panic: %v", r)
			}
		}()

		for !p.shutdown.Load() {
			if err := p.keepWatching(); err != nil {
				if p.shutdown.Load() {
					return
				}
				if p.cluster != nil {
					p.cluster.Logger().Error("Watcher failed, retrying",
						slog.String("provider", "natskv"),
						slog.Any("error", err))
				}
				p.clusterError = err

				select {
				case <-time.After(p.config.RetryInterval):
				case <-p.ctx.Done():
					return
				}
			}
		}
	}()
}

// keepWatching runs the KV watcher until an error or shutdown.
func (p *Provider) keepWatching() error {
	prefix := p.config.KeyPrefix + ".members."
	watcher, err := p.memberBucket.Watch(p.ctx, prefix+">", jetstream.UpdatesOnly())
	if err != nil {
		return fmt.Errorf("natskv: start watcher: %w", err)
	}
	defer watcher.Stop()

	for entry := range watcher.Updates() {
		if entry == nil {
			continue
		}

		key := entry.Key()

		switch entry.Operation() {
		case jetstream.KeyValuePut:
			p.handleMemberPut(entry)
		case jetstream.KeyValueDelete, jetstream.KeyValuePurge:
			p.handleMemberDelete(entry)
		default:
			if p.cluster != nil {
				p.cluster.Logger().Warn("Unknown KV operation",
					slog.String("key", key),
					slog.String("operation", entry.Operation().String()))
			}
		}

		p.publishClusterTopologyEvent()
	}

	return nil
}

// startRefresh starts the goroutine that periodically re-puts the member key.
func (p *Provider) startRefresh() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				if p.cluster != nil {
					p.cluster.Logger().Error("Recovered from panic in refresh",
						slog.String("provider", "natskv"),
						slog.Any("error", r))
				}
			}
		}()

		ticker := time.NewTicker(p.config.RefreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if p.shutdown.Load() {
					return
				}
				if err := p.refreshMemberKey(); err != nil {
					if p.cluster != nil {
						p.cluster.Logger().Warn("Failed to refresh member key",
							slog.String("provider", "natskv"),
							slog.Any("error", err))
					}
				}
				if err := p.refreshLeaderKey(); err != nil {
					if p.cluster != nil {
						p.cluster.Logger().Warn("Failed to refresh leader key",
							slog.String("provider", "natskv"),
							slog.Any("error", err))
					}
				}
			case <-p.ctx.Done():
				return
			}
		}
	}()
}

// refreshMemberKey re-puts the member key with TTL to keep it alive.
func (p *Provider) refreshMemberKey() error {
	data, err := p.self.Serialize()
	if err != nil {
		return err
	}
	key := p.memberKey(p.self.ID)
	_, err = p.memberBucket.Put(p.ctx, key, data, jetstream.KeyTTL(p.config.MemberTTL))
	return err
}

// refreshLeaderKey re-puts the leader key if this node is the leader.
func (p *Provider) refreshLeaderKey() error {
	if !p.isLeader.Load() {
		return nil
	}

	data := []byte(fmt.Sprintf(`{"memberID":"%s","electedAt":"%s"}`, p.self.ID, time.Now().UTC().Format(time.RFC3339)))
	key := p.leaderKey()
	_, err := p.memberBucket.Put(p.ctx, key, data, jetstream.KeyTTL(p.config.LeaderTTL))
	if err != nil {
		// Lost leader key — re-attempt election
		p.isLeader.Store(false)
		p.setRole(Follower)
		p.attemptLeaderElection()
	}
	return err
}

// attemptLeaderElection tries to become leader using atomic Create.
func (p *Provider) attemptLeaderElection() {
	key := p.leaderKey()
	data := []byte(fmt.Sprintf(`{"memberID":"%s","electedAt":"%s"}`, p.self.ID, time.Now().UTC().Format(time.RFC3339)))

	_, err := p.memberBucket.Create(p.ctx, key, data)
	if err != nil {
		// Another member is already leader or NATS error
		return
	}

	// We are leader — set the TTL on the key
	_, _ = p.memberBucket.Put(p.ctx, key, data, jetstream.KeyTTL(p.config.LeaderTTL))
	p.isLeader.Store(true)
	p.setRole(Leader)
}

// setRole updates the role and notifies listeners.
func (p *Provider) setRole(role RoleType) {
	if role == p.role {
		return
	}

	if p.cluster != nil {
		p.cluster.Logger().Info("Role changed",
			slog.String("provider", "natskv"),
			slog.String("from", p.role.String()),
			slog.String("to", role.String()))
	}

	p.role = role

	// Non-blocking send to role changed channel
	select {
	case p.roleChangedChan <- role:
	default:
	}

	// Notify all registered singleton schedulers
	for _, scheduler := range p.schedulers {
		safeRun(p.logger(), func() {
			scheduler.OnRoleChanged(role)
		})
	}
}

// startRoleChangedNotifyLoop starts the goroutine that notifies the role changed listener.
func (p *Provider) startRoleChangedNotifyLoop() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for {
			select {
			case role := <-p.roleChangedChan:
				if lis := p.roleChangedListener; lis != nil {
					safeRun(p.logger(), func() { lis.OnRoleChanged(role) })
				}
			case <-p.ctx.Done():
				return
			}
		}
	}()
}

func (p *Provider) logger() *slog.Logger {
	if p.cluster != nil {
		return p.cluster.Logger()
	}
	return slog.Default()
}

// safeRun executes fn and recovers from panics.
func safeRun(logger *slog.Logger, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 64<<10)
			buf = buf[:runtime.Stack(buf, false)]
			logger.Warn("OnRoleChanged panic recovered",
				slog.Any("error", fmt.Errorf("%v\n%s", r, buf)))
		}
	}()
	fn()
}

// extractMemberID extracts the member ID from a KV key by stripping the prefix.
func extractMemberID(key, prefix string) string {
	if len(key) <= len(prefix) {
		return ""
	}
	return key[len(prefix):]
}

// splitHostPort parses an address string into host and port components.
func splitHostPort(addr string) (host string, port int, err error) {
	if h, p, e := net.SplitHostPort(addr); e != nil {
		if addr != "nonhost" {
			err = e
		}
		host = "nonhost"
		port = -1
	} else {
		host = h
		port, err = strconv.Atoi(p)
	}
	return
}
```

**Step 4: Run tests to verify they pass**

```bash
cd cluster/clusterproviders/natskv && go test -run TestNew -v && go test -run TestProvider -v
```

Expected: All PASS.

**Step 5: Commit**

```bash
git add cluster/clusterproviders/natskv/natskv_provider.go cluster/clusterproviders/natskv/natskv_provider_test.go
git commit -m "feat(natskv): add Provider struct with constructors and ClusterProvider implementation"
```

---

## Task 7: Identity Lookup

**Files:**
- Create: `cluster/clusterproviders/natskv/natskv_identity.go`
- Create: `cluster/clusterproviders/natskv/natskv_identity_test.go`

**Reference:** `cluster/identitylookup/nats/nats_identity.go`, `cluster/identitylookup/storage/identity_storage_lookup.go`

**Step 1: Write identity tests**

Create `cluster/clusterproviders/natskv/natskv_identity_test.go`:

```go
package natskv

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asynkron/protoactor-go/cluster"
)

func TestIdentityLookup_InterfaceCompliance(t *testing.T) {
	// Compile-time check (also verified by var _ declaration)
	var _ cluster.IdentityLookup = (*IdentityLookup)(nil)
}

func TestIdentityLookup_kvKey(t *testing.T) {
	ci := &cluster.ClusterIdentity{Kind: "MyGrain", Identity: "abc-123"}
	key := kvKey(ci)

	// Should replace / with .
	assert.NotContains(t, key, "/")
	assert.Contains(t, key, ".")
}

func TestIdentityLookup_AcquireLock(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	ctx := context.Background()
	bucket, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_identities",
	})
	require.NoError(t, err)

	il := &IdentityLookup{
		identities: bucket,
		config:     newDefaultConfig(),
		semaphore:  make(chan struct{}, 200),
	}

	ci := &cluster.ClusterIdentity{Kind: "TestGrain", Identity: "id1"}

	// First acquire should succeed
	lockID, rev, ok := il.tryAcquireLock(ctx, ci)
	assert.True(t, ok)
	assert.NotEmpty(t, lockID)
	assert.Greater(t, rev, uint64(0))

	// Second acquire should fail (key exists)
	_, _, ok2 := il.tryAcquireLock(ctx, ci)
	assert.False(t, ok2)
}

func TestIdentityLookup_StoreActivation(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	ctx := context.Background()
	idBucket, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_store_identities",
	})
	require.NoError(t, err)

	memBucket, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_store_members",
	})
	require.NoError(t, err)

	il := &IdentityLookup{
		identities:    idBucket,
		memberTracker: memBucket,
		config:        newDefaultConfig(),
		semaphore:     make(chan struct{}, 200),
	}

	ci := &cluster.ClusterIdentity{Kind: "TestGrain", Identity: "id1"}

	// Acquire the lock first
	lockID, rev, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok)

	// Store activation
	err = il.storeActivation(ctx, ci, lockID, rev, "member1", "127.0.0.1:8080", "TestGrain/id1")
	assert.NoError(t, err)

	// Verify the stored record
	key := kvKey(ci)
	entry, err := idBucket.Get(ctx, key)
	require.NoError(t, err)

	var rec activationRecord
	require.NoError(t, json.Unmarshal(entry.Value(), &rec))
	assert.Empty(t, rec.LockID)
	assert.Equal(t, "TestGrain/id1", rec.PidID)
	assert.Equal(t, "127.0.0.1:8080", rec.PidAddress)
	assert.Equal(t, "member1", rec.MemberID)
}

func TestIdentityLookup_GetExistingActivation(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	ctx := context.Background()
	idBucket, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_get_identities",
	})
	require.NoError(t, err)

	memBucket, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_get_members",
	})
	require.NoError(t, err)

	il := &IdentityLookup{
		identities:    idBucket,
		memberTracker: memBucket,
		config:        newDefaultConfig(),
		semaphore:     make(chan struct{}, 200),
	}

	ci := &cluster.ClusterIdentity{Kind: "TestGrain", Identity: "id1"}

	// No activation yet
	rec := il.getExistingActivation(ctx, ci)
	assert.Nil(t, rec)

	// Store an activation
	lockID, rev, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok)
	require.NoError(t, il.storeActivation(ctx, ci, lockID, rev, "member1", "127.0.0.1:8080", "TestGrain/id1"))

	// Now it should exist
	rec = il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec)
	assert.Equal(t, "TestGrain/id1", rec.PidID)
	assert.Equal(t, "127.0.0.1:8080", rec.PidAddress)
	assert.Equal(t, "member1", rec.MemberID)
}

func TestIdentityLookup_RemoveMember(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	ctx := context.Background()
	idBucket, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_rm_identities",
	})
	require.NoError(t, err)

	memBucket, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_rm_members",
	})
	require.NoError(t, err)

	il := &IdentityLookup{
		identities:    idBucket,
		memberTracker: memBucket,
		config:        newDefaultConfig(),
		semaphore:     make(chan struct{}, 200),
	}

	ci1 := &cluster.ClusterIdentity{Kind: "TestGrain", Identity: "id1"}
	ci2 := &cluster.ClusterIdentity{Kind: "TestGrain", Identity: "id2"}

	// Create two activations for the same member
	lockID1, rev1, ok := il.tryAcquireLock(ctx, ci1)
	require.True(t, ok)
	require.NoError(t, il.storeActivation(ctx, ci1, lockID1, rev1, "member1", "127.0.0.1:8080", "TestGrain/id1"))

	lockID2, rev2, ok := il.tryAcquireLock(ctx, ci2)
	require.True(t, ok)
	require.NoError(t, il.storeActivation(ctx, ci2, lockID2, rev2, "member1", "127.0.0.1:8080", "TestGrain/id2"))

	// Remove member
	il.removeMemberID(ctx, "member1")

	// Both activations should be gone
	assert.Nil(t, il.getExistingActivation(ctx, ci1))
	assert.Nil(t, il.getExistingActivation(ctx, ci2))
}

func TestIdentityLookup_WaitForActivation(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	ctx := context.Background()
	idBucket, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_wait_identities",
	})
	require.NoError(t, err)

	memBucket, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_wait_members",
	})
	require.NoError(t, err)

	cfg := newDefaultConfig()
	cfg.LockTTL = 3 * time.Second

	il := &IdentityLookup{
		identities:    idBucket,
		memberTracker: memBucket,
		config:        cfg,
		semaphore:     make(chan struct{}, 200),
	}

	ci := &cluster.ClusterIdentity{Kind: "TestGrain", Identity: "wait1"}

	// Acquire lock first (simulates another node starting to spawn)
	lockID, rev, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok)

	// Start waiting in a goroutine
	done := make(chan *activationRecord, 1)
	go func() {
		rec := il.waitForActivation(ctx, ci)
		done <- rec
	}()

	// Simulate storing activation after a short delay
	time.Sleep(200 * time.Millisecond)
	require.NoError(t, il.storeActivation(ctx, ci, lockID, rev, "member1", "127.0.0.1:8080", "TestGrain/wait1"))

	// Should receive the activation
	select {
	case rec := <-done:
		require.NotNil(t, rec)
		assert.Equal(t, "TestGrain/wait1", rec.PidID)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for activation")
	}
}

func TestIdentityLookup_WaitForActivation_Timeout(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	ctx := context.Background()
	idBucket, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_wait_timeout_identities",
	})
	require.NoError(t, err)

	cfg := newDefaultConfig()
	cfg.LockTTL = 500 * time.Millisecond

	il := &IdentityLookup{
		identities: idBucket,
		config:     cfg,
		semaphore:  make(chan struct{}, 200),
	}

	ci := &cluster.ClusterIdentity{Kind: "TestGrain", Identity: "timeout1"}

	// No one will store activation — should timeout
	rec := il.waitForActivation(ctx, ci)
	assert.Nil(t, rec)
}
```

**Step 2: Run test to verify it fails**

```bash
cd cluster/clusterproviders/natskv && go test -run TestIdentityLookup_InterfaceCompliance -v
```

Expected: FAIL — `IdentityLookup` type not defined.

**Step 3: Write natskv_identity.go**

```go
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

// Compile-time interface check.
var _ cluster.IdentityLookup = (*IdentityLookup)(nil)

// activationRecord is the JSON-encoded value stored in the identities KV bucket.
type activationRecord struct {
	LockID     string `json:"lid,omitempty"`
	PidID      string `json:"pid,omitempty"`
	PidAddress string `json:"adr,omitempty"`
	MemberID   string `json:"mid,omitempty"`
}

// memberRecord tracks identity keys owned by a member.
type memberRecord struct {
	Keys []string `json:"keys"`
}

// IdentityLookup implements cluster.IdentityLookup using NATS JetStream KV.
type IdentityLookup struct {
	provider      *Provider
	cluster       *cluster.Cluster
	memberID      string
	isClient      bool
	identities    jetstream.KeyValue
	memberTracker jetstream.KeyValue
	config        *config
	semaphore     chan struct{}
}

// newIdentityLookup creates a new IdentityLookup bound to the given provider.
func newIdentityLookup(p *Provider) *IdentityLookup {
	return &IdentityLookup{
		provider:  p,
		config:    p.config,
		semaphore: make(chan struct{}, p.config.MaxConcurrency),
	}
}

// Setup initializes the lookup with the cluster context. It creates the
// identity KV bucket and subscribes to topology events to clean up
// activations when members depart.
func (il *IdentityLookup) Setup(c *cluster.Cluster, kinds []string, isClient bool) {
	il.cluster = c
	il.isClient = isClient
	il.memberID = c.ActorSystem.ID

	ctx := context.Background()

	bucketName := il.config.identityBucketName(c.Config.Name)
	idBucket, err := il.provider.js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:   bucketName,
		Replicas: il.config.Replicas,
	})
	if err != nil {
		slog.Error("natskv identity: failed to create identity bucket",
			slog.String("bucket", bucketName), slog.Any("error", err))
		return
	}
	il.identities = idBucket

	memBucketName := bucketName + "_tracking"
	memBucket, err := il.provider.js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:   memBucketName,
		Replicas: il.config.Replicas,
	})
	if err != nil {
		slog.Error("natskv identity: failed to create member tracking bucket",
			slog.String("bucket", memBucketName), slog.Any("error", err))
		return
	}
	il.memberTracker = memBucket

	// Subscribe to topology events to clean up departing members' activations.
	c.ActorSystem.EventStream.Subscribe(func(evt any) {
		if topology, ok := evt.(*cluster.ClusterTopology); ok {
			for _, member := range topology.Left {
				il.removeMemberID(context.Background(), member.Id)
			}
		}
	})
}

// Shutdown cleans up activations owned by this member.
func (il *IdentityLookup) Shutdown() {
	if il.memberID != "" && il.memberTracker != nil {
		il.removeMemberID(context.Background(), il.memberID)
	}
}

// Get resolves a cluster identity to an actor PID.
//
// Protocol:
//  1. Check for existing activation
//  2. If client, wait for activation
//  3. Try acquire lock
//  4. If lock acquired, spawn actor, store activation
//  5. If lock not acquired, wait for activation
func (il *IdentityLookup) Get(ci *cluster.ClusterIdentity) *actor.PID {
	il.acquire()
	defer il.release()

	ctx := context.Background()

	// Step 1: Check for existing activation
	existing := il.getExistingActivation(ctx, ci)
	if existing != nil {
		return pidFromRecord(existing)
	}

	// Step 2: Clients wait for activation
	if il.isClient {
		rec := il.waitForActivation(ctx, ci)
		if rec != nil {
			return pidFromRecord(rec)
		}
		return nil
	}

	// Step 3: Try to acquire lock
	lockID, rev, ok := il.tryAcquireLock(ctx, ci)
	if !ok {
		// Another node is spawning. Wait for it.
		rec := il.waitForActivation(ctx, ci)
		if rec != nil {
			return pidFromRecord(rec)
		}
		return nil
	}

	// Step 4: We hold the lock — spawn the actor
	pid := il.spawnActivation(ci, lockID, rev)
	if pid == nil {
		// Spawn failed, remove lock
		key := kvKey(ci)
		_ = il.identities.Delete(ctx, key)
		return nil
	}

	return pid
}

// RemovePid removes the activation for a cluster identity.
func (il *IdentityLookup) RemovePid(ci *cluster.ClusterIdentity, pid *actor.PID) {
	il.acquire()
	defer il.release()

	ctx := context.Background()
	key := kvKey(ci)

	// Read entry to find member for tracking cleanup
	entry, err := il.identities.Get(ctx, key)
	if err != nil {
		return
	}

	var rec activationRecord
	if err := json.Unmarshal(entry.Value(), &rec); err == nil && rec.MemberID != "" {
		il.removeKeyFromMember(ctx, rec.MemberID, key)
	}

	_ = il.identities.Delete(ctx, key)
}

// kvKey converts a ClusterIdentity to a NATS KV-safe key.
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

	// Only return if activation is complete (lock cleared, PID set)
	if rec.PidID == "" || rec.PidAddress == "" || rec.MemberID == "" {
		return nil
	}

	return &rec
}

// tryAcquireLock attempts to atomically create a lock entry for the identity.
func (il *IdentityLookup) tryAcquireLock(ctx context.Context, ci *cluster.ClusterIdentity) (lockID string, revision uint64, ok bool) {
	lockID = uuid.New().String()
	key := kvKey(ci)

	rec := activationRecord{LockID: lockID}
	data, err := json.Marshal(&rec)
	if err != nil {
		return "", 0, false
	}

	rev, err := il.identities.Create(ctx, key, data)
	if err != nil {
		if !errors.Is(err, jetstream.ErrKeyExists) {
			slog.Error("natskv identity: lock failed",
				slog.String("key", key), slog.Any("error", err))
		}
		return "", 0, false
	}

	return lockID, rev, true
}

// storeActivation stores a completed activation, replacing the lock with the PID.
func (il *IdentityLookup) storeActivation(ctx context.Context, ci *cluster.ClusterIdentity, lockID string, revision uint64, memberID, pidAddress, pidID string) error {
	key := kvKey(ci)

	updated := activationRecord{
		LockID:     "",
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
		return fmt.Errorf("natskv identity: store activation CAS failed: %w", err)
	}

	// Track this key under the member
	if il.memberTracker != nil {
		il.addKeyToMember(ctx, memberID, key)
	}

	return nil
}

// waitForActivation watches the identity key until an activation appears or timeout.
func (il *IdentityLookup) waitForActivation(ctx context.Context, ci *cluster.ClusterIdentity) *activationRecord {
	key := kvKey(ci)
	watchCtx, cancel := context.WithTimeout(ctx, il.config.LockTTL)
	defer cancel()

	watcher, err := il.identities.Watch(watchCtx, key)
	if err != nil {
		slog.Error("natskv identity: watch failed",
			slog.String("key", key), slog.Any("error", err))
		return nil
	}
	defer watcher.Stop()

	for entry := range watcher.Updates() {
		if entry == nil {
			// Initial values done — continue waiting
			continue
		}

		if entry.Operation() == jetstream.KeyValueDelete || entry.Operation() == jetstream.KeyValuePurge {
			return nil
		}

		var rec activationRecord
		if err := json.Unmarshal(entry.Value(), &rec); err != nil {
			continue
		}

		// Activation complete when lock is cleared and PID is set
		if rec.LockID == "" && rec.PidID != "" {
			return &rec
		}
	}

	return nil
}

// removeMemberID removes all activations belonging to the given member.
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
		slog.Error("natskv identity: unmarshal member record failed",
			slog.String("memberID", memberID), slog.Any("error", err))
		return
	}

	for _, key := range mrec.Keys {
		if il.identities != nil {
			if err := il.identities.Delete(ctx, key); err != nil && !errors.Is(err, jetstream.ErrKeyNotFound) {
				slog.Error("natskv identity: delete identity failed",
					slog.String("key", key), slog.Any("error", err))
			}
		}
	}

	_ = il.memberTracker.Delete(ctx, memberID)
}

// addKeyToMember adds an identity key to a member's tracking record.
func (il *IdentityLookup) addKeyToMember(ctx context.Context, memberID, key string) {
	for i := 0; i < 3; i++ {
		entry, err := il.memberTracker.Get(ctx, memberID)
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			mrec := memberRecord{Keys: []string{key}}
			data, _ := json.Marshal(&mrec)
			_, err = il.memberTracker.Create(ctx, memberID, data)
			if err == nil {
				return
			}
			if errors.Is(err, jetstream.ErrKeyExists) {
				continue
			}
			slog.Error("natskv identity: create member record failed",
				slog.String("memberID", memberID), slog.Any("error", err))
			return
		}
		if err != nil {
			slog.Error("natskv identity: get member record failed",
				slog.String("memberID", memberID), slog.Any("error", err))
			return
		}

		var mrec memberRecord
		if err := json.Unmarshal(entry.Value(), &mrec); err != nil {
			return
		}

		for _, k := range mrec.Keys {
			if k == key {
				return
			}
		}

		mrec.Keys = append(mrec.Keys, key)
		data, _ := json.Marshal(&mrec)

		_, err = il.memberTracker.Update(ctx, memberID, data, entry.Revision())
		if err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// removeKeyFromMember removes an identity key from a member's tracking record.
func (il *IdentityLookup) removeKeyFromMember(ctx context.Context, memberID, key string) {
	if il.memberTracker == nil {
		return
	}

	entry, err := il.memberTracker.Get(ctx, memberID)
	if err != nil {
		return
	}

	var mrec memberRecord
	if err := json.Unmarshal(entry.Value(), &mrec); err != nil {
		return
	}

	filtered := mrec.Keys[:0]
	for _, k := range mrec.Keys {
		if k != key {
			filtered = append(filtered, k)
		}
	}
	mrec.Keys = filtered

	data, _ := json.Marshal(&mrec)
	_, _ = il.memberTracker.Update(ctx, memberID, data, entry.Revision())
}

// spawnActivation attempts to spawn an actor and store its activation.
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
		slog.Error("natskv identity: spawn failed",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity),
			slog.Any("error", err))
		return nil
	}

	ctx := context.Background()
	if err := il.storeActivation(ctx, ci, lockID, revision, il.memberID, pid.Address, pid.Id); err != nil {
		slog.Error("natskv identity: store activation failed",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity),
			slog.Any("error", err))
		// Kill the actor since we couldn't store activation
		il.cluster.ActorSystem.Root.Poison(pid)
		return nil
	}

	il.cluster.PidCache.Set(ci.Identity, ci.Kind, pid)

	return pid
}

// pidFromRecord converts an activationRecord to an actor.PID.
func pidFromRecord(rec *activationRecord) *actor.PID {
	return actor.NewPID(rec.PidAddress, rec.PidID)
}
```

**Step 4: Run tests to verify they pass**

```bash
cd cluster/clusterproviders/natskv && go test -run TestIdentityLookup -v
```

Expected: All PASS.

**Step 5: Commit**

```bash
git add cluster/clusterproviders/natskv/natskv_identity.go cluster/clusterproviders/natskv/natskv_identity_test.go
git commit -m "feat(natskv): add integrated IdentityLookup with lock, spawn, and member tracking"
```

---

## Task 8: Provider Unit Tests (Member Registration, Discovery, Shutdown)

**Files:**
- Modify: `cluster/clusterproviders/natskv/natskv_provider_test.go` (add tests)

**Step 1: Add comprehensive provider tests**

Append to `cluster/clusterproviders/natskv/natskv_provider_test.go`:

```go
func TestStartMember_RegistersSelfInKV(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc, js := connectNATS(t, srv)

	p, err := New(nc, WithMemberTTL(10*time.Second), WithRefreshInterval(1*time.Second))
	require.NoError(t, err)

	system := actor.NewActorSystem()
	remoteCfg := remote.Configure("127.0.0.1", 0)
	clusterCfg := cluster.Configure("test-cluster", p, disthash.New(), remoteCfg)
	c := cluster.NewCluster(system, clusterCfg)

	// Use the provider's StartMember directly
	err = p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	// Verify the key exists in KV
	ctx := context.Background()
	bucketName := p.config.memberBucketName("test-cluster")
	kv, err := js.KeyValue(ctx, bucketName)
	require.NoError(t, err)

	key := p.memberKey(p.self.ID)
	entry, err := kv.Get(ctx, key)
	require.NoError(t, err)

	var node Node
	require.NoError(t, json.Unmarshal(entry.Value(), &node))
	assert.Equal(t, p.self.ID, node.ID)
	assert.True(t, node.Alive)
}

func TestStartMember_DiscoversExistingMembers(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc1, js := connectNATS(t, srv)
	nc2, _ := connectNATS(t, srv)

	// Start first member
	p1, err := New(nc1, WithMemberTTL(10*time.Second))
	require.NoError(t, err)

	system1 := actor.NewActorSystem()
	remoteCfg1 := remote.Configure("127.0.0.1", 0)
	clusterCfg1 := cluster.Configure("test-cluster", p1, disthash.New(), remoteCfg1)
	c1 := cluster.NewCluster(system1, clusterCfg1)

	err = p1.StartMember(c1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p1.Shutdown(true) })

	// Verify key is in KV
	ctx := context.Background()
	bucketName := p1.config.memberBucketName("test-cluster")
	kv, err := js.KeyValue(ctx, bucketName)
	require.NoError(t, err)

	key1 := p1.memberKey(p1.self.ID)
	_, err = kv.Get(ctx, key1)
	require.NoError(t, err)

	// Start second member — should discover the first
	p2, err := New(nc2, WithMemberTTL(10*time.Second))
	require.NoError(t, err)

	system2 := actor.NewActorSystem()
	remoteCfg2 := remote.Configure("127.0.0.1", 0)
	clusterCfg2 := cluster.Configure("test-cluster", p2, disthash.New(), remoteCfg2)
	c2 := cluster.NewCluster(system2, clusterCfg2)

	err = p2.StartMember(c2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	// p2 should have discovered p1
	p2.membersMu.RLock()
	_, found := p2.members[p1.self.ID]
	p2.membersMu.RUnlock()
	assert.True(t, found, "second member should discover first member")
}

func TestStartClient_WatchOnly(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc1, js := connectNATS(t, srv)
	nc2, _ := connectNATS(t, srv)

	// Start a member first
	p1, err := New(nc1, WithMemberTTL(10*time.Second))
	require.NoError(t, err)

	system1 := actor.NewActorSystem()
	remoteCfg1 := remote.Configure("127.0.0.1", 0)
	clusterCfg1 := cluster.Configure("test-cluster", p1, disthash.New(), remoteCfg1)
	c1 := cluster.NewCluster(system1, clusterCfg1)

	err = p1.StartMember(c1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p1.Shutdown(true) })

	// Start a client
	p2, err := New(nc2, WithMemberTTL(10*time.Second))
	require.NoError(t, err)

	system2 := actor.NewActorSystem()
	remoteCfg2 := remote.Configure("127.0.0.1", 0)
	clusterCfg2 := cluster.Configure("test-cluster", p2, disthash.New(), remoteCfg2)
	c2 := cluster.NewCluster(system2, clusterCfg2)

	err = p2.StartClient(c2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	// Client should have no self key in KV
	assert.Nil(t, p2.self, "client should not have self node")

	// But should have discovered the member
	ctx := context.Background()
	bucketName := p2.config.memberBucketName("test-cluster")
	kv, err := js.KeyValue(ctx, bucketName)
	require.NoError(t, err)

	// Member key should exist
	key1 := p1.memberKey(p1.self.ID)
	_, err = kv.Get(ctx, key1)
	require.NoError(t, err)
}

func TestShutdown_Graceful(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc, js := connectNATS(t, srv)

	p, err := New(nc, WithMemberTTL(10*time.Second))
	require.NoError(t, err)

	system := actor.NewActorSystem()
	remoteCfg := remote.Configure("127.0.0.1", 0)
	clusterCfg := cluster.Configure("test-cluster", p, disthash.New(), remoteCfg)
	c := cluster.NewCluster(system, clusterCfg)

	err = p.StartMember(c)
	require.NoError(t, err)

	selfID := p.self.ID
	key := p.memberKey(selfID)

	// Shutdown
	err = p.Shutdown(true)
	require.NoError(t, err)

	// Key should be deleted
	ctx := context.Background()
	bucketName := p.config.memberBucketName("test-cluster")
	kv, err := js.KeyValue(ctx, bucketName)
	require.NoError(t, err)

	_, err = kv.Get(ctx, key)
	assert.Error(t, err, "member key should be deleted after shutdown")
}

func TestShutdown_Idempotent(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc, _ := connectNATS(t, srv)

	p, err := New(nc, WithMemberTTL(10*time.Second))
	require.NoError(t, err)

	system := actor.NewActorSystem()
	remoteCfg := remote.Configure("127.0.0.1", 0)
	clusterCfg := cluster.Configure("test-cluster", p, disthash.New(), remoteCfg)
	c := cluster.NewCluster(system, clusterCfg)

	err = p.StartMember(c)
	require.NoError(t, err)

	// Double shutdown should not panic
	require.NoError(t, p.Shutdown(true))
	require.NoError(t, p.Shutdown(true))
}

func TestLeaderElection_FirstWins(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc1, _ := connectNATS(t, srv)
	nc2, _ := connectNATS(t, srv)

	p1, err := New(nc1, WithMemberTTL(10*time.Second), WithLeaderTTL(10*time.Second))
	require.NoError(t, err)

	system1 := actor.NewActorSystem()
	remoteCfg1 := remote.Configure("127.0.0.1", 0)
	clusterCfg1 := cluster.Configure("test-cluster", p1, disthash.New(), remoteCfg1)
	c1 := cluster.NewCluster(system1, clusterCfg1)

	err = p1.StartMember(c1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p1.Shutdown(true) })

	p2, err := New(nc2, WithMemberTTL(10*time.Second), WithLeaderTTL(10*time.Second))
	require.NoError(t, err)

	system2 := actor.NewActorSystem()
	remoteCfg2 := remote.Configure("127.0.0.1", 0)
	clusterCfg2 := cluster.Configure("test-cluster", p2, disthash.New(), remoteCfg2)
	c2 := cluster.NewCluster(system2, clusterCfg2)

	err = p2.StartMember(c2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	// Exactly one should be leader
	leaderCount := 0
	if p1.isLeader.Load() {
		leaderCount++
	}
	if p2.isLeader.Load() {
		leaderCount++
	}
	assert.Equal(t, 1, leaderCount, "exactly one member should be leader")
}

func TestRoleChangedListener_Called(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc, _ := connectNATS(t, srv)

	var lastRole RoleType = -1
	listener := &mockRoleListener{callback: func(r RoleType) { lastRole = r }}

	p, err := New(nc,
		WithMemberTTL(10*time.Second),
		WithRoleChangedListener(listener),
	)
	require.NoError(t, err)

	system := actor.NewActorSystem()
	remoteCfg := remote.Configure("127.0.0.1", 0)
	clusterCfg := cluster.Configure("test-cluster", p, disthash.New(), remoteCfg)
	c := cluster.NewCluster(system, clusterCfg)

	err = p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	// Wait a moment for role notification
	time.Sleep(200 * time.Millisecond)

	// First member should become leader
	assert.Equal(t, Leader, lastRole)
}

type mockRoleListener struct {
	callback func(RoleType)
}

func (m *mockRoleListener) OnRoleChanged(r RoleType) {
	m.callback(r)
}
```

Note: These tests require additional imports. Add to the imports at the top of the test file:

```go
import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
```

**Step 2: Run tests**

```bash
cd cluster/clusterproviders/natskv && go test -run TestStartMember -v -timeout 30s
```

Expected: Tests pass (may need to add `disthash` and `remote` dependencies — run `go mod tidy` first).

**Step 3: Run all provider tests**

```bash
cd cluster/clusterproviders/natskv && go test -v -timeout 60s
```

Expected: All PASS.

**Step 4: Commit**

```bash
git add cluster/clusterproviders/natskv/natskv_provider_test.go
git commit -m "test(natskv): add provider unit tests for member registration, discovery, and leader election"
```

---

## Task 9: Member Crash Detection Test (TTL Expiry)

**Files:**
- Modify: `cluster/clusterproviders/natskv/natskv_provider_test.go` (add test)

**Step 1: Add crash detection test**

Append to `natskv_provider_test.go`:

```go
func TestMemberCrash_TTLExpiry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TTL expiry test in short mode")
	}

	srv := startEmbeddedNATS(t)
	nc1, _ := connectNATS(t, srv)
	nc2, _ := connectNATS(t, srv)

	shortTTL := 2 * time.Second

	// Start first member with short TTL
	p1, err := New(nc1, WithMemberTTL(shortTTL), WithRefreshInterval(500*time.Millisecond))
	require.NoError(t, err)

	system1 := actor.NewActorSystem()
	remoteCfg1 := remote.Configure("127.0.0.1", 0)
	clusterCfg1 := cluster.Configure("test-cluster", p1, disthash.New(), remoteCfg1)
	c1 := cluster.NewCluster(system1, clusterCfg1)

	err = p1.StartMember(c1)
	require.NoError(t, err)

	// Start second member
	p2, err := New(nc2, WithMemberTTL(shortTTL), WithRefreshInterval(500*time.Millisecond))
	require.NoError(t, err)

	system2 := actor.NewActorSystem()
	remoteCfg2 := remote.Configure("127.0.0.1", 0)
	clusterCfg2 := cluster.Configure("test-cluster", p2, disthash.New(), remoteCfg2)
	c2 := cluster.NewCluster(system2, clusterCfg2)

	err = p2.StartMember(c2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	// Verify p2 sees p1
	time.Sleep(500 * time.Millisecond)
	p2.membersMu.RLock()
	_, found := p2.members[p1.self.ID]
	p2.membersMu.RUnlock()
	require.True(t, found, "p2 should see p1")

	// "Crash" p1 by shutting down without deleting key (simulate crash)
	p1.cancel() // Cancel context to stop refresh goroutine
	p1.shutdown.Store(true)
	p1.wg.Wait()

	// Wait for TTL to expire
	time.Sleep(shortTTL + 2*time.Second)

	// p2 should no longer see p1
	p2.membersMu.RLock()
	_, found = p2.members[p1.self.ID]
	p2.membersMu.RUnlock()
	assert.False(t, found, "p2 should detect p1's departure after TTL expiry")
}
```

**Step 2: Run test**

```bash
cd cluster/clusterproviders/natskv && go test -run TestMemberCrash_TTLExpiry -v -timeout 30s
```

Expected: PASS (may take ~5s due to TTL wait).

**Step 3: Commit**

```bash
git add cluster/clusterproviders/natskv/natskv_provider_test.go
git commit -m "test(natskv): add TTL-based crash detection test"
```

---

## Task 10: Singleton Scheduler Integration Test

**Files:**
- Modify: `cluster/clusterproviders/natskv/natskv_provider_test.go` (add test)

**Step 1: Add singleton scheduler test**

```go
func TestSingletonScheduler_SpawnOnLeader(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc, _ := connectNATS(t, srv)

	p, err := New(nc, WithMemberTTL(10*time.Second))
	require.NoError(t, err)

	system := actor.NewActorSystem()
	remoteCfg := remote.Configure("127.0.0.1", 0)
	clusterCfg := cluster.Configure("test-cluster", p, disthash.New(), remoteCfg)
	c := cluster.NewCluster(system, clusterCfg)

	var spawned atomic.Bool
	scheduler := NewSingletonScheduler(system.Root)
	scheduler.FromFunc(func(ctx actor.Context) {
		if _, ok := ctx.Message().(*actor.Started); ok {
			spawned.Store(true)
		}
	})
	p.RegisterSingletonScheduler(scheduler)

	err = p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	// As the only member, should become leader and spawn singleton
	time.Sleep(500 * time.Millisecond)
	assert.True(t, spawned.Load(), "singleton should be spawned on leader")
}
```

Add `"sync/atomic"` to imports if not already present.

**Step 2: Run test**

```bash
cd cluster/clusterproviders/natskv && go test -run TestSingletonScheduler -v -timeout 15s
```

Expected: PASS.

**Step 3: Commit**

```bash
git add cluster/clusterproviders/natskv/natskv_provider_test.go
git commit -m "test(natskv): add singleton scheduler spawn-on-leader test"
```

---

## Task 11: go.mod Cleanup and Compilation Verification

**Files:**
- Modify: `cluster/clusterproviders/natskv/go.mod` (tidy)

**Step 1: Run go mod tidy**

```bash
cd cluster/clusterproviders/natskv && go mod tidy
```

**Step 2: Run go vet**

```bash
cd cluster/clusterproviders/natskv && go vet ./...
```

Expected: No issues.

**Step 3: Run all tests**

```bash
cd cluster/clusterproviders/natskv && go test -v -timeout 120s ./...
```

Expected: All PASS.

**Step 4: Commit**

```bash
git add cluster/clusterproviders/natskv/go.mod cluster/clusterproviders/natskv/go.sum
git commit -m "chore(natskv): tidy go.mod dependencies"
```

---

## Task 12: Integration Tests with Testcontainers

**Files:**
- Create: `cluster/clusterproviders/natskv/natskv_integration_test.go`

**Step 1: Write integration tests**

```go
//go:build integration

package natskv

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/remote"
)

func startNATSContainer(t *testing.T) string {
	t.Helper()

	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "nats:latest",
		ExposedPorts: []string{"4222/tcp"},
		Cmd:          []string{"-js"},
		WaitingFor:   wait.ForListeningPort("4222/tcp").WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	endpoint, err := container.Endpoint(ctx, "")
	require.NoError(t, err)

	return "nats://" + endpoint
}

func TestIntegration_TwoMemberCluster(t *testing.T) {
	url := startNATSContainer(t)

	nc1, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(func() { nc1.Close() })

	nc2, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(func() { nc2.Close() })

	p1, err := New(nc1, WithMemberTTL(5*time.Second))
	require.NoError(t, err)

	system1 := actor.NewActorSystem()
	remoteCfg1 := remote.Configure("127.0.0.1", 0)
	clusterCfg1 := cluster.Configure("integration-test", p1, disthash.New(), remoteCfg1)
	c1 := cluster.NewCluster(system1, clusterCfg1)

	err = p1.StartMember(c1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p1.Shutdown(true) })

	p2, err := New(nc2, WithMemberTTL(5*time.Second))
	require.NoError(t, err)

	system2 := actor.NewActorSystem()
	remoteCfg2 := remote.Configure("127.0.0.1", 0)
	clusterCfg2 := cluster.Configure("integration-test", p2, disthash.New(), remoteCfg2)
	c2 := cluster.NewCluster(system2, clusterCfg2)

	err = p2.StartMember(c2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	// Wait for topology convergence
	time.Sleep(2 * time.Second)

	// Both should see each other
	p1.membersMu.RLock()
	_, p1SeesP2 := p1.members[p2.self.ID]
	p1.membersMu.RUnlock()

	p2.membersMu.RLock()
	_, p2SeesP1 := p2.members[p1.self.ID]
	p2.membersMu.RUnlock()

	assert.True(t, p1SeesP2, "p1 should see p2")
	assert.True(t, p2SeesP1, "p2 should see p1")
}

func TestIntegration_MemberJoinLeave(t *testing.T) {
	url := startNATSContainer(t)

	nc1, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(func() { nc1.Close() })

	nc2, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(func() { nc2.Close() })

	// Start member 1
	p1, err := New(nc1, WithMemberTTL(5*time.Second))
	require.NoError(t, err)

	system1 := actor.NewActorSystem()
	remoteCfg1 := remote.Configure("127.0.0.1", 0)
	clusterCfg1 := cluster.Configure("join-leave-test", p1, disthash.New(), remoteCfg1)
	c1 := cluster.NewCluster(system1, clusterCfg1)

	err = p1.StartMember(c1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p1.Shutdown(true) })

	// Start member 2
	p2, err := New(nc2, WithMemberTTL(5*time.Second))
	require.NoError(t, err)

	system2 := actor.NewActorSystem()
	remoteCfg2 := remote.Configure("127.0.0.1", 0)
	clusterCfg2 := cluster.Configure("join-leave-test", p2, disthash.New(), remoteCfg2)
	c2 := cluster.NewCluster(system2, clusterCfg2)

	err = p2.StartMember(c2)
	require.NoError(t, err)

	// Wait for topology
	time.Sleep(2 * time.Second)
	p1.membersMu.RLock()
	_, found := p1.members[p2.self.ID]
	p1.membersMu.RUnlock()
	require.True(t, found, "p1 should see p2")

	// Gracefully shut down member 2
	require.NoError(t, p2.Shutdown(true))

	// Wait for watcher to detect departure
	time.Sleep(2 * time.Second)

	p1.membersMu.RLock()
	_, found = p1.members[p2.self.ID]
	p1.membersMu.RUnlock()
	assert.False(t, found, "p1 should detect p2's graceful departure")
}

func TestIntegration_LeaderElection_ThreeNodes(t *testing.T) {
	url := startNATSContainer(t)

	providers := make([]*Provider, 3)
	for i := 0; i < 3; i++ {
		nc, err := nats.Connect(url)
		require.NoError(t, err)
		t.Cleanup(func() { nc.Close() })

		p, err := New(nc, WithMemberTTL(5*time.Second), WithLeaderTTL(10*time.Second))
		require.NoError(t, err)

		system := actor.NewActorSystem()
		remoteCfg := remote.Configure("127.0.0.1", 0)
		clusterCfg := cluster.Configure("leader-test", p, disthash.New(), remoteCfg)
		c := cluster.NewCluster(system, clusterCfg)

		err = p.StartMember(c)
		require.NoError(t, err)
		t.Cleanup(func() { _ = p.Shutdown(true) })

		providers[i] = p
	}

	time.Sleep(2 * time.Second)

	leaderCount := 0
	for _, p := range providers {
		if p.isLeader.Load() {
			leaderCount++
		}
	}
	assert.Equal(t, 1, leaderCount, "exactly one of three members should be leader")
}
```

**Step 2: Run integration tests (requires Docker)**

```bash
cd cluster/clusterproviders/natskv && go test -tags=integration -run TestIntegration -v -timeout 120s
```

Expected: All PASS.

**Step 3: Commit**

```bash
git add cluster/clusterproviders/natskv/natskv_integration_test.go
git commit -m "test(natskv): add integration tests with testcontainers"
```

---

## Task 13: Single-Node Example

**Files:**
- Create: `examples/cluster-nats-kv/main.go`
- Create: `examples/cluster-nats-kv/docker-compose.yml`
- Create: `examples/cluster-nats-kv/go.mod`

**Reference:** `examples/nats-jetstream-virtual-actor-ingress/`

**Step 1: Create directory structure**

```bash
mkdir -p examples/cluster-nats-kv
```

**Step 2: Write docker-compose.yml**

```yaml
version: "3"
services:
  nats:
    image: nats:2-alpine
    ports:
      - "4222:4222"
      - "8222:8222"
    command: ["-js", "-m", "8222"]
```

**Step 3: Write go.mod**

```go
module github.com/asynkron/protoactor-go/examples/cluster-nats-kv

go 1.25.3

require (
	github.com/asynkron/protoactor-go v0.0.0
	github.com/asynkron/protoactor-go/cluster/clusterproviders/natskv v0.0.0
	github.com/nats-io/nats.go v1.48.0
)

replace (
	github.com/asynkron/protoactor-go => ../../
	github.com/asynkron/protoactor-go/cluster/clusterproviders/natskv => ../../cluster/clusterproviders/natskv
)
```

**Step 4: Write main.go**

```go
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/natskv"
	"github.com/asynkron/protoactor-go/remote"
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

	system := actor.NewActorSystem()

	provider, err := natskv.New(nc)
	if err != nil {
		log.Fatalf("Failed to create NATS KV provider: %v", err)
	}

	lookup := provider.IdentityLookup()
	remoteCfg := remote.Configure("127.0.0.1", 0)

	clusterCfg := cluster.Configure("example-cluster", provider, lookup, remoteCfg)
	c := cluster.NewCluster(system, clusterCfg)

	if err := c.StartMember(); err != nil {
		log.Fatalf("Failed to start cluster member: %v", err)
	}

	fmt.Println("Cluster member started. Press Ctrl+C to stop.")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("Shutting down...")
	c.Shutdown(true)
}
```

**Step 5: Run go mod tidy**

```bash
cd examples/cluster-nats-kv && go mod tidy
```

**Step 6: Verify compilation**

```bash
cd examples/cluster-nats-kv && go build ./...
```

Expected: Compiles successfully.

**Step 7: Commit**

```bash
git add examples/cluster-nats-kv/
git commit -m "feat(examples): add single-node NATS KV cluster example"
```

---

## Task 14: Multi-Node Example

**Files:**
- Create: `examples/cluster-nats-kv-multi/node/main.go`
- Create: `examples/cluster-nats-kv-multi/client/main.go`
- Create: `examples/cluster-nats-kv-multi/docker-compose.yml`
- Create: `examples/cluster-nats-kv-multi/go.mod`

**Step 1: Create directory structure**

```bash
mkdir -p examples/cluster-nats-kv-multi/node examples/cluster-nats-kv-multi/client
```

**Step 2: Write docker-compose.yml**

```yaml
version: "3"
services:
  nats:
    image: nats:2-alpine
    ports:
      - "4222:4222"
    command: ["-js"]

  node1:
    build:
      context: .
      dockerfile: Dockerfile.node
    environment:
      - NATS_URL=nats://nats:4222
      - NODE_HOST=node1
      - NODE_PORT=8080
    depends_on:
      - nats

  node2:
    build:
      context: .
      dockerfile: Dockerfile.node
    environment:
      - NATS_URL=nats://nats:4222
      - NODE_HOST=node2
      - NODE_PORT=8080
    depends_on:
      - nats

  node3:
    build:
      context: .
      dockerfile: Dockerfile.node
    environment:
      - NATS_URL=nats://nats:4222
      - NODE_HOST=node3
      - NODE_PORT=8080
    depends_on:
      - nats

  client:
    build:
      context: .
      dockerfile: Dockerfile.client
    environment:
      - NATS_URL=nats://nats:4222
    depends_on:
      - node1
      - node2
      - node3
```

**Step 3: Write go.mod**

```go
module github.com/asynkron/protoactor-go/examples/cluster-nats-kv-multi

go 1.25.3

require (
	github.com/asynkron/protoactor-go v0.0.0
	github.com/asynkron/protoactor-go/cluster/clusterproviders/natskv v0.0.0
	github.com/nats-io/nats.go v1.48.0
)

replace (
	github.com/asynkron/protoactor-go => ../../
	github.com/asynkron/protoactor-go/cluster/clusterproviders/natskv => ../../cluster/clusterproviders/natskv
)
```

**Step 4: Write node/main.go**

```go
package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/natskv"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/nats-io/nats.go"
)

// HelloGrain is a simple virtual actor.
type HelloGrain struct{}

func (h *HelloGrain) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		slog.Info("HelloGrain started")
	case *actor.Stopping:
		slog.Info("HelloGrain stopping")
	case string:
		slog.Info("HelloGrain received", slog.String("message", msg))
		ctx.Respond(fmt.Sprintf("Hello from %s!", msg))
	}
}

func main() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}
	host := os.Getenv("NODE_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	portStr := os.Getenv("NODE_PORT")
	port := 0
	if portStr != "" {
		port, _ = strconv.Atoi(portStr)
	}

	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	system := actor.NewActorSystem()

	provider, err := natskv.New(nc)
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	lookup := provider.IdentityLookup()
	remoteCfg := remote.Configure(host, port)

	helloKind := cluster.NewKind("HelloGrain", actor.PropsFromProducer(func() actor.Actor {
		return &HelloGrain{}
	}))

	clusterCfg := cluster.Configure("multi-example", provider, lookup, remoteCfg,
		cluster.WithKinds(helloKind),
	)
	c := cluster.NewCluster(system, clusterCfg)

	if err := c.StartMember(); err != nil {
		log.Fatalf("Failed to start: %v", err)
	}

	slog.Info("Node started", slog.String("host", host), slog.Int("port", port))

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	c.Shutdown(true)
}
```

**Step 5: Write client/main.go**

```go
package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/natskv"
	"github.com/asynkron/protoactor-go/remote"
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

	system := actor.NewActorSystem()

	provider, err := natskv.New(nc)
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	lookup := provider.IdentityLookup()
	remoteCfg := remote.Configure("127.0.0.1", 0)

	clusterCfg := cluster.Configure("multi-example", provider, lookup, remoteCfg)
	c := cluster.NewCluster(system, clusterCfg)

	if err := c.StartClient(); err != nil {
		log.Fatalf("Failed to start client: %v", err)
	}

	slog.Info("Client started, waiting for cluster topology...")
	time.Sleep(3 * time.Second)

	// Make grain calls
	for i := 0; i < 5; i++ {
		ci := &cluster.ClusterIdentity{Kind: "HelloGrain", Identity: fmt.Sprintf("grain-%d", i)}
		pid := c.Get(ci.Identity, ci.Kind)
		if pid == nil {
			slog.Warn("Failed to resolve grain", slog.String("identity", ci.Identity))
			continue
		}
		slog.Info("Resolved grain",
			slog.String("identity", ci.Identity),
			slog.String("pid", pid.String()))
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	c.Shutdown(true)
}
```

**Step 6: Run go mod tidy**

```bash
cd examples/cluster-nats-kv-multi && go mod tidy
```

**Step 7: Verify compilation**

```bash
cd examples/cluster-nats-kv-multi && go build ./node/... && go build ./client/...
```

Expected: Compiles successfully.

**Step 8: Commit**

```bash
git add examples/cluster-nats-kv-multi/
git commit -m "feat(examples): add multi-node NATS KV cluster example with docker-compose"
```

---

## Task 15: Final Verification

**Step 1: Run all unit tests**

```bash
cd cluster/clusterproviders/natskv && go test -v -timeout 120s ./...
```

Expected: All PASS.

**Step 2: Run go vet**

```bash
cd cluster/clusterproviders/natskv && go vet ./...
```

Expected: No issues.

**Step 3: Verify examples compile**

```bash
cd examples/cluster-nats-kv && go build ./... && cd ../cluster-nats-kv-multi && go build ./node/... && go build ./client/...
```

Expected: All compile.

**Step 4: Run integration tests (if Docker available)**

```bash
cd cluster/clusterproviders/natskv && go test -tags=integration -v -timeout 120s ./...
```

Expected: All PASS.

**Step 5: Commit any final changes**

```bash
git add -A && git status
```

If clean, no commit needed.

---

## Summary of Files Created

| File | Purpose |
|------|---------|
| `cluster/clusterproviders/natskv/go.mod` | Module definition with dependencies |
| `cluster/clusterproviders/natskv/doc.go` | Package documentation |
| `cluster/clusterproviders/natskv/config.go` | Options pattern configuration |
| `cluster/clusterproviders/natskv/config_test.go` | Config unit tests |
| `cluster/clusterproviders/natskv/node.go` | Node struct with serialization |
| `cluster/clusterproviders/natskv/node_test.go` | Node unit tests |
| `cluster/clusterproviders/natskv/singleton.go` | SingletonScheduler + RoleType |
| `cluster/clusterproviders/natskv/singleton_test.go` | Singleton unit tests |
| `cluster/clusterproviders/natskv/testhelpers_test.go` | Embedded NATS test helpers |
| `cluster/clusterproviders/natskv/natskv_provider.go` | ClusterProvider implementation |
| `cluster/clusterproviders/natskv/natskv_provider_test.go` | Provider unit tests |
| `cluster/clusterproviders/natskv/natskv_identity.go` | IdentityLookup implementation |
| `cluster/clusterproviders/natskv/natskv_identity_test.go` | Identity lookup unit tests |
| `cluster/clusterproviders/natskv/natskv_integration_test.go` | Integration tests |
| `examples/cluster-nats-kv/main.go` | Single-node example |
| `examples/cluster-nats-kv/docker-compose.yml` | Single-node docker-compose |
| `examples/cluster-nats-kv/go.mod` | Example module |
| `examples/cluster-nats-kv-multi/node/main.go` | Multi-node member |
| `examples/cluster-nats-kv-multi/client/main.go` | Multi-node client |
| `examples/cluster-nats-kv-multi/docker-compose.yml` | Multi-node docker-compose |
| `examples/cluster-nats-kv-multi/go.mod` | Example module |

## Implementation Notes for the Executing Agent

1. **NATS KV Per-Key TTL:** The `jetstream.KeyTTL(duration)` option on `Put` requires the KV bucket to be created with a `TTL` field (acts as `MaxAge` on the underlying stream). The bucket-level `TTL` is set to `MemberTTL * 3` as a safety net. Per-key TTL overrides this for individual keys.

2. **Watcher Patterns:** NATS KV uses `>` as the multi-level wildcard in key patterns (not `*`). Example: `cluster.members.>` matches all member keys.

3. **Leader Election Atomicity:** `kv.Create()` provides atomic "create-if-not-exists" semantics. This is the foundation for leader election. If `Create` succeeds, the caller is the leader. If it returns `jetstream.ErrKeyExists`, another member is already leader.

4. **The `StartClient` flow:** Clients do NOT call `registerSelf()` or `startRefresh()`. They only watch. The `init()` sets `p.self = nil` for clients, and `publishClusterTopologyEvent()` only includes self if `p.self != nil`.
   - **Important correction:** Looking at the etcd provider, `StartClient` does call `init()` which creates a self node. But the client does NOT register in KV. The design doc says "Same as StartMember but skips self-registration and refresh. Watch-only mode." So `init()` IS called but `registerSelf()` is NOT called. Adjust the `StartClient` implementation accordingly — `p.self` will be set (for topology purposes) but no KV key is written.

5. **Import paths:** The provider tests need `disthash` and `remote` imports. These are in separate modules that use `replace` directives. Add them to `go.mod` requires.

6. **Cluster.Get vs IdentityLookup.Get:** `cluster.Get(identity, kind)` calls through to the configured `IdentityLookup.Get()`. The examples use this path.

7. **ClusterTopology event:** The `cluster.MemberList.UpdateClusterTopology()` call computes joined/left diff and publishes a `*cluster.ClusterTopology` event on the event stream. The identity lookup subscribes to this event to detect member departures.
