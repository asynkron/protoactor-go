package natskv

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time check that Provider implements KindUpdater.
var _ cluster.KindUpdater = (*Provider)(nil)

// mockRoleListener records role changes via a callback.
type mockRoleListener struct {
	callback func(cluster.RoleType)
}

func (m *mockRoleListener) OnRoleChanged(r cluster.RoleType) { m.callback(r) }

// setupCluster creates a provider, actor system, and cluster for testing.
// It returns the provider, cluster, and a cleanup function.
// Each call produces a unique cluster name to avoid bucket collisions.
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

func TestProvider_GetHealthStatus_NilByDefault(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc, _ := connectNATS(t, srv)

	p, err := New(nc)
	require.NoError(t, err)

	assert.NoError(t, p.GetHealthStatus())
}

// --- Task 8: Provider unit tests (registration, discovery, shutdown) ---

func TestStartMember_RegistersSelfInKV(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-register")

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	// Verify the member key exists in the KV bucket with correct data.
	require.NotNil(t, p.self, "self should be initialized")
	key := p.memberKey(p.self.ID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	entry, err := p.memberBucket.Get(ctx, key)
	require.NoError(t, err, "member key should exist in KV")

	var node Node
	require.NoError(t, json.Unmarshal(entry.Value(), &node))
	assert.Equal(t, p.self.ID, node.ID)
	assert.True(t, node.Alive, "registered node should be alive")
	assert.Equal(t, p.self.Host, node.Host)
	assert.Equal(t, p.self.Port, node.Port)
}

func TestStartMember_DiscoversExistingMembers(t *testing.T) {
	srv := startEmbeddedNATS(t)

	// Start first provider/member.
	p1, c1 := setupCluster(t, srv, "test-discover")
	err := p1.StartMember(c1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p1.Shutdown(true) })

	// Give the first member time to register and settle.
	time.Sleep(500 * time.Millisecond)

	// Start second provider/member on the same cluster.
	p2, c2 := setupCluster(t, srv, "test-discover")
	err = p2.StartMember(c2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	// Give time for discovery.
	time.Sleep(500 * time.Millisecond)

	// Second provider should see the first member in its members map.
	p2.membersMu.RLock()
	defer p2.membersMu.RUnlock()

	found := false
	for _, m := range p2.members {
		if m.ID == p1.self.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "second provider should discover the first member")
}

func TestStartClient_WatchOnly(t *testing.T) {
	srv := startEmbeddedNATS(t)

	// Start a member first so there's something in the bucket.
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

	// Client should have self != nil (init is called).
	require.NotNil(t, pClient.self, "client self should be initialized")

	// Client should NOT have a key in KV (StartClient does not call registerSelf).
	clientKey := pClient.memberKey(pClient.self.ID)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = pClient.memberBucket.Get(ctx, clientKey)
	assert.Error(t, err, "client should NOT register a key in KV")

	// Client should discover the member.
	time.Sleep(500 * time.Millisecond)
	pClient.membersMu.RLock()
	defer pClient.membersMu.RUnlock()

	found := false
	for _, m := range pClient.members {
		if m.ID == pMember.self.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "client should discover the member")
}

func TestShutdown_Graceful(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-shutdown")

	err := p.StartMember(c)
	require.NoError(t, err)

	require.NotNil(t, p.self)
	key := p.memberKey(p.self.ID)

	// Verify key exists before shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = p.memberBucket.Get(ctx, key)
	require.NoError(t, err, "member key should exist before shutdown")

	// Shutdown gracefully.
	err = p.Shutdown(true)
	require.NoError(t, err)

	// Verify key is deleted after shutdown.
	_, err = p.memberBucket.Get(ctx, key)
	assert.Error(t, err, "member key should be deleted after graceful shutdown")
}

func TestShutdown_Idempotent(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-shutdown-idem")

	err := p.StartMember(c)
	require.NoError(t, err)

	// Shutdown twice -- no panic expected.
	err = p.Shutdown(true)
	assert.NoError(t, err)

	err = p.Shutdown(true)
	assert.NoError(t, err, "second shutdown should not fail or panic")
}

func TestLeaderElection_FirstWins(t *testing.T) {
	srv := startEmbeddedNATS(t)

	p1, c1 := setupCluster(t, srv, "test-leader")
	err := p1.StartMember(c1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p1.Shutdown(true) })

	// Give the first member time to attempt leader election.
	time.Sleep(500 * time.Millisecond)

	p2, c2 := setupCluster(t, srv, "test-leader")
	err = p2.StartMember(c2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	// Give the second member time to attempt leader election.
	time.Sleep(500 * time.Millisecond)

	// Exactly one should be leader.
	p1IsLeader := p1.isLeader.Load()
	p2IsLeader := p2.isLeader.Load()

	assert.True(t, p1IsLeader || p2IsLeader, "at least one provider should be leader")
	assert.False(t, p1IsLeader && p2IsLeader, "only one provider should be leader at a time")

	// The first member should be the leader (it registered first).
	assert.True(t, p1IsLeader, "first member should win leader election")
}

func TestRoleChangedListener_Called(t *testing.T) {
	srv := startEmbeddedNATS(t)

	var gotRole atomic.Int32
	gotRole.Store(-1) // sentinel: no call yet

	listener := &mockRoleListener{
		callback: func(r cluster.RoleType) {
			gotRole.Store(int32(r))
		},
	}

	p, c := setupCluster(t, srv, "test-rolechange", WithRoleChangedListener(listener))
	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	// As the only member, this provider will become leader.
	// Wait for the role changed listener to be called.
	require.Eventually(t, func() bool {
		return gotRole.Load() == int32(cluster.RoleLeader)
	}, 5*time.Second, 100*time.Millisecond, "role changed listener should be called with Leader")
}

// --- Task 9: Crash detection test ---

func TestMemberCrash_TTLExpiry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TTL expiry test in short mode")
	}

	srv := startEmbeddedNATS(t)

	ttl := 2 * time.Second
	refresh := 500 * time.Millisecond

	// Start first member with short TTL.
	p1, c1 := setupCluster(t, srv, "test-crash",
		WithMemberTTL(ttl),
		WithRefreshInterval(refresh),
	)
	err := p1.StartMember(c1)
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	// Start second member with short TTL.
	p2, c2 := setupCluster(t, srv, "test-crash",
		WithMemberTTL(ttl),
		WithRefreshInterval(refresh),
	)
	err = p2.StartMember(c2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	// Verify p2 sees p1.
	time.Sleep(500 * time.Millisecond)
	p2.membersMu.RLock()
	_, found := p2.members[p1.self.ID]
	p2.membersMu.RUnlock()
	require.True(t, found, "p2 should see p1 before crash")

	// "Crash" first member: cancel context and set shutdown flag to stop refresh.
	p1.shutdown.Store(true)
	if p1.cancel != nil {
		p1.cancel()
	}
	p1.wg.Wait()

	// Wait for TTL to expire plus buffer.
	time.Sleep(ttl + 2*time.Second)

	// After TTL expiry, p2's watcher should have removed p1 from its members.
	p2.membersMu.RLock()
	_, stillFound := p2.members[p1.self.ID]
	p2.membersMu.RUnlock()
	assert.False(t, stillFound, "second member should no longer see the crashed member after TTL expiry")
}

// --- Task 10: Singleton scheduler integration test ---

func TestSingletonScheduler_SpawnOnLeader(t *testing.T) {
	srv := startEmbeddedNATS(t)

	var spawned atomic.Bool

	p, c := setupCluster(t, srv, "test-singleton")

	// Create a singleton scheduler and register a simple actor.
	scheduler := cluster.NewSingletonScheduler(c.ActorSystem.Root)
	scheduler.FromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *actor.Started:
			spawned.Store(true)
		}
	})
	p.RegisterSingletonScheduler(scheduler)

	// Start the member (only member = will become leader).
	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	// Wait for leader election and singleton spawn.
	require.Eventually(t, func() bool {
		return spawned.Load()
	}, 5*time.Second, 100*time.Millisecond, "singleton actor should be spawned on leader")
}

// --- Task 7: Late-registration and concurrent race tests for singleton scheduler ---

func TestSingletonScheduler_RegisterAfterStart_SpawnsImmediately(t *testing.T) {
	srv := startEmbeddedNATS(t)

	p, c := setupCluster(t, srv, "test-singleton-late")

	// Start the member first (only member = will become leader).
	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(false) })

	// Wait for leader election.
	require.Eventually(t, func() bool {
		return p.isLeader.Load()
	}, 5*time.Second, 100*time.Millisecond, "provider should become leader")

	// Create scheduler AFTER start and leader election.
	var spawned atomic.Bool
	scheduler := cluster.NewSingletonScheduler(c.ActorSystem.Root)
	scheduler.FromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *actor.Started:
			spawned.Store(true)
		}
	})

	// Register after start — should immediately notify since already leader.
	p.RegisterSingletonScheduler(scheduler)

	// Assert actor spawns within timeout.
	require.Eventually(t, func() bool {
		return spawned.Load()
	}, 5*time.Second, 100*time.Millisecond, "late-registered singleton actor should be spawned immediately on leader")
}

func TestSingletonScheduler_ConcurrentRegisterAndRoleChange(t *testing.T) {
	srv := startEmbeddedNATS(t)

	p, c := setupCluster(t, srv, "test-singleton-race")

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(false) })

	// Wait for leader election.
	require.Eventually(t, func() bool {
		return p.isLeader.Load()
	}, 5*time.Second, 100*time.Millisecond, "provider should become leader")

	// Concurrently register schedulers and toggle role.
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			scheduler := cluster.NewSingletonScheduler(c.ActorSystem.Root)
			scheduler.FromFunc(func(ctx actor.Context) {})
			p.RegisterSingletonScheduler(scheduler)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			if i%2 == 0 {
				p.setRole(cluster.RoleFollower)
			} else {
				p.setRole(cluster.RoleLeader)
			}
		}
	}()

	wg.Wait()
}

// --- Task 5: KindUpdater ---

func TestProvider_UpdateKinds(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-updatekinds")

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	originalKinds := p.self.Kinds

	// Update kinds
	newKinds := append([]string{}, originalKinds...)
	newKinds = append(newKinds, "dynamicKind")
	err = p.UpdateKinds(newKinds)
	require.NoError(t, err)

	// Verify self.Kinds is updated
	assert.ElementsMatch(t, newKinds, p.self.Kinds)

	// Verify the KV bucket reflects the update
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := p.memberKey(p.self.ID)
	entry, err := p.memberBucket.Get(ctx, key)
	require.NoError(t, err)

	var node Node
	require.NoError(t, json.Unmarshal(entry.Value(), &node))
	assert.ElementsMatch(t, newKinds, node.Kinds)
}
