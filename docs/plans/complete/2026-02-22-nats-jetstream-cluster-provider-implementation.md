# NATS JetStream Cluster Provider Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a NATS JetStream raw-stream-based cluster provider for protoactor-go with integrated identity lookup, leader election, singleton scheduling, and local timeout-based crash detection.

**Architecture:** Uses raw JetStream streams (not KV) with `MaxMsgsPerSubject: 1` for compaction. Members publish heartbeats with per-message TTL. An ordered consumer receives all cluster events. A local `map[string]time.Time` tracks member liveness with periodic stale checks. Leader election uses publish-race CAS via `WithExpectLastSubjectSequence`. An integrated `IdentityLookup` uses a separate stream for activation records with CAS-based locking.

**Tech Stack:** Go 1.25.3, `nats.go v1.48.0`, `nats-server/v2 v2.12.4`, `testcontainers-go`, `testify`, `google/uuid`

**Design Document:** `docs/plans/2026-02-22-nats-jetstream-cluster-provider-design.md`

**Reference Implementation:** `cluster/clusterproviders/natskv/` — follow its patterns closely but adapt KV operations to raw stream operations.

---

## Task 1: Module Setup and Config

**Files:**
- Create: `cluster/clusterproviders/natsstream/go.mod`
- Create: `cluster/clusterproviders/natsstream/config.go`
- Test: `cluster/clusterproviders/natsstream/config_test.go`

**Step 1: Create `go.mod`**

```go
module github.com/awevoke/protoactor-go/cluster/clusterproviders/natsstream

go 1.25.3

require (
	github.com/awevoke/protoactor-go v0.0.0
	github.com/google/uuid v1.6.0
	github.com/nats-io/nats-server/v2 v2.12.4
	github.com/nats-io/nats.go v1.48.0
	github.com/stretchr/testify v1.11.1
	github.com/testcontainers/testcontainers-go v0.40.0
)

replace github.com/awevoke/protoactor-go => ../../../
```

**Step 2: Write the config tests**

```go
package natsstream

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	cfg := newDefaultConfig()
	assert.Equal(t, defaultHeartbeatInterval, cfg.HeartbeatInterval)
	assert.Equal(t, defaultHeartbeatTTL, cfg.HeartbeatTTL)
	assert.Equal(t, defaultMemberTimeout, cfg.MemberTimeout)
	assert.Equal(t, defaultCheckInterval, cfg.CheckInterval)
	assert.Equal(t, defaultLeaderTTL, cfg.LeaderTTL)
	assert.Equal(t, defaultLockTTL, cfg.LockTTL)
	assert.Equal(t, defaultMaxConcurrency, cfg.MaxConcurrency)
	assert.Equal(t, defaultReplicas, cfg.Replicas)
	assert.Equal(t, defaultSubjectPrefix, cfg.SubjectPrefix)
	assert.Equal(t, defaultRetryInterval, cfg.RetryInterval)
	assert.Equal(t, defaultMaxAge, cfg.MaxAge)
}

func TestWithOptions(t *testing.T) {
	cfg := newDefaultConfig()

	WithHeartbeatInterval(1 * time.Second)(cfg)
	assert.Equal(t, 1*time.Second, cfg.HeartbeatInterval)

	WithHeartbeatTTL(3 * time.Second)(cfg)
	assert.Equal(t, 3*time.Second, cfg.HeartbeatTTL)

	WithMemberTimeout(6 * time.Second)(cfg)
	assert.Equal(t, 6*time.Second, cfg.MemberTimeout)

	WithCheckInterval(2 * time.Second)(cfg)
	assert.Equal(t, 2*time.Second, cfg.CheckInterval)

	WithLeaderTTL(15 * time.Second)(cfg)
	assert.Equal(t, 15*time.Second, cfg.LeaderTTL)

	WithLockTTL(10 * time.Second)(cfg)
	assert.Equal(t, 10*time.Second, cfg.LockTTL)

	WithMaxConcurrency(100)(cfg)
	assert.Equal(t, 100, cfg.MaxConcurrency)

	WithReplicas(3)(cfg)
	assert.Equal(t, 3, cfg.Replicas)

	WithStreamName("MY_STREAM")(cfg)
	assert.Equal(t, "MY_STREAM", cfg.StreamName)

	WithIdentityStreamName("MY_IDENTITIES")(cfg)
	assert.Equal(t, "MY_IDENTITIES", cfg.IdentityStreamName)

	WithSubjectPrefix("myapp.cluster")(cfg)
	assert.Equal(t, "myapp.cluster", cfg.SubjectPrefix)

	WithRetryInterval(5 * time.Second)(cfg)
	assert.Equal(t, 5*time.Second, cfg.RetryInterval)

	WithMaxAge(2 * time.Hour)(cfg)
	assert.Equal(t, 2*time.Hour, cfg.MaxAge)
}

func TestStreamName_Default(t *testing.T) {
	cfg := newDefaultConfig()
	assert.Equal(t, "PROTOACTOR_mycluster", cfg.streamName("mycluster"))
}

func TestStreamName_Custom(t *testing.T) {
	cfg := newDefaultConfig()
	cfg.StreamName = "CUSTOM"
	assert.Equal(t, "CUSTOM", cfg.streamName("mycluster"))
}

func TestIdentityStreamName_Default(t *testing.T) {
	cfg := newDefaultConfig()
	assert.Equal(t, "PROTOACTOR_mycluster_IDENTITIES", cfg.identityStreamName("mycluster"))
}

func TestIdentityStreamName_Custom(t *testing.T) {
	cfg := newDefaultConfig()
	cfg.IdentityStreamName = "CUSTOM_ID"
	assert.Equal(t, "CUSTOM_ID", cfg.identityStreamName("mycluster"))
}

func TestSubjectPrefix_Default(t *testing.T) {
	cfg := newDefaultConfig()
	assert.Equal(t, "cluster.mycluster", cfg.subjectPrefix("mycluster"))
}

func TestSubjectPrefix_Custom(t *testing.T) {
	cfg := newDefaultConfig()
	cfg.SubjectPrefix = "myapp"
	assert.Equal(t, "myapp", cfg.subjectPrefix("mycluster"))
}

func TestRoleType_String(t *testing.T) {
	assert.Equal(t, "Follower", Follower.String())
	assert.Equal(t, "Leader", Leader.String())
}
```

**Step 3: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/protoactor-go/cluster/clusterproviders/natsstream && go test -run TestDefaultConfig -v`
Expected: Compilation error — types not defined yet

**Step 4: Write minimal `config.go` implementation**

```go
package natsstream

import (
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

const (
	defaultHeartbeatInterval = 2 * time.Second
	defaultHeartbeatTTL      = 5 * time.Second
	defaultMemberTimeout     = 8 * time.Second
	defaultCheckInterval     = 3 * time.Second
	defaultLeaderTTL         = 10 * time.Second
	defaultLockTTL           = 5 * time.Second
	defaultMaxConcurrency    = 200
	defaultReplicas          = 1
	defaultSubjectPrefix     = "cluster"
	defaultRetryInterval     = 1 * time.Second
	defaultMaxAge            = 1 * time.Hour
)

// RoleType represents the leadership role of a node in the cluster.
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

// RoleChangedListener receives notifications when the node's leadership role changes.
type RoleChangedListener interface {
	OnRoleChanged(RoleType)
}

// config holds internal configuration for the NATS JetStream cluster provider.
type config struct {
	StreamName         string
	IdentityStreamName string
	SubjectPrefix      string
	Replicas           int
	Storage            jetstream.StorageType
	MaxAge             time.Duration
	HeartbeatInterval  time.Duration
	HeartbeatTTL       time.Duration
	MemberTimeout      time.Duration
	CheckInterval      time.Duration
	LeaderTTL          time.Duration
	LockTTL            time.Duration
	MaxConcurrency     int
	RetryInterval      time.Duration
	RoleChanged        RoleChangedListener
}

// Option configures the NATS JetStream cluster provider.
type Option func(*config)

// WithStreamName sets a custom stream name for cluster membership.
func WithStreamName(name string) Option {
	return func(c *config) { c.StreamName = name }
}

// WithIdentityStreamName sets a custom stream name for identity storage.
func WithIdentityStreamName(name string) Option {
	return func(c *config) { c.IdentityStreamName = name }
}

// WithSubjectPrefix sets the subject prefix used for all messages.
func WithSubjectPrefix(prefix string) Option {
	return func(c *config) { c.SubjectPrefix = prefix }
}

// WithReplicas sets the number of JetStream replicas for streams.
func WithReplicas(n int) Option {
	return func(c *config) { c.Replicas = n }
}

// WithStorage sets the storage type for JetStream streams.
func WithStorage(s jetstream.StorageType) Option {
	return func(c *config) { c.Storage = s }
}

// WithMaxAge sets the maximum age for messages in the cluster stream.
func WithMaxAge(d time.Duration) Option {
	return func(c *config) { c.MaxAge = d }
}

// WithHeartbeatInterval sets the interval at which heartbeats are published.
func WithHeartbeatInterval(d time.Duration) Option {
	return func(c *config) { c.HeartbeatInterval = d }
}

// WithHeartbeatTTL sets the per-message TTL for heartbeat messages.
func WithHeartbeatTTL(d time.Duration) Option {
	return func(c *config) { c.HeartbeatTTL = d }
}

// WithMemberTimeout sets the local timeout threshold for crash detection.
func WithMemberTimeout(d time.Duration) Option {
	return func(c *config) { c.MemberTimeout = d }
}

// WithCheckInterval sets the interval for the stale member check goroutine.
func WithCheckInterval(d time.Duration) Option {
	return func(c *config) { c.CheckInterval = d }
}

// WithLeaderTTL sets the per-message TTL for the leader claim message.
func WithLeaderTTL(d time.Duration) Option {
	return func(c *config) { c.LeaderTTL = d }
}

// WithLockTTL sets the TTL for identity spawn locks.
func WithLockTTL(d time.Duration) Option {
	return func(c *config) { c.LockTTL = d }
}

// WithMaxConcurrency sets the maximum number of concurrent identity operations.
func WithMaxConcurrency(n int) Option {
	return func(c *config) { c.MaxConcurrency = n }
}

// WithRetryInterval sets the interval between retry attempts.
func WithRetryInterval(d time.Duration) Option {
	return func(c *config) { c.RetryInterval = d }
}

// WithRoleChangedListener sets a callback for leadership role changes.
func WithRoleChangedListener(l RoleChangedListener) Option {
	return func(c *config) { c.RoleChanged = l }
}

func newDefaultConfig() *config {
	return &config{
		Replicas:          defaultReplicas,
		Storage:           jetstream.FileStorage,
		MaxAge:            defaultMaxAge,
		HeartbeatInterval: defaultHeartbeatInterval,
		HeartbeatTTL:      defaultHeartbeatTTL,
		MemberTimeout:     defaultMemberTimeout,
		CheckInterval:     defaultCheckInterval,
		LeaderTTL:         defaultLeaderTTL,
		LockTTL:           defaultLockTTL,
		MaxConcurrency:    defaultMaxConcurrency,
		SubjectPrefix:     defaultSubjectPrefix,
		RetryInterval:     defaultRetryInterval,
	}
}

func (c *config) streamName(clusterName string) string {
	if c.StreamName != "" {
		return c.StreamName
	}
	return "PROTOACTOR_" + clusterName
}

func (c *config) identityStreamName(clusterName string) string {
	if c.IdentityStreamName != "" {
		return c.IdentityStreamName
	}
	return "PROTOACTOR_" + clusterName + "_IDENTITIES"
}

func (c *config) subjectPrefix(clusterName string) string {
	if c.SubjectPrefix != defaultSubjectPrefix {
		return c.SubjectPrefix
	}
	return "cluster." + clusterName
}
```

**Step 5: Run `go mod tidy` and run tests**

Run: `cd /home/cchamplin/development/protoactor-go/cluster/clusterproviders/natsstream && go mod tidy && go test -run Test -v -count=1`
Expected: All config tests PASS

**Step 6: Commit**

```bash
git add cluster/clusterproviders/natsstream/go.mod cluster/clusterproviders/natsstream/go.sum cluster/clusterproviders/natsstream/config.go cluster/clusterproviders/natsstream/config_test.go
git commit -m "feat(natsstream): add module setup and config with options pattern"
```

---

## Task 2: Node Type and Test Helpers

**Files:**
- Create: `cluster/clusterproviders/natsstream/node.go`
- Create: `cluster/clusterproviders/natsstream/node_test.go`
- Create: `cluster/clusterproviders/natsstream/testhelpers_test.go`

**Step 1: Write the node tests**

```go
package natsstream

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNode(t *testing.T) {
	n := NewNode("member1", "127.0.0.1", 8080, []string{"MyKind"})
	assert.Equal(t, "member1", n.ID)
	assert.Equal(t, "member1", n.Name)
	assert.Equal(t, "127.0.0.1", n.Host)
	assert.Equal(t, 8080, n.Port)
	assert.Equal(t, []string{"MyKind"}, n.Kinds)
	assert.True(t, n.Alive)
}

func TestNode_Serialize_Deserialize(t *testing.T) {
	n := NewNode("member1", "127.0.0.1", 8080, []string{"MyKind"})
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

func TestNode_MemberStatus(t *testing.T) {
	n := NewNode("member1", "127.0.0.1", 8080, []string{"MyKind"})
	m := n.MemberStatus()
	assert.Equal(t, "member1", m.Id)
	assert.Equal(t, "127.0.0.1", m.Host)
	assert.Equal(t, int32(8080), m.Port)
	assert.Equal(t, []string{"MyKind"}, m.Kinds)
}

func TestNode_MemberStatus_NilKinds(t *testing.T) {
	n := NewNode("member1", "127.0.0.1", 8080, nil)
	m := n.MemberStatus()
	assert.Equal(t, []string{}, m.Kinds)
}

func TestNode_Equal(t *testing.T) {
	n1 := NewNode("member1", "127.0.0.1", 8080, nil)
	n2 := NewNode("member1", "10.0.0.1", 9090, nil)
	n3 := NewNode("member2", "127.0.0.1", 8080, nil)

	assert.True(t, n1.Equal(n2), "same ID should be equal")
	assert.False(t, n1.Equal(n3), "different ID should not be equal")
	assert.False(t, n1.Equal(nil), "nil comparison should be false")

	var nilNode *Node
	assert.False(t, nilNode.Equal(n1), "nil receiver should be false")
}

func TestNode_AliveFlag(t *testing.T) {
	n := NewNode("member1", "127.0.0.1", 8080, nil)
	assert.True(t, n.IsAlive())

	n.SetAlive(false)
	assert.False(t, n.IsAlive())
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/protoactor-go/cluster/clusterproviders/natsstream && go test -run TestNewNode -v`
Expected: Compilation error — Node type not defined

**Step 3: Write `node.go`**

Copy directly from `cluster/clusterproviders/natskv/node.go` — the Node type is identical since both need the same fields to represent cluster members. The JetStream stream provider stores the same JSON payload.

```go
package natsstream

import (
	"encoding/json"

	"github.com/awevoke/protoactor-go/cluster"
)

// Node represents a cluster member stored in a JetStream stream message.
type Node struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Host  string   `json:"host"`
	Port  int      `json:"port"`
	Kinds []string `json:"kinds"`
	Alive bool     `json:"alive"`
}

// NewNode constructs a new Node instance with Alive set to true.
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

// Equal compares two nodes by ID. Returns false if either node is nil.
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

**Step 4: Write `testhelpers_test.go`**

Same pattern as `natskv/testhelpers_test.go` but also provides a helper to get a JetStream handle.

```go
package natsstream

import (
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/remote"
)

// startEmbeddedNATS starts an embedded NATS server with JetStream enabled.
func startEmbeddedNATS(t *testing.T) *server.Server {
	t.Helper()

	opts := &server.Options{
		JetStream: true,
		Port:      -1,
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

// connectNATS connects to the given embedded NATS server and returns a connection and JetStream handle.
func connectNATS(t *testing.T, srv *server.Server) (*nats.Conn, jetstream.JetStream) {
	t.Helper()

	nc, err := nats.Connect(srv.ClientURL())
	require.NoError(t, err, "failed to connect to NATS")
	t.Cleanup(func() { nc.Close() })

	js, err := jetstream.New(nc)
	require.NoError(t, err, "failed to create JetStream context")

	return nc, js
}

// setupCluster creates a provider, actor system, and cluster for testing.
func setupCluster(t *testing.T, srv *server.Server, clusterName string, opts ...Option) (*Provider, *cluster.Cluster) {
	t.Helper()

	nc, _ := connectNATS(t, srv)

	p, err := New(nc, opts...)
	require.NoError(t, err)

	system := actor.NewActorSystem()
	remoteConfig := remote.Configure("127.0.0.1", 0)
	clusterConfig := cluster.Configure(clusterName, p, p.IdentityLookup(), remoteConfig)
	c := cluster.NewCluster(system, clusterConfig)

	// Initialize the remote so that ActorSystem.Address() returns a proper host:port.
	c.Remote = remote.NewRemote(system, remoteConfig)

	return p, c
}
```

Note: `setupCluster` references `Provider`, `New`, and `IdentityLookup` which don't exist yet. These test helpers will only compile once the provider stub exists (Task 3). The node tests can run independently.

**Step 5: Run node tests**

Run: `cd /home/cchamplin/development/protoactor-go/cluster/clusterproviders/natsstream && go test -run TestNewNode -v && go test -run TestNode -v`
Expected: All node tests PASS (test helpers won't compile yet but aren't needed for node tests — use a build tag or just run the specific test files)

Actually, since the test helpers import Provider (which doesn't exist), we need to either: (a) skip running all tests now and wait for Task 3, or (b) temporarily comment out testhelpers_test.go. The pragmatic approach: create testhelpers_test.go **in Task 3** after the Provider stub exists. For now, just create node.go and node_test.go.

**Step 5 (revised): Run node tests only**

Run: `cd /home/cchamplin/development/protoactor-go/cluster/clusterproviders/natsstream && go test -run TestNode -v -count=1`
Expected: All node tests PASS

**Step 6: Commit**

```bash
git add cluster/clusterproviders/natsstream/node.go cluster/clusterproviders/natsstream/node_test.go
git commit -m "feat(natsstream): add Node type with serialization and tests"
```

---

## Task 3: Singleton Scheduler

**Files:**
- Create: `cluster/clusterproviders/natsstream/singleton.go`
- Create: `cluster/clusterproviders/natsstream/singleton_test.go`

**Step 1: Write the singleton tests**

```go
package natsstream

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSingleton_FromFunc(t *testing.T) {
	system := actor.NewActorSystem()
	s := NewSingletonScheduler(system.Root)
	s.FromFunc(func(ctx actor.Context) {})
	s.Lock()
	defer s.Unlock()
	assert.Len(t, s.props, 1)
}

func TestSingleton_FromProducer(t *testing.T) {
	system := actor.NewActorSystem()
	s := NewSingletonScheduler(system.Root)
	s.FromProducer(func() actor.Actor { return &dummyActor{} })
	s.Lock()
	defer s.Unlock()
	assert.Len(t, s.props, 1)
}

func TestSingleton_OnRoleChanged_Leader_Spawns(t *testing.T) {
	system := actor.NewActorSystem()
	var spawned atomic.Bool
	s := NewSingletonScheduler(system.Root)
	s.FromFunc(func(ctx actor.Context) {
		if _, ok := ctx.Message().(*actor.Started); ok {
			spawned.Store(true)
		}
	})

	s.OnRoleChanged(Leader)

	require.Eventually(t, func() bool {
		return spawned.Load()
	}, 2*time.Second, 50*time.Millisecond)

	s.Lock()
	assert.Len(t, s.pids, 1)
	assert.NotNil(t, s.pids[0])
	s.Unlock()
}

func TestSingleton_OnRoleChanged_Follower_Poisons(t *testing.T) {
	system := actor.NewActorSystem()
	s := NewSingletonScheduler(system.Root)
	s.FromFunc(func(ctx actor.Context) {})

	// Become leader first to spawn actors
	s.OnRoleChanged(Leader)
	s.Lock()
	require.Len(t, s.pids, 1)
	s.Unlock()

	// Become follower — actors should be poisoned
	s.OnRoleChanged(Follower)
	s.Lock()
	assert.Nil(t, s.pids)
	s.Unlock()
}

type dummyActor struct{}

func (d *dummyActor) Receive(ctx actor.Context) {}
```

**Step 2: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/protoactor-go/cluster/clusterproviders/natsstream && go test -run TestSingleton -v`
Expected: Compilation error — SingletonScheduler not defined

**Step 3: Write `singleton.go`**

Copy from `natskv/singleton.go` — this is identical across providers.

```go
package natsstream

import (
	"sync"

	"github.com/awevoke/protoactor-go/actor"
)

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

**Step 4: Run tests**

Run: `cd /home/cchamplin/development/protoactor-go/cluster/clusterproviders/natsstream && go test -run TestSingleton -v -count=1`
Expected: All singleton tests PASS

**Step 5: Commit**

```bash
git add cluster/clusterproviders/natsstream/singleton.go cluster/clusterproviders/natsstream/singleton_test.go
git commit -m "feat(natsstream): add SingletonScheduler with role-based spawn/poison"
```

---

## Task 4: Provider Stub, Constructor, and Test Helpers

**Files:**
- Create: `cluster/clusterproviders/natsstream/natsstream_provider.go`
- Create: `cluster/clusterproviders/natsstream/testhelpers_test.go`
- Create: `cluster/clusterproviders/natsstream/natsstream_provider_test.go` (initial tests only)

This task creates the Provider struct with constructors and a minimal stub so that test helpers compile. The full provider logic (StartMember, etc.) comes in subsequent tasks.

**Step 1: Write initial provider tests**

```go
package natsstream

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
		WithStreamName("MY_STREAM"),
		WithSubjectPrefix("myprefix"),
	)
	require.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, "MY_STREAM", p.config.StreamName)
	assert.Equal(t, "myprefix", p.config.SubjectPrefix)
}

func TestProvider_GetHealthStatus_NilByDefault(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc, _ := connectNATS(t, srv)

	p, err := New(nc)
	require.NoError(t, err)

	assert.NoError(t, p.GetHealthStatus())
}

func TestProvider_IdentityLookup_NotNil(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc, _ := connectNATS(t, srv)

	p, err := New(nc)
	require.NoError(t, err)

	assert.NotNil(t, p.IdentityLookup())
}
```

**Step 2: Write `testhelpers_test.go`**

Use the code from Task 2, Step 4 above (the `startEmbeddedNATS`, `connectNATS`, `setupCluster` helper functions).

**Step 3: Write the Provider struct and constructors in `natsstream_provider.go`**

```go
package natsstream

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

	"github.com/awevoke/protoactor-go/cluster"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Compile-time interface check.
var _ cluster.ClusterProvider = (*Provider)(nil)

// Provider uses NATS JetStream raw streams for cluster membership discovery,
// health checking via local timeout-based crash detection, and leader election
// via publish-race CAS semantics.
type Provider struct {
	cluster     *cluster.Cluster
	clusterName string
	prefix      string // resolved subject prefix
	config      *config
	js          jetstream.JetStream
	stream      jetstream.Stream // cluster membership stream
	self        *Node
	isMember    bool
	members     map[string]*Node
	membersMu   sync.RWMutex
	lastSeen    map[string]time.Time // memberID -> last heartbeat time
	lastSeenMu  sync.RWMutex
	shutdown    atomic.Bool
	clusterError error
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc

	// Leader election
	role                RoleType
	roleMu              sync.Mutex
	roleChangedChan     chan RoleType
	roleChangedListener RoleChangedListener
	schedulers          []*SingletonScheduler
	isLeader            atomic.Bool
	leaderSeq           uint64 // last successful leader publish sequence
	leaderMemberID      string // ID of the current leader (from consumed messages)
	leaderMu            sync.RWMutex

	// Integrated identity lookup
	identity *IdentityLookup
}

// New creates a Provider using a NATS connection. It creates a JetStream
// context from the connection and delegates to NewFromJetStream.
func New(conn *nats.Conn, opts ...Option) (*Provider, error) {
	js, err := jetstream.New(conn)
	if err != nil {
		return nil, fmt.Errorf("natsstream: create JetStream context: %w", err)
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
		config:              cfg,
		js:                  js,
		members:             make(map[string]*Node),
		lastSeen:            make(map[string]time.Time),
		role:                Follower,
		roleChangedChan:     make(chan RoleType, 1),
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
// Must be called before StartMember.
func (p *Provider) RegisterSingletonScheduler(scheduler *SingletonScheduler) {
	p.schedulers = append(p.schedulers, scheduler)
}

// init extracts host, port, memberID, and kinds from the cluster and builds the self node.
func (p *Provider) init(c *cluster.Cluster) error {
	p.cluster = c
	p.clusterName = c.Config.Name
	p.prefix = p.config.subjectPrefix(p.clusterName)

	addr := c.ActorSystem.Address()
	host, port, err := splitHostPort(addr)
	if err != nil {
		return err
	}

	memberID := c.ActorSystem.ID
	knownKinds := c.GetClusterKinds()
	nodeName := fmt.Sprintf("%v_%v", p.clusterName, memberID)
	p.self = NewNode(nodeName, host, port, knownKinds)
	return nil
}

// StartMember registers the node and starts all background goroutines.
func (p *Provider) StartMember(c *cluster.Cluster) error {
	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.isMember = true

	if err := p.init(c); err != nil {
		return err
	}

	if err := p.createClusterStream(); err != nil {
		return err
	}

	if err := p.publishJoin(); err != nil {
		return err
	}

	if err := p.publishHeartbeat(); err != nil {
		return err
	}

	p.startRoleChangedNotifyLoop()
	p.startStreamConsumer()
	p.startHeartbeatPublisher()
	p.startStaleMemberChecker()
	p.startLeaderElection()

	return nil
}

// StartClient initializes the provider without registering the node.
func (p *Provider) StartClient(c *cluster.Cluster) error {
	p.ctx, p.cancel = context.WithCancel(context.Background())

	if err := p.init(c); err != nil {
		return err
	}

	if err := p.createClusterStream(); err != nil {
		return err
	}

	p.startStreamConsumer()

	return nil
}

// Shutdown deregisters the node and stops background tasks.
func (p *Provider) Shutdown(_ bool) error {
	if !p.shutdown.CompareAndSwap(false, true) {
		return nil
	}

	// Publish leave event before cancelling context
	if p.isMember && p.self != nil {
		leaveCtx, leaveCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer leaveCancel()
		p.publishLeave(leaveCtx)
	}

	if p.cancel != nil {
		p.cancel()
	}

	// Wait for all goroutines to finish
	p.wg.Wait()

	return nil
}

// createClusterStream creates or binds the cluster membership stream.
func (p *Provider) createClusterStream() error {
	name := p.config.streamName(p.clusterName)

	s, err := p.js.CreateOrUpdateStream(p.ctx, jetstream.StreamConfig{
		Name:                    name,
		Subjects:                []string{p.prefix + ".>"},
		MaxMsgsPerSubject:       1,
		MaxAge:                  p.config.MaxAge,
		AllowMsgTTL:             true,
		SubjectDeleteMarkerTTL:  p.config.HeartbeatTTL,
		Retention:               jetstream.LimitsPolicy,
		Storage:                 p.config.Storage,
		Replicas:                p.config.Replicas,
	})
	if err != nil {
		return fmt.Errorf("natsstream: create cluster stream %q: %w", name, err)
	}

	p.stream = s
	return nil
}

// publishJoin publishes a join event to the stream.
func (p *Provider) publishJoin() error {
	type joinPayload struct {
		ID    string   `json:"id"`
		Host  string   `json:"host"`
		Port  int      `json:"port"`
		Kinds []string `json:"kinds"`
	}
	payload := joinPayload{
		ID:    p.self.ID,
		Host:  p.self.Host,
		Port:  p.self.Port,
		Kinds: p.self.Kinds,
	}
	data, err := marshalJSON(payload)
	if err != nil {
		return fmt.Errorf("natsstream: marshal join: %w", err)
	}

	subject := p.prefix + ".join." + p.self.ID
	_, err = p.js.Publish(p.ctx, subject, data)
	if err != nil {
		return fmt.Errorf("natsstream: publish join: %w", err)
	}
	return nil
}

// publishHeartbeat publishes a heartbeat message with per-message TTL.
func (p *Provider) publishHeartbeat() error {
	data, err := p.self.Serialize()
	if err != nil {
		return fmt.Errorf("natsstream: serialize self: %w", err)
	}

	subject := p.prefix + ".members." + p.self.ID
	_, err = p.js.Publish(p.ctx, subject, data, jetstream.WithMsgTTL(p.config.HeartbeatTTL))
	if err != nil {
		return fmt.Errorf("natsstream: publish heartbeat: %w", err)
	}
	return nil
}

// publishLeave publishes a leave event.
func (p *Provider) publishLeave(ctx context.Context) {
	type leavePayload struct {
		ID     string `json:"id"`
		Reason string `json:"reason"`
	}
	payload := leavePayload{ID: p.self.ID, Reason: "shutdown"}
	data, _ := marshalJSON(payload)

	subject := p.prefix + ".leave." + p.self.ID
	_, err := p.js.Publish(ctx, subject, data)
	if err != nil {
		p.logger().Warn("Failed to publish leave event",
			slog.String("provider", "natsstream"),
			slog.Any("error", err))
	}
}

// startHeartbeatPublisher starts the goroutine that periodically publishes heartbeats.
func (p *Provider) startHeartbeatPublisher() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				p.logger().Error("Recovered from panic in heartbeat publisher",
					slog.String("provider", "natsstream"),
					slog.Any("error", r))
			}
		}()

		ticker := time.NewTicker(p.config.HeartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if p.shutdown.Load() {
					return
				}
				if err := p.publishHeartbeat(); err != nil {
					p.logger().Warn("Failed to publish heartbeat",
						slog.String("provider", "natsstream"),
						slog.Any("error", err))
				}
			case <-p.ctx.Done():
				return
			}
		}
	}()
}

// startStreamConsumer starts the ordered consumer goroutine that processes all cluster events.
func (p *Provider) startStreamConsumer() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				p.logger().Error("Recovered from panic in stream consumer",
					slog.String("provider", "natsstream"),
					slog.Any("error", r))
				p.clusterError = fmt.Errorf("stream consumer panic: %v", r)
			}
		}()

		for !p.shutdown.Load() {
			if err := p.consumeStream(); err != nil {
				if p.shutdown.Load() {
					return
				}
				p.logger().Error("Stream consumer failed, retrying",
					slog.String("provider", "natsstream"),
					slog.Any("error", err))
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

// consumeStream creates an ordered consumer and processes messages.
func (p *Provider) consumeStream() error {
	streamName := p.config.streamName(p.clusterName)
	consumer, err := p.js.OrderedConsumer(p.ctx, streamName, jetstream.OrderedConsumerConfig{
		FilterSubjects: []string{p.prefix + ".>"},
	})
	if err != nil {
		return fmt.Errorf("natsstream: create ordered consumer: %w", err)
	}

	iter, err := consumer.Messages()
	if err != nil {
		return fmt.Errorf("natsstream: get message iterator: %w", err)
	}
	defer iter.Stop()

	// Track whether we've done the initial replay.
	// After processing all existing messages, publish initial topology.
	initialPublished := false

	for {
		msg, err := iter.Next()
		if err != nil {
			if p.ctx.Err() != nil {
				return nil // context cancelled, graceful exit
			}
			return fmt.Errorf("natsstream: next message: %w", err)
		}

		p.handleStreamMessage(msg)
		msg.Ack()

		// After processing each message, check if there are more pending.
		// If not, and we haven't published initial topology yet, do so now.
		if !initialPublished {
			info, infoErr := consumer.Info(p.ctx)
			if infoErr == nil && info.NumPending == 0 {
				initialPublished = true
				p.publishClusterTopologyEvent()
			}
		}
	}
}

// handleStreamMessage routes a stream message to the appropriate handler based on subject.
func (p *Provider) handleStreamMessage(msg jetstream.Msg) {
	subject := msg.Subject()

	switch {
	case subjectMatches(subject, p.prefix+".members."):
		p.handleHeartbeatMessage(msg)
	case subjectMatches(subject, p.prefix+".join."):
		// Join events are informational; the heartbeat carries the actual member data.
		// Log for observability.
		p.logger().Info("Member join event received",
			slog.String("provider", "natsstream"),
			slog.String("subject", subject))
	case subjectMatches(subject, p.prefix+".leave."):
		p.handleLeaveMessage(msg)
	case subject == p.prefix+".leader":
		p.handleLeaderMessage(msg)
	}
}

// handleHeartbeatMessage processes a heartbeat message from a member.
func (p *Provider) handleHeartbeatMessage(msg jetstream.Msg) {
	node, err := NewNodeFromBytes(msg.Data())
	if err != nil {
		p.logger().Error("Invalid heartbeat data",
			slog.String("provider", "natsstream"),
			slog.String("subject", msg.Subject()),
			slog.Any("error", err))
		return
	}

	// Don't track self via the consumer.
	if p.self != nil && node.Equal(p.self) {
		return
	}

	// Check if this is a stale replay message.
	meta, metaErr := msg.Metadata()
	if metaErr == nil {
		age := time.Since(meta.Timestamp)
		if age > p.config.MemberTimeout {
			// Stale message from replay — ignore.
			return
		}
	}

	p.membersMu.Lock()
	p.members[node.ID] = node
	p.membersMu.Unlock()

	// Update last seen with local time (not message timestamp) to avoid clock skew.
	p.lastSeenMu.Lock()
	p.lastSeen[node.ID] = time.Now()
	p.lastSeenMu.Unlock()

	p.publishClusterTopologyEvent()
}

// handleLeaveMessage processes a leave event from a member.
func (p *Provider) handleLeaveMessage(msg jetstream.Msg) {
	type leavePayload struct {
		ID string `json:"id"`
	}
	var payload leavePayload
	if err := unmarshalJSON(msg.Data(), &payload); err != nil {
		return
	}

	// Skip self
	if p.self != nil && payload.ID == p.self.ID {
		return
	}

	p.membersMu.Lock()
	delete(p.members, payload.ID)
	p.membersMu.Unlock()

	p.lastSeenMu.Lock()
	delete(p.lastSeen, payload.ID)
	p.lastSeenMu.Unlock()

	p.logger().Info("Member left",
		slog.String("provider", "natsstream"),
		slog.String("memberID", payload.ID))

	p.publishClusterTopologyEvent()
}

// handleLeaderMessage processes a leader claim message.
func (p *Provider) handleLeaderMessage(msg jetstream.Msg) {
	type leaderPayload struct {
		MemberID string `json:"memberID"`
	}
	var payload leaderPayload
	if err := unmarshalJSON(msg.Data(), &payload); err != nil {
		return
	}

	p.leaderMu.Lock()
	p.leaderMemberID = payload.MemberID
	p.leaderMu.Unlock()
}

// startStaleMemberChecker starts the goroutine that periodically checks for
// members whose heartbeats have timed out.
func (p *Provider) startStaleMemberChecker() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				p.logger().Error("Recovered from panic in stale member checker",
					slog.String("provider", "natsstream"),
					slog.Any("error", r))
			}
		}()

		ticker := time.NewTicker(p.config.CheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if p.shutdown.Load() {
					return
				}
				p.checkStaleMembersOnce()
			case <-p.ctx.Done():
				return
			}
		}
	}()
}

// checkStaleMembersOnce scans the lastSeen map and removes members that have
// exceeded the member timeout.
func (p *Provider) checkStaleMembersOnce() {
	now := time.Now()
	var stale []string

	p.lastSeenMu.RLock()
	for id, last := range p.lastSeen {
		if now.Sub(last) > p.config.MemberTimeout {
			stale = append(stale, id)
		}
	}
	p.lastSeenMu.RUnlock()

	if len(stale) == 0 {
		return
	}

	p.membersMu.Lock()
	for _, id := range stale {
		delete(p.members, id)
	}
	p.membersMu.Unlock()

	p.lastSeenMu.Lock()
	for _, id := range stale {
		delete(p.lastSeen, id)
	}
	p.lastSeenMu.Unlock()

	for _, id := range stale {
		p.logger().Warn("Stale member removed",
			slog.String("provider", "natsstream"),
			slog.String("memberID", id))
	}

	p.publishClusterTopologyEvent()
}

// startLeaderElection starts the goroutine that periodically attempts leader election.
func (p *Provider) startLeaderElection() {
	// Attempt immediately
	p.attemptLeaderElection()

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				p.logger().Error("Recovered from panic in leader election",
					slog.String("provider", "natsstream"),
					slog.Any("error", r))
			}
		}()

		// Attempt interval: slightly shorter than leader TTL
		interval := p.config.LeaderTTL * 7 / 10
		if interval < 1*time.Second {
			interval = 1 * time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if p.shutdown.Load() {
					return
				}
				p.attemptLeaderElection()
			case <-p.ctx.Done():
				return
			}
		}
	}()
}

// attemptLeaderElection tries to claim or refresh leadership via CAS publish.
func (p *Provider) attemptLeaderElection() {
	subject := p.prefix + ".leader"

	type leaderPayload struct {
		MemberID  string `json:"memberID"`
		ElectedAt string `json:"electedAt"`
	}
	payload := leaderPayload{
		MemberID:  p.self.ID,
		ElectedAt: time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := marshalJSON(payload)

	if p.isLeader.Load() {
		// Leader refresh: use the last known sequence
		seq := atomic.LoadUint64(&p.leaderSeq)
		ack, err := p.js.Publish(p.ctx, subject, data,
			jetstream.WithExpectLastSubjectSequence(seq),
			jetstream.WithMsgTTL(p.config.LeaderTTL))
		if err != nil {
			// Lost leadership
			p.isLeader.Store(false)
			atomic.StoreUint64(&p.leaderSeq, 0)
			p.setRole(Follower)
			return
		}
		atomic.StoreUint64(&p.leaderSeq, ack.Sequence)
	} else {
		// Non-leader attempt: try with seq=0 (expects no prior message)
		ack, err := p.js.Publish(p.ctx, subject, data,
			jetstream.WithExpectLastSubjectSequence(0),
			jetstream.WithMsgTTL(p.config.LeaderTTL))
		if err != nil {
			// Another member is leader — this is expected
			return
		}
		// Won leadership
		p.isLeader.Store(true)
		atomic.StoreUint64(&p.leaderSeq, ack.Sequence)
		p.setRole(Leader)
	}
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

	// Include self if registered as a member (not client).
	if p.self != nil && p.isMember {
		members = append(members, p.self.MemberStatus())
	}

	if p.cluster != nil {
		p.cluster.Logger().Info("Update cluster topology",
			slog.String("provider", "natsstream"),
			slog.Int("members", len(members)))
		p.cluster.MemberList.UpdateClusterTopology(members)
	}
}

// setRole updates the role and notifies listeners.
func (p *Provider) setRole(role RoleType) {
	p.roleMu.Lock()
	if role == p.role {
		p.roleMu.Unlock()
		return
	}

	p.logger().Info("Role changed",
		slog.String("provider", "natsstream"),
		slog.String("from", p.role.String()),
		slog.String("to", role.String()))

	p.role = role
	p.roleMu.Unlock()

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

// startRoleChangedNotifyLoop starts the goroutine that notifies the
// external role changed listener.
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

// logger returns the cluster logger if available, otherwise the default logger.
func (p *Provider) logger() *slog.Logger {
	if p.cluster != nil {
		return p.cluster.Logger()
	}
	return slog.Default()
}

// safeRun executes fn and recovers from panics, logging the stack trace.
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

// subjectMatches checks if a subject starts with the given prefix.
func subjectMatches(subject, prefix string) bool {
	return len(subject) > len(prefix) && subject[:len(prefix)] == prefix
}

// marshalJSON is a convenience wrapper around json.Marshal.
func marshalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

// unmarshalJSON is a convenience wrapper around json.Unmarshal.
func unmarshalJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
```

Note: Add `"encoding/json"` to the imports.

**Step 4: Create a minimal IdentityLookup stub** so the provider compiles

Create `cluster/clusterproviders/natsstream/natsstream_identity.go` with just enough to compile:

```go
package natsstream

import (
	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
)

// Compile-time check that IdentityLookup implements cluster.IdentityLookup.
var _ cluster.IdentityLookup = (*IdentityLookup)(nil)

// IdentityLookup implements cluster.IdentityLookup directly using NATS JetStream
// streams for lock acquisition, activation storage, and member tracking.
type IdentityLookup struct {
	provider  *Provider
	cluster   *cluster.Cluster
	memberID  string
	isClient  bool
	config    *config
	semaphore chan struct{}
}

// newIdentityLookup creates a new IdentityLookup associated with the given provider.
func newIdentityLookup(p *Provider) *IdentityLookup {
	return &IdentityLookup{
		provider:  p,
		config:    p.config,
		semaphore: make(chan struct{}, p.config.MaxConcurrency),
	}
}

// Get resolves a cluster identity to an actor PID.
func (il *IdentityLookup) Get(ci *cluster.ClusterIdentity) *actor.PID {
	// TODO: implement in Task 6
	return nil
}

// RemovePid removes the activation for a cluster identity.
func (il *IdentityLookup) RemovePid(ci *cluster.ClusterIdentity, pid *actor.PID) {
	// TODO: implement in Task 6
}

// Setup initializes the identity lookup with the cluster context.
func (il *IdentityLookup) Setup(c *cluster.Cluster, kinds []string, isClient bool) {
	il.cluster = c
	il.memberID = c.ActorSystem.ID
	il.isClient = isClient
	// TODO: implement fully in Task 6
}

// Shutdown performs cleanup when the cluster is shutting down.
func (il *IdentityLookup) Shutdown() {
	// TODO: implement in Task 6
}
```

**Step 5: Run `go mod tidy` and run tests**

Run: `cd /home/cchamplin/development/protoactor-go/cluster/clusterproviders/natsstream && go mod tidy && go test -run "TestNew|TestProvider" -v -count=1`
Expected: All constructor/health tests PASS

**Step 6: Commit**

```bash
git add cluster/clusterproviders/natsstream/natsstream_provider.go cluster/clusterproviders/natsstream/natsstream_identity.go cluster/clusterproviders/natsstream/testhelpers_test.go cluster/clusterproviders/natsstream/natsstream_provider_test.go cluster/clusterproviders/natsstream/go.mod cluster/clusterproviders/natsstream/go.sum
git commit -m "feat(natsstream): add Provider struct with constructors, stream consumer, heartbeat, leader election, crash detection"
```

---

## Task 5: Provider Unit Tests

**Files:**
- Modify: `cluster/clusterproviders/natsstream/natsstream_provider_test.go`

This task adds tests for the full provider lifecycle: StartMember, StartClient, Shutdown, member discovery, crash detection, leader election, and singleton scheduling.

**Step 1: Add provider lifecycle tests**

Append to `natsstream_provider_test.go`:

```go
// mockRoleListener records role changes via a callback.
type mockRoleListener struct {
	callback func(RoleType)
}

func (m *mockRoleListener) OnRoleChanged(r RoleType) { m.callback(r) }

func TestStartMember_PublishesHeartbeat(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-heartbeat")

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	// Verify the heartbeat message exists on the stream.
	require.NotNil(t, p.self)
	subject := p.prefix + ".members." + p.self.ID

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msg, err := p.stream.GetLastMsgForSubject(ctx, subject)
	require.NoError(t, err, "heartbeat message should exist on stream")

	var node Node
	require.NoError(t, json.Unmarshal(msg.Data, &node))
	assert.Equal(t, p.self.ID, node.ID)
	assert.True(t, node.Alive)
}

func TestStartMember_DiscoversExistingMembers(t *testing.T) {
	srv := startEmbeddedNATS(t)

	// Start first member.
	p1, c1 := setupCluster(t, srv, "test-discover")
	err := p1.StartMember(c1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p1.Shutdown(true) })

	time.Sleep(500 * time.Millisecond)

	// Start second member.
	p2, c2 := setupCluster(t, srv, "test-discover")
	err = p2.StartMember(c2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	// Second should discover first.
	require.Eventually(t, func() bool {
		p2.membersMu.RLock()
		defer p2.membersMu.RUnlock()
		_, found := p2.members[p1.self.ID]
		return found
	}, 10*time.Second, 200*time.Millisecond, "second provider should discover the first member")
}

func TestStartClient_WatchOnly(t *testing.T) {
	srv := startEmbeddedNATS(t)

	// Start a member first.
	pMember, cMember := setupCluster(t, srv, "test-client")
	err := pMember.StartMember(cMember)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pMember.Shutdown(true) })

	time.Sleep(500 * time.Millisecond)

	// Start a client.
	pClient, cClient := setupCluster(t, srv, "test-client")
	err = pClient.StartClient(cClient)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pClient.Shutdown(true) })

	// Client should NOT have published a heartbeat.
	subject := pClient.prefix + ".members." + pClient.self.ID
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = pClient.stream.GetLastMsgForSubject(ctx, subject)
	assert.Error(t, err, "client should NOT publish heartbeat")

	// Client should discover the member.
	require.Eventually(t, func() bool {
		pClient.membersMu.RLock()
		defer pClient.membersMu.RUnlock()
		_, found := pClient.members[pMember.self.ID]
		return found
	}, 10*time.Second, 200*time.Millisecond, "client should discover the member")
}

func TestShutdown_PublishesLeave(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-leave")

	err := p.StartMember(c)
	require.NoError(t, err)

	require.NotNil(t, p.self)

	// Shutdown gracefully.
	err = p.Shutdown(true)
	require.NoError(t, err)

	// Verify leave message was published.
	subject := p.prefix + ".leave." + p.self.ID
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	msg, err := p.stream.GetLastMsgForSubject(ctx, subject)
	require.NoError(t, err, "leave event should be published on shutdown")
	assert.Contains(t, string(msg.Data), p.self.ID)
}

func TestShutdown_Idempotent(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-shutdown-idem")

	err := p.StartMember(c)
	require.NoError(t, err)

	err = p.Shutdown(true)
	assert.NoError(t, err)

	err = p.Shutdown(true)
	assert.NoError(t, err, "second shutdown should not fail or panic")
}

func TestMemberCrash_TimeoutDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timeout detection test in short mode")
	}

	srv := startEmbeddedNATS(t)

	// Use short timeouts for fast tests.
	opts := []Option{
		WithHeartbeatInterval(100 * time.Millisecond),
		WithHeartbeatTTL(300 * time.Millisecond),
		WithMemberTimeout(500 * time.Millisecond),
		WithCheckInterval(100 * time.Millisecond),
	}

	p1, c1 := setupCluster(t, srv, "test-crash", opts...)
	err := p1.StartMember(c1)
	require.NoError(t, err)

	time.Sleep(300 * time.Millisecond)

	p2, c2 := setupCluster(t, srv, "test-crash", opts...)
	err = p2.StartMember(c2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	// Verify p2 sees p1.
	require.Eventually(t, func() bool {
		p2.membersMu.RLock()
		defer p2.membersMu.RUnlock()
		_, found := p2.members[p1.self.ID]
		return found
	}, 5*time.Second, 100*time.Millisecond, "p2 should see p1")

	// "Crash" p1: cancel context and set shutdown to stop heartbeats.
	p1.shutdown.Store(true)
	if p1.cancel != nil {
		p1.cancel()
	}
	p1.wg.Wait()

	// After timeout, p2 should remove p1.
	require.Eventually(t, func() bool {
		p2.membersMu.RLock()
		defer p2.membersMu.RUnlock()
		_, found := p2.members[p1.self.ID]
		return !found
	}, 5*time.Second, 100*time.Millisecond, "p2 should detect p1 crash via timeout")
}

func TestLeaderElection_FirstWins(t *testing.T) {
	srv := startEmbeddedNATS(t)

	p1, c1 := setupCluster(t, srv, "test-leader")
	err := p1.StartMember(c1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p1.Shutdown(true) })

	time.Sleep(500 * time.Millisecond)

	p2, c2 := setupCluster(t, srv, "test-leader")
	err = p2.StartMember(c2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	time.Sleep(500 * time.Millisecond)

	p1IsLeader := p1.isLeader.Load()
	p2IsLeader := p2.isLeader.Load()

	assert.True(t, p1IsLeader || p2IsLeader, "at least one should be leader")
	assert.False(t, p1IsLeader && p2IsLeader, "only one should be leader at a time")
	assert.True(t, p1IsLeader, "first member should win leader election")
}

func TestLeaderElection_Failover(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping leader failover test in short mode")
	}

	srv := startEmbeddedNATS(t)

	leaderTTL := 3 * time.Second
	opts := []Option{
		WithLeaderTTL(leaderTTL),
		WithHeartbeatInterval(100 * time.Millisecond),
		WithHeartbeatTTL(500 * time.Millisecond),
		WithMemberTimeout(1 * time.Second),
		WithCheckInterval(200 * time.Millisecond),
	}

	p1, c1 := setupCluster(t, srv, "test-failover", opts...)
	err := p1.StartMember(c1)
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	p2, c2 := setupCluster(t, srv, "test-failover", opts...)
	err = p2.StartMember(c2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	// Wait for p1 to become leader.
	require.Eventually(t, func() bool {
		return p1.isLeader.Load()
	}, 5*time.Second, 100*time.Millisecond, "p1 should be leader")

	// Crash p1.
	p1.shutdown.Store(true)
	if p1.cancel != nil {
		p1.cancel()
	}
	p1.wg.Wait()

	// Wait for p2 to become leader after TTL expires.
	require.Eventually(t, func() bool {
		return p2.isLeader.Load()
	}, leaderTTL+5*time.Second, 200*time.Millisecond, "p2 should become leader after failover")
}

func TestRoleChangedListener_Called(t *testing.T) {
	srv := startEmbeddedNATS(t)

	var gotRole atomic.Int32
	gotRole.Store(-1)

	listener := &mockRoleListener{
		callback: func(r RoleType) {
			gotRole.Store(int32(r))
		},
	}

	p, c := setupCluster(t, srv, "test-rolechange", WithRoleChangedListener(listener))
	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	require.Eventually(t, func() bool {
		return gotRole.Load() == int32(Leader)
	}, 5*time.Second, 100*time.Millisecond, "role changed listener should be called with Leader")
}

func TestSingletonScheduler_SpawnOnLeader(t *testing.T) {
	srv := startEmbeddedNATS(t)

	var spawned atomic.Bool

	p, c := setupCluster(t, srv, "test-singleton")

	scheduler := NewSingletonScheduler(c.ActorSystem.Root)
	scheduler.FromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *actor.Started:
			spawned.Store(true)
		}
	})
	p.RegisterSingletonScheduler(scheduler)

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	require.Eventually(t, func() bool {
		return spawned.Load()
	}, 5*time.Second, 100*time.Millisecond, "singleton actor should be spawned on leader")

	scheduler.Lock()
	assert.Len(t, scheduler.pids, 1)
	assert.NotNil(t, scheduler.pids[0])
	scheduler.Unlock()
}
```

Note: Add required imports to the test file: `"context"`, `"encoding/json"`, `"sync/atomic"`, `"time"`, `"github.com/awevoke/protoactor-go/actor"`, etc.

**Step 2: Run tests**

Run: `cd /home/cchamplin/development/protoactor-go/cluster/clusterproviders/natsstream && go test -v -count=1 -timeout 120s`
Expected: All tests PASS. Some tests take a few seconds for timeout-based detection.

**Step 3: Fix any failing tests**

Debug and fix issues. Common problems:
- Stream consumer not processing messages fast enough: adjust timing
- Leader election race: ensure `attemptLeaderElection` is called before the loop starts
- Stale member checker timing: ensure test timeouts are generous enough

**Step 4: Commit**

```bash
git add cluster/clusterproviders/natsstream/natsstream_provider_test.go
git commit -m "test(natsstream): add provider unit tests for lifecycle, discovery, crash detection, leader election"
```

---

## Task 6: Identity Lookup Implementation

**Files:**
- Modify: `cluster/clusterproviders/natsstream/natsstream_identity.go`
- Create: `cluster/clusterproviders/natsstream/natsstream_identity_test.go`

**Step 1: Write identity lookup tests**

```go
package natsstream

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/cluster"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdentityLookup_InterfaceCompliance(t *testing.T) {
	// Compile-time check is in natsstream_identity.go
	var _ cluster.IdentityLookup = (*IdentityLookup)(nil)
}

func TestIdentityLookup_KvKey(t *testing.T) {
	ci := &cluster.ClusterIdentity{Kind: "MyKind", Identity: "abc-123"}
	key := kvKey(ci)
	assert.Equal(t, "MyKind.abc-123", key)
	assert.False(t, strings.Contains(key, "/"), "key should not contain slashes")
}

func TestIdentityLookup_TryAcquireLock(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-identity-lock")

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	il := p.IdentityLookup()
	il.Setup(c, []string{"MyKind"}, false)

	// Wait for identity stream to be created
	time.Sleep(500 * time.Millisecond)

	ci := &cluster.ClusterIdentity{Kind: "MyKind", Identity: "abc-123"}
	ctx := context.Background()

	lockID, seq, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok, "should acquire lock successfully")
	assert.NotEmpty(t, lockID)
	assert.Greater(t, seq, uint64(0))

	// Second attempt should fail (lock already held).
	_, _, ok2 := il.tryAcquireLock(ctx, ci)
	assert.False(t, ok2, "second lock attempt should fail")
}

func TestIdentityLookup_StoreActivation(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-identity-store")

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	il := p.IdentityLookup()
	il.Setup(c, []string{"MyKind"}, false)

	time.Sleep(500 * time.Millisecond)

	ci := &cluster.ClusterIdentity{Kind: "MyKind", Identity: "def-456"}
	ctx := context.Background()

	lockID, seq, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok)

	storeErr := il.storeActivation(ctx, ci, lockID, seq, "member1", "127.0.0.1:8080", "MyKind/def-456")
	require.NoError(t, storeErr)

	// Verify we can read it back.
	rec := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec, "should find stored activation")
	assert.Equal(t, "MyKind/def-456", rec.PidID)
	assert.Equal(t, "127.0.0.1:8080", rec.PidAddress)
	assert.Equal(t, "member1", rec.MemberID)
}

func TestIdentityLookup_GetExisting_NotFound(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-identity-notfound")

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	il := p.IdentityLookup()
	il.Setup(c, []string{"MyKind"}, false)

	time.Sleep(500 * time.Millisecond)

	ci := &cluster.ClusterIdentity{Kind: "MyKind", Identity: "nonexistent"}
	ctx := context.Background()

	rec := il.getExistingActivation(ctx, ci)
	assert.Nil(t, rec, "non-existent identity should return nil")
}

func TestIdentityLookup_RemoveMember_PurgesActivations(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-identity-remove")

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	il := p.IdentityLookup()
	il.Setup(c, []string{"MyKind"}, false)

	time.Sleep(500 * time.Millisecond)

	ctx := context.Background()
	ci := &cluster.ClusterIdentity{Kind: "MyKind", Identity: "to-remove"}

	// Acquire lock and store activation.
	lockID, seq, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok)
	require.NoError(t, il.storeActivation(ctx, ci, lockID, seq, "member-x", "127.0.0.1:8080", "MyKind/to-remove"))

	// Verify it exists.
	rec := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec)

	// Remove all activations for member-x.
	il.removeMemberID(ctx, "member-x")

	// Verify it's gone.
	rec = il.getExistingActivation(ctx, ci)
	assert.Nil(t, rec, "activation should be purged after removeMemberID")
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/protoactor-go/cluster/clusterproviders/natsstream && go test -run TestIdentityLookup -v`
Expected: Tests fail because identity methods are stubs

**Step 3: Implement the full identity lookup**

Replace the stub in `natsstream_identity.go` with the full implementation:

```go
package natsstream

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
)

// Compile-time check that IdentityLookup implements cluster.IdentityLookup.
var _ cluster.IdentityLookup = (*IdentityLookup)(nil)

// IdentityLookup implements cluster.IdentityLookup directly using NATS JetStream
// streams for lock acquisition, activation storage, and member tracking.
type IdentityLookup struct {
	provider        *Provider
	cluster         *cluster.Cluster
	memberID        string
	isClient        bool
	identityStream  jetstream.Stream
	config          *config
	semaphore       chan struct{}
	memberKeys      map[string][]string // memberID -> list of identity subject keys
	memberKeysMu    sync.Mutex
}

// activationRecord is the JSON-encoded value stored in the identity stream.
type activationRecord struct {
	LockID     string `json:"lid,omitempty"`
	PidID      string `json:"pid,omitempty"`
	PidAddress string `json:"adr,omitempty"`
	MemberID   string `json:"mid,omitempty"`
}

// newIdentityLookup creates a new IdentityLookup associated with the given provider.
func newIdentityLookup(p *Provider) *IdentityLookup {
	return &IdentityLookup{
		provider:   p,
		config:     p.config,
		semaphore:  make(chan struct{}, p.config.MaxConcurrency),
		memberKeys: make(map[string][]string),
	}
}

// Setup initializes the identity lookup with the cluster context, creates the
// identity stream, and subscribes to topology events for member cleanup.
func (il *IdentityLookup) Setup(c *cluster.Cluster, kinds []string, isClient bool) {
	il.cluster = c
	il.memberID = c.ActorSystem.ID
	il.isClient = isClient

	ctx := context.Background()
	js := il.provider.js
	clusterName := c.Config.Name
	prefix := il.config.subjectPrefix(clusterName)

	streamName := il.config.identityStreamName(clusterName)
	s, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:              streamName,
		Subjects:          []string{prefix + ".identities.>"},
		MaxMsgsPerSubject: 1,
		Retention:         jetstream.LimitsPolicy,
		Storage:           il.config.Storage,
		Replicas:          il.config.Replicas,
	})
	if err != nil {
		slog.Error("natsstream identity: failed to create identity stream",
			slog.Any("error", err))
		return
	}
	il.identityStream = s

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
func (il *IdentityLookup) Get(ci *cluster.ClusterIdentity) *actor.PID {
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
	lockID, seq, ok := il.tryAcquireLock(ctx, ci)
	if !ok {
		// Another node is spawning. Wait for it.
		rec = il.waitForActivation(ctx, ci)
		if rec != nil {
			return pidFromRecord(rec)
		}
		return nil
	}

	// Step 4: Lock acquired — spawn and store activation.
	pid := il.spawnActivation(ci, lockID, seq)
	if pid == nil {
		// Spawn failed; purge the lock subject so another node can try.
		prefix := il.config.subjectPrefix(il.cluster.Config.Name)
		subject := prefix + ".identities." + kvKey(ci)
		_ = il.identityStream.PurgeSubject(ctx, subject)
		return nil
	}

	return pid
}

// RemovePid removes the activation for a cluster identity.
func (il *IdentityLookup) RemovePid(ci *cluster.ClusterIdentity, pid *actor.PID) {
	ctx := context.Background()
	prefix := il.config.subjectPrefix(il.cluster.Config.Name)
	subject := prefix + ".identities." + kvKey(ci)

	// Read the record to find the memberID for tracking cleanup.
	rec := il.getExistingActivation(ctx, ci)
	if rec != nil && rec.MemberID != "" {
		il.removeKeyFromMember(rec.MemberID, subject)
	}

	if err := il.identityStream.PurgeSubject(ctx, subject); err != nil {
		slog.Error("natsstream identity: RemovePid purge failed",
			slog.String("subject", subject), slog.Any("error", err))
	}
}

// Shutdown performs cleanup when the cluster is shutting down.
func (il *IdentityLookup) Shutdown() {
	if il.memberID != "" {
		il.removeMemberID(context.Background(), il.memberID)
	}
}

// kvKey converts a ClusterIdentity to a subject-safe key.
// ClusterIdentity.AsKey() returns "kind/identity" but NATS subjects use '.' as delimiter
// and '/' is not valid in subject tokens.
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
// Returns nil if no activation exists or if the message only contains a lock.
func (il *IdentityLookup) getExistingActivation(ctx context.Context, ci *cluster.ClusterIdentity) *activationRecord {
	prefix := il.config.subjectPrefix(il.cluster.Config.Name)
	subject := prefix + ".identities." + kvKey(ci)

	msg, err := il.identityStream.GetLastMsgForSubject(ctx, subject)
	if err != nil {
		return nil
	}

	var rec activationRecord
	if err := json.Unmarshal(msg.Data, &rec); err != nil {
		return nil
	}

	// Only return if it's a completed activation (has PID info).
	if rec.PidID == "" || rec.PidAddress == "" {
		return nil
	}

	return &rec
}

// tryAcquireLock attempts to acquire an exclusive spawn lock for the given
// cluster identity using CAS publish with expected seq 0 (no prior message).
func (il *IdentityLookup) tryAcquireLock(ctx context.Context, ci *cluster.ClusterIdentity) (lockID string, seq uint64, ok bool) {
	lockID = uuid.New().String()
	prefix := il.config.subjectPrefix(il.cluster.Config.Name)
	subject := prefix + ".identities." + kvKey(ci)

	rec := activationRecord{
		LockID: lockID,
	}
	data, err := json.Marshal(&rec)
	if err != nil {
		slog.Error("natsstream identity: tryAcquireLock marshal failed", slog.Any("error", err))
		return "", 0, false
	}

	ack, err := il.provider.js.Publish(ctx, subject, data,
		jetstream.WithExpectLastSubjectSequence(0))
	if err != nil {
		// Expected when another node already has the lock or activation.
		return "", 0, false
	}

	return lockID, ack.Sequence, true
}

// storeActivation stores a completed activation using CAS publish with the
// expected sequence from the lock creation.
func (il *IdentityLookup) storeActivation(ctx context.Context, ci *cluster.ClusterIdentity, lockID string, lockSeq uint64, memberID, pidAddress, pidID string) error {
	prefix := il.config.subjectPrefix(il.cluster.Config.Name)
	subject := prefix + ".identities." + kvKey(ci)

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

	_, err = il.provider.js.Publish(ctx, subject, data,
		jetstream.WithExpectLastSubjectSequence(lockSeq))
	if err != nil {
		return err
	}

	// Track this identity subject under the member.
	il.addKeyToMember(memberID, subject)
	return nil
}

// waitForActivation creates a temporary ordered consumer filtered to the specific
// identity subject and waits for a message with a completed activation record.
func (il *IdentityLookup) waitForActivation(ctx context.Context, ci *cluster.ClusterIdentity) *activationRecord {
	prefix := il.config.subjectPrefix(il.cluster.Config.Name)
	subject := prefix + ".identities." + kvKey(ci)
	streamName := il.config.identityStreamName(il.cluster.Config.Name)

	waitCtx, cancel := context.WithTimeout(ctx, il.config.LockTTL)
	defer cancel()

	consumer, err := il.provider.js.OrderedConsumer(waitCtx, streamName, jetstream.OrderedConsumerConfig{
		FilterSubjects: []string{subject},
	})
	if err != nil {
		slog.Error("natsstream identity: waitForActivation consumer failed",
			slog.String("subject", subject), slog.Any("error", err))
		return nil
	}

	iter, err := consumer.Messages()
	if err != nil {
		slog.Error("natsstream identity: waitForActivation messages failed",
			slog.String("subject", subject), slog.Any("error", err))
		return nil
	}
	defer iter.Stop()

	for {
		msg, err := iter.Next()
		if err != nil {
			return nil
		}

		var rec activationRecord
		if err := json.Unmarshal(msg.Data(), &rec); err != nil {
			msg.Ack()
			continue
		}

		msg.Ack()

		// Activation is complete when PID is set.
		if rec.PidID != "" && rec.PidAddress != "" {
			return &rec
		}
	}
}

// removeMemberID removes all activations belonging to the given member
// by purging their subjects from the identity stream.
func (il *IdentityLookup) removeMemberID(ctx context.Context, memberID string) {
	if il.identityStream == nil {
		return
	}

	il.memberKeysMu.Lock()
	keys := il.memberKeys[memberID]
	delete(il.memberKeys, memberID)
	il.memberKeysMu.Unlock()

	for _, subject := range keys {
		if err := il.identityStream.PurgeSubject(ctx, subject); err != nil {
			slog.Error("natsstream identity: removeMemberID purge failed",
				slog.String("subject", subject), slog.Any("error", err))
		}
	}
}

// addKeyToMember tracks an identity subject under a member.
func (il *IdentityLookup) addKeyToMember(memberID, subject string) {
	il.memberKeysMu.Lock()
	defer il.memberKeysMu.Unlock()

	keys := il.memberKeys[memberID]
	for _, k := range keys {
		if k == subject {
			return // already tracked
		}
	}
	il.memberKeys[memberID] = append(keys, subject)
}

// removeKeyFromMember removes an identity subject from a member's tracking.
func (il *IdentityLookup) removeKeyFromMember(memberID, subject string) {
	il.memberKeysMu.Lock()
	defer il.memberKeysMu.Unlock()

	keys := il.memberKeys[memberID]
	filtered := make([]string, 0, len(keys))
	for _, k := range keys {
		if k != subject {
			filtered = append(filtered, k)
		}
	}
	il.memberKeys[memberID] = filtered
}

// spawnActivation attempts to spawn an actor for the given cluster identity,
// stores the activation, and populates the PID cache.
func (il *IdentityLookup) spawnActivation(ci *cluster.ClusterIdentity, lockID string, lockSeq uint64) *actor.PID {
	kind, ok := il.cluster.TryGetClusterKind(ci.Kind)
	if !ok {
		slog.Error("natsstream identity: unknown kind",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity))
		return nil
	}

	props := cluster.WithClusterIdentity(kind.Props, ci)
	pid, err := il.cluster.ActorSystem.Root.SpawnNamed(props, ci.Kind+"/"+ci.Identity)
	if err != nil {
		slog.Error("natsstream identity: failed to spawn actor",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity),
			slog.Any("error", err))
		return nil
	}

	// Store the activation (CAS update with sequence from lock creation).
	ctx := context.Background()
	storeErr := il.storeActivation(ctx, ci, lockID, lockSeq, il.memberID, pid.Address, pid.Id)
	if storeErr != nil {
		slog.Error("natsstream identity: failed to store activation",
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

// pidFromRecord converts an activationRecord into an actor.PID.
func pidFromRecord(rec *activationRecord) *actor.PID {
	return actor.NewPID(rec.PidAddress, rec.PidID)
}
```

**Step 4: Run identity tests**

Run: `cd /home/cchamplin/development/protoactor-go/cluster/clusterproviders/natsstream && go test -run TestIdentityLookup -v -count=1 -timeout 60s`
Expected: All identity tests PASS

**Step 5: Run all tests to make sure nothing is broken**

Run: `cd /home/cchamplin/development/protoactor-go/cluster/clusterproviders/natsstream && go test -v -count=1 -timeout 120s`
Expected: All tests PASS

**Step 6: Commit**

```bash
git add cluster/clusterproviders/natsstream/natsstream_identity.go cluster/clusterproviders/natsstream/natsstream_identity_test.go
git commit -m "feat(natsstream): add integrated IdentityLookup with stream-based CAS locking"
```

---

## Task 7: Integration Tests

**Files:**
- Create: `cluster/clusterproviders/natsstream/natsstream_integration_test.go`

**Step 1: Write integration tests with testcontainers**

```go
//go:build integration

package natsstream

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/remote"
)

// startNATSContainer starts a NATS server in a Docker container with JetStream enabled.
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

// setupClusterFromURL creates a provider and cluster using an external NATS URL.
func setupClusterFromURL(t *testing.T, natsURL, clusterName string, opts ...Option) (*Provider, *cluster.Cluster) {
	t.Helper()
	nc, err := nats.Connect(natsURL)
	require.NoError(t, err)
	t.Cleanup(func() { nc.Close() })

	p, err := New(nc, opts...)
	require.NoError(t, err)

	system := actor.NewActorSystem()
	remoteCfg := remote.Configure("127.0.0.1", 0)
	clusterCfg := cluster.Configure(clusterName, p, p.IdentityLookup(), remoteCfg)
	c := cluster.NewCluster(system, clusterCfg)
	c.Remote = remote.NewRemote(system, remoteCfg)

	return p, c
}

func TestIntegration_TwoMemberCluster(t *testing.T) {
	natsURL := startNATSContainer(t)

	p1, c1 := setupClusterFromURL(t, natsURL, "integ-two-member")
	err := p1.StartMember(c1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p1.Shutdown(true) })

	time.Sleep(500 * time.Millisecond)

	p2, c2 := setupClusterFromURL(t, natsURL, "integ-two-member")
	err = p2.StartMember(c2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	require.Eventually(t, func() bool {
		p1.membersMu.RLock()
		_, found := p1.members[p2.self.ID]
		p1.membersMu.RUnlock()
		return found
	}, 10*time.Second, 200*time.Millisecond, "member 1 should discover member 2")

	require.Eventually(t, func() bool {
		p2.membersMu.RLock()
		_, found := p2.members[p1.self.ID]
		p2.membersMu.RUnlock()
		return found
	}, 10*time.Second, 200*time.Millisecond, "member 2 should discover member 1")

	assert.NotEmpty(t, p1.self.ID)
	assert.NotEmpty(t, p2.self.ID)
	assert.NotEqual(t, p1.self.ID, p2.self.ID)
}

func TestIntegration_MemberJoinLeave(t *testing.T) {
	natsURL := startNATSContainer(t)

	opts := []Option{
		WithHeartbeatInterval(500 * time.Millisecond),
		WithHeartbeatTTL(2 * time.Second),
		WithMemberTimeout(3 * time.Second),
		WithCheckInterval(500 * time.Millisecond),
	}

	p1, c1 := setupClusterFromURL(t, natsURL, "integ-join-leave", opts...)
	err := p1.StartMember(c1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p1.Shutdown(true) })

	time.Sleep(500 * time.Millisecond)

	p2, c2 := setupClusterFromURL(t, natsURL, "integ-join-leave", opts...)
	err = p2.StartMember(c2)
	require.NoError(t, err)

	p2ID := p2.self.ID
	require.Eventually(t, func() bool {
		p1.membersMu.RLock()
		_, found := p1.members[p2ID]
		p1.membersMu.RUnlock()
		return found
	}, 10*time.Second, 200*time.Millisecond, "member 1 should discover member 2")

	err = p2.Shutdown(true)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		p1.membersMu.RLock()
		_, found := p1.members[p2ID]
		p1.membersMu.RUnlock()
		return !found
	}, 10*time.Second, 200*time.Millisecond, "member 1 should detect member 2 departure")
}

func TestIntegration_MemberCrash(t *testing.T) {
	natsURL := startNATSContainer(t)

	opts := []Option{
		WithHeartbeatInterval(200 * time.Millisecond),
		WithHeartbeatTTL(500 * time.Millisecond),
		WithMemberTimeout(1 * time.Second),
		WithCheckInterval(200 * time.Millisecond),
	}

	p1, c1 := setupClusterFromURL(t, natsURL, "integ-crash", opts...)
	err := p1.StartMember(c1)
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	p2, c2 := setupClusterFromURL(t, natsURL, "integ-crash", opts...)
	err = p2.StartMember(c2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	p1ID := p1.self.ID
	require.Eventually(t, func() bool {
		p2.membersMu.RLock()
		_, found := p2.members[p1ID]
		p2.membersMu.RUnlock()
		return found
	}, 10*time.Second, 200*time.Millisecond, "p2 should discover p1")

	// Crash p1 (no graceful leave).
	p1.shutdown.Store(true)
	if p1.cancel != nil {
		p1.cancel()
	}
	p1.wg.Wait()

	// p2 should detect crash via local timeout.
	require.Eventually(t, func() bool {
		p2.membersMu.RLock()
		_, found := p2.members[p1ID]
		p2.membersMu.RUnlock()
		return !found
	}, 10*time.Second, 200*time.Millisecond, "p2 should detect p1 crash")
}

func TestIntegration_LeaderElection_ThreeNodes(t *testing.T) {
	natsURL := startNATSContainer(t)

	providers := make([]*Provider, 3)
	for i := 0; i < 3; i++ {
		p, c := setupClusterFromURL(t, natsURL, "integ-leader-3")
		err := p.StartMember(c)
		require.NoError(t, err)
		t.Cleanup(func() { _ = p.Shutdown(true) })
		providers[i] = p
		time.Sleep(300 * time.Millisecond)
	}

	require.Eventually(t, func() bool {
		leaderCount := 0
		for _, p := range providers {
			if p.isLeader.Load() {
				leaderCount++
			}
		}
		return leaderCount == 1
	}, 10*time.Second, 200*time.Millisecond, "exactly one should be leader")

	leaderCount := 0
	for _, p := range providers {
		if p.isLeader.Load() {
			leaderCount++
		}
	}
	assert.Equal(t, 1, leaderCount)
}

func TestIntegration_LeaderFailover(t *testing.T) {
	natsURL := startNATSContainer(t)

	leaderTTL := 3 * time.Second
	opts := []Option{
		WithLeaderTTL(leaderTTL),
		WithHeartbeatInterval(200 * time.Millisecond),
		WithHeartbeatTTL(1 * time.Second),
		WithMemberTimeout(2 * time.Second),
		WithCheckInterval(200 * time.Millisecond),
	}

	p1, c1 := setupClusterFromURL(t, natsURL, "integ-failover", opts...)
	err := p1.StartMember(c1)
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	p2, c2 := setupClusterFromURL(t, natsURL, "integ-failover", opts...)
	err = p2.StartMember(c2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	require.Eventually(t, func() bool {
		return p1.isLeader.Load()
	}, 5*time.Second, 100*time.Millisecond, "p1 should be leader")

	// Crash p1.
	p1.shutdown.Store(true)
	if p1.cancel != nil {
		p1.cancel()
	}
	p1.wg.Wait()

	// p2 should become leader after TTL expires.
	require.Eventually(t, func() bool {
		return p2.isLeader.Load()
	}, leaderTTL+5*time.Second, 200*time.Millisecond, "p2 should become leader after failover")
}
```

**Step 2: Run integration tests**

Run: `cd /home/cchamplin/development/protoactor-go/cluster/clusterproviders/natsstream && go test -tags=integration -v -count=1 -timeout 300s`
Expected: All integration tests PASS (requires Docker)

**Step 3: Commit**

```bash
git add cluster/clusterproviders/natsstream/natsstream_integration_test.go
git commit -m "test(natsstream): add integration tests with testcontainers"
```

---

## Task 8: Single-Node Example

**Files:**
- Create: `examples/cluster-nats-stream/main.go`
- Create: `examples/cluster-nats-stream/docker-compose.yml`
- Create: `examples/cluster-nats-stream/go.mod`

**Step 1: Create `docker-compose.yml`**

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

**Step 2: Create `main.go`**

```go
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/cluster/clusterproviders/natsstream"
	"github.com/awevoke/protoactor-go/remote"
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

	provider, err := natsstream.New(nc)
	if err != nil {
		log.Fatalf("Failed to create NATS JetStream provider: %v", err)
	}

	lookup := provider.IdentityLookup()
	remoteCfg := remote.Configure("127.0.0.1", 0)
	clusterCfg := cluster.Configure("example-cluster", provider, lookup, remoteCfg)
	c := cluster.NewCluster(system, clusterCfg)

	if err := c.StartMember(); err != nil {
		log.Fatalf("Failed to start cluster member: %v", err)
	}

	fmt.Println("Cluster member started with NATS JetStream provider. Press Ctrl+C to stop.")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("Shutting down...")
	c.Shutdown(true)
}
```

**Step 3: Create `go.mod`**

```
module github.com/awevoke/protoactor-go/examples/cluster-nats-stream

go 1.25.3

require (
	github.com/awevoke/protoactor-go v0.0.0
	github.com/awevoke/protoactor-go/cluster/clusterproviders/natsstream v0.0.0
	github.com/nats-io/nats.go v1.48.0
)

replace (
	github.com/awevoke/protoactor-go => ../../
	github.com/awevoke/protoactor-go/cluster/clusterproviders/natsstream => ../../cluster/clusterproviders/natsstream
)
```

**Step 4: Run `go mod tidy` and build**

Run: `cd /home/cchamplin/development/protoactor-go/examples/cluster-nats-stream && go mod tidy && go build -o /dev/null .`
Expected: Build succeeds

**Step 5: Commit**

```bash
git add examples/cluster-nats-stream/
git commit -m "feat(examples): add single-node NATS JetStream cluster example"
```

---

## Task 9: Multi-Node Example

**Files:**
- Create: `examples/cluster-nats-stream-multi/node/main.go`
- Create: `examples/cluster-nats-stream-multi/client/main.go`
- Create: `examples/cluster-nats-stream-multi/shared/protos.go`
- Create: `examples/cluster-nats-stream-multi/docker-compose.yml`
- Create: `examples/cluster-nats-stream-multi/go.mod`

**Step 1: Create `shared/protos.go`**

```go
package shared

import "github.com/awevoke/protoactor-go/actor"

const HelloKind = "hello"

// HelloActor is a simple grain that responds to string messages.
type HelloActor struct{}

func (h *HelloActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		ctx.Logger().Info("HelloActor started")
	case string:
		ctx.Respond("Hello, " + msg + "!")
	}
}
```

**Step 2: Create `node/main.go`**

```go
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/cluster/clusterproviders/natsstream"
	"github.com/awevoke/protoactor-go/examples/cluster-nats-stream-multi/shared"
	"github.com/awevoke/protoactor-go/remote"
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

	provider, err := natsstream.New(nc)
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	helloKind := cluster.NewKind(shared.HelloKind, actor.PropsFromProducer(func() actor.Actor {
		return &shared.HelloActor{}
	}))

	lookup := provider.IdentityLookup()
	remoteCfg := remote.Configure("0.0.0.0", 0)
	clusterCfg := cluster.Configure("multi-example", provider, lookup, remoteCfg,
		cluster.WithKinds(helloKind))
	c := cluster.NewCluster(system, clusterCfg)

	if err := c.StartMember(); err != nil {
		log.Fatalf("Failed to start cluster member: %v", err)
	}

	fmt.Println("Cluster node started. Press Ctrl+C to stop.")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("Shutting down...")
	c.Shutdown(true)
}
```

**Step 3: Create `client/main.go`**

```go
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/cluster/clusterproviders/natsstream"
	"github.com/awevoke/protoactor-go/remote"
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

	provider, err := natsstream.New(nc)
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	lookup := provider.IdentityLookup()
	remoteCfg := remote.Configure("0.0.0.0", 0)
	clusterCfg := cluster.Configure("multi-example", provider, lookup, remoteCfg)
	c := cluster.NewCluster(system, clusterCfg)

	if err := c.StartClient(); err != nil {
		log.Fatalf("Failed to start cluster client: %v", err)
	}

	fmt.Println("Cluster client started.")

	// Give time for topology to settle.
	time.Sleep(3 * time.Second)

	// Make a grain call (will be placed on a cluster node).
	fmt.Println("Attempting grain call...")
	// Note: actual grain calls require registered kinds on the cluster members.
	// This client demo shows the startup/shutdown flow.

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("Shutting down...")
	c.Shutdown(true)
}
```

**Step 4: Create `docker-compose.yml`**

```yaml
version: "3"
services:
  nats:
    image: nats:2-alpine
    ports:
      - "4222:4222"
      - "8222:8222"
    command: ["-js", "-m", "8222"]

  node1:
    build:
      context: .
      dockerfile: Dockerfile.node
    environment:
      - NATS_URL=nats://nats:4222
    depends_on:
      - nats

  node2:
    build:
      context: .
      dockerfile: Dockerfile.node
    environment:
      - NATS_URL=nats://nats:4222
    depends_on:
      - nats

  node3:
    build:
      context: .
      dockerfile: Dockerfile.node
    environment:
      - NATS_URL=nats://nats:4222
    depends_on:
      - nats

  client:
    build:
      context: .
      dockerfile: Dockerfile.client
    environment:
      - NATS_URL=nats://nats:4222
    depends_on:
      - nats
      - node1
      - node2
      - node3
```

**Step 5: Create `go.mod`**

```
module github.com/awevoke/protoactor-go/examples/cluster-nats-stream-multi

go 1.25.3

require (
	github.com/awevoke/protoactor-go v0.0.0
	github.com/awevoke/protoactor-go/cluster/clusterproviders/natsstream v0.0.0
	github.com/nats-io/nats.go v1.48.0
)

replace (
	github.com/awevoke/protoactor-go => ../../
	github.com/awevoke/protoactor-go/cluster/clusterproviders/natsstream => ../../cluster/clusterproviders/natsstream
)
```

**Step 6: Run `go mod tidy` and build**

Run: `cd /home/cchamplin/development/protoactor-go/examples/cluster-nats-stream-multi && go mod tidy && go build -o /dev/null ./node && go build -o /dev/null ./client`
Expected: Build succeeds

**Step 7: Commit**

```bash
git add examples/cluster-nats-stream-multi/
git commit -m "feat(examples): add multi-node NATS JetStream cluster example"
```

---

## Task 10: Final Validation and Cleanup

**Files:**
- All files in `cluster/clusterproviders/natsstream/`

**Step 1: Run all unit tests**

Run: `cd /home/cchamplin/development/protoactor-go/cluster/clusterproviders/natsstream && go test -v -count=1 -timeout 120s`
Expected: All tests PASS

**Step 2: Run `go vet`**

Run: `cd /home/cchamplin/development/protoactor-go/cluster/clusterproviders/natsstream && go vet ./...`
Expected: No issues

**Step 3: Verify examples build**

Run: `cd /home/cchamplin/development/protoactor-go/examples/cluster-nats-stream && go build -o /dev/null . && cd /home/cchamplin/development/protoactor-go/examples/cluster-nats-stream-multi && go build -o /dev/null ./node && go build -o /dev/null ./client`
Expected: All builds succeed

**Step 4: Run integration tests (if Docker available)**

Run: `cd /home/cchamplin/development/protoactor-go/cluster/clusterproviders/natsstream && go test -tags=integration -v -count=1 -timeout 300s`
Expected: All integration tests PASS

**Step 5: Final commit if any cleanup was needed**

```bash
git add -A cluster/clusterproviders/natsstream/ examples/cluster-nats-stream/ examples/cluster-nats-stream-multi/
git commit -m "chore(natsstream): final validation and cleanup"
```

---

## Summary of All Files Created

```
cluster/clusterproviders/natsstream/
├── go.mod                              # Module with nats.go v1.48.0
├── config.go                           # Options pattern, defaults, RoleType
├── config_test.go                      # Config defaults and option tests
├── node.go                             # Node struct, serialization, MemberStatus
├── node_test.go                        # Node construction, serialization, equality
├── singleton.go                        # SingletonScheduler + RoleChangedListener
├── singleton_test.go                   # Singleton spawn/poison tests
├── natsstream_provider.go              # ClusterProvider: streams, heartbeat, consumer, crash detection, leader election
├── natsstream_identity.go              # IdentityLookup: CAS locking, activation, member cleanup
├── testhelpers_test.go                 # Embedded NATS, connection helpers, setupCluster
├── natsstream_provider_test.go         # Provider lifecycle, discovery, crash, leader, singleton tests
├── natsstream_identity_test.go         # Identity lock, store, get, remove tests
└── natsstream_integration_test.go      # Testcontainer-based multi-member tests

examples/cluster-nats-stream/
├── main.go                             # Single-node example
├── docker-compose.yml                  # NATS server
└── go.mod

examples/cluster-nats-stream-multi/
├── node/main.go                        # Cluster member with HelloKind
├── client/main.go                      # Cluster client
├── shared/protos.go                    # Shared HelloActor
├── docker-compose.yml                  # NATS + 3 nodes + client
└── go.mod
```

## Key Differences from NATS KV Provider

| Aspect | KV Provider (`natskv`) | Stream Provider (`natsstream`) |
|--------|------------------------|-------------------------------|
| Storage | KV buckets with bucket-level TTL | Raw streams with per-message TTL |
| Crash Detection | Server-driven (KV watcher sees deletes on TTL expiry) | Local timeout (periodic scan of lastSeen map) |
| Leader Election | KV atomic Create | Publish-race CAS via WithExpectLastSubjectSequence |
| Member Discovery | KV Watch with IncludeHistory/UpdatesOnly | Ordered consumer on stream |
| Identity Storage | Separate KV bucket (no TTL) | Separate stream (no TTL, MaxMsgsPerSubject: 1) |
| Identity Locking | KV Create (atomic) | Publish with expected seq=0 |
| Identity CAS | KV Update with revision | Publish with expected last subject seq |
| Member Tracking | KV bucket for member→keys | In-memory map with mutex |
| Goroutines | 3 (watcher, leader watcher, refresh) | 5 (heartbeat, consumer, stale checker, leader election, role notify) |
| Compaction | KV inherent (one value per key) | MaxMsgsPerSubject: 1 |
