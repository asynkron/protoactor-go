package storage_test

import (
	"sync"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/identitylookup"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/storage"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testKind = "test-grain"

// testClusterProvider is a minimal ClusterProvider for tests.
type testClusterProvider struct{}

func (p *testClusterProvider) StartMember(_ *cluster.Cluster) error { return nil }
func (p *testClusterProvider) StartClient(_ *cluster.Cluster) error { return nil }
func (p *testClusterProvider) Shutdown(_ bool) error                { return nil }

// setupTestCluster creates a cluster with InMemoryStorageLookup and an
// IdentityStorageLookup, ready for testing. It registers a simple echo
// kind, starts the remote layer (to get a real address), and publishes
// a self-only topology so the strategy manager has a member to target.
func setupTestCluster(t *testing.T) (*cluster.Cluster, *storage.IdentityStorageLookup, *identitylookup.InMemoryStorageLookup) {
	t.Helper()

	props := actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *actor.Started:
			// no-op
		}
	})

	kind := cluster.NewKind(testKind, props)
	storageLookup := identitylookup.NewInMemoryStorageLookup()
	isl := storage.New(storageLookup)
	system := actor.NewActorSystem()
	provider := &testClusterProvider{}
	remoteCfg := remote.Configure("127.0.0.1", 0)
	cfg := cluster.Configure("test-cluster", provider, isl, remoteCfg, cluster.WithKinds(kind))
	c := cluster.NewCluster(system, cfg)

	// Set up remote so the actor system has a real address.
	c.Remote = remote.NewRemote(system, c.Config.RemoteConfig)
	err := c.Remote.Start()
	require.NoError(t, err)

	// Initialize kinds so the placement actor can look them up.
	c.InitKindsForTest(kind)

	// Set up the identity lookup (spawns placement + proxy actors).
	isl.Setup(c, []string{testKind}, false)

	// Publish a self-only topology so the strategy manager knows about us.
	host, port, err := system.GetHostPort()
	require.NoError(t, err)
	self := &cluster.Member{
		Host:  host,
		Port:  int32(port),
		Id:    system.ID,
		Kinds: []string{testKind},
	}
	c.MemberList.UpdateClusterTopology(cluster.Members{self})

	t.Cleanup(func() {
		isl.Shutdown()
		c.Remote.Shutdown(true)
	})

	return c, isl, storageLookup
}

func TestIdentityStorageLookup_SetupSpawnsPlacementAndProxy(t *testing.T) {
	_, isl, _ := setupTestCluster(t)

	// After Setup, the placement and proxy PIDs should be accessible
	// by attempting to resolve a known actor name.
	// We verify indirectly: calling Get should work (which requires
	// the placement actor to be alive).
	ci := &cluster.ClusterIdentity{Kind: testKind, Identity: "setup-test"}
	pid := isl.Get(ci)
	require.NotNil(t, pid, "Get() should return a PID after Setup()")
}

func TestIdentityStorageLookup_CoalesceConcurrentGets(t *testing.T) {
	_, isl, _ := setupTestCluster(t)

	ci := &cluster.ClusterIdentity{Kind: testKind, Identity: "coalesce-test"}
	const concurrency = 10

	var wg sync.WaitGroup
	pids := make([]*actor.PID, concurrency)

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			pids[idx] = isl.Get(ci)
		}(i)
	}
	wg.Wait()

	// All should have returned a non-nil PID.
	for i, pid := range pids {
		require.NotNil(t, pid, "goroutine %d returned nil PID", i)
	}

	// All should be the same PID (coalesced).
	for i := 1; i < concurrency; i++ {
		assert.True(t, pids[0].Equal(pids[i]),
			"PID[0]=%v != PID[%d]=%v — concurrent Gets should coalesce", pids[0], i, pids[i])
	}
}

func TestIdentityStorageLookup_StaleActivationCleaned(t *testing.T) {
	c, isl, storageLookup := setupTestCluster(t)

	ci := &cluster.ClusterIdentity{Kind: testKind, Identity: "stale-test"}

	// Simulate a stale activation from a dead member by directly
	// inserting an activation record with a member ID that doesn't
	// exist in the topology.
	deadMemberID := "dead-member-123"
	stalePID := actor.NewPID("127.0.0.1:9999", testKind+"/stale-test")
	staleLock := &cluster.SpawnLock{
		LockID:          "stale-lock",
		ClusterIdentity: ci,
	}
	storageLookup.StoreActivation(deadMemberID, staleLock, stalePID)

	// Verify the stale activation exists.
	existing := storageLookup.TryGetExistingActivation(ci)
	require.NotNil(t, existing, "stale activation should exist before Get()")

	// Now Get() should detect the stale member, clean it up, and
	// spawn a fresh activation.
	pid := isl.Get(ci)
	require.NotNil(t, pid, "Get() should return a PID after cleaning stale activation")

	// The new PID should be on this node, not the dead one.
	assert.Equal(t, c.ActorSystem.Address(), pid.Address,
		"new activation should be on the local node, not the dead member")
	assert.NotEqual(t, stalePID.Address, pid.Address,
		"PID should not be the stale one")
}

func TestIdentityStorageLookup_ShutdownStopsPlacementFirst(t *testing.T) {
	c, isl, _ := setupTestCluster(t)

	ci := &cluster.ClusterIdentity{Kind: testKind, Identity: "shutdown-1"}

	// Activate an actor.
	pid := isl.Get(ci)
	require.NotNil(t, pid, "should get a PID")

	// Shutdown the lookup. This should stop placement/proxy first.
	isl.Shutdown()

	// The placement actor should be stopped after Shutdown.
	// Sending a request should timeout or return DeadLetterResponse.
	req := &cluster.ActivationRequest{
		ClusterIdentity: &cluster.ClusterIdentity{Kind: testKind, Identity: "post-shutdown"},
		RequestId:       "post-shutdown-req",
	}
	future := c.ActorSystem.Root.RequestFuture(actor.NewPID(c.ActorSystem.Address(), "$placement-activator"), req, 1*time.Second)
	_, err := future.Result()
	assert.Error(t, err, "placement actor should be stopped after Shutdown")
}

func TestIdentityStorageLookup_RemovePidCleansStorage(t *testing.T) {
	_, isl, storageLookup := setupTestCluster(t)

	ci := &cluster.ClusterIdentity{Kind: testKind, Identity: "remove-1"}

	// Activate an actor via Get().
	pid := isl.Get(ci)
	require.NotNil(t, pid)

	// Verify activation exists in storage.
	existing := storageLookup.TryGetExistingActivation(ci)
	require.NotNil(t, existing, "activation should be in storage after Get()")

	// RemovePid should remove it.
	isl.RemovePid(ci, pid)

	// Verify it's gone.
	gone := storageLookup.TryGetExistingActivation(ci)
	assert.Nil(t, gone, "activation should be removed after RemovePid")
}

func TestIdentityStorageLookup_CoalesceFirstFailAllGetNil(t *testing.T) {
	_, isl, _ := setupTestCluster(t)

	// Use a kind that doesn't exist in the cluster -> placement actor returns Failed.
	ci := &cluster.ClusterIdentity{Kind: "nonExistentKind", Identity: "fail-1"}

	const concurrency = 5
	var wg sync.WaitGroup
	pids := make([]*actor.PID, concurrency)

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			pids[idx] = isl.Get(ci)
		}(i)
	}
	wg.Wait()

	// All should return nil since the kind is unknown.
	for i := 0; i < concurrency; i++ {
		assert.Nil(t, pids[i], "PID %d should be nil for unknown kind", i)
	}
}

func TestIdentityStorageLookup_StrategySelectsLocal(t *testing.T) {
	c, isl, _ := setupTestCluster(t)

	ci := &cluster.ClusterIdentity{Kind: testKind, Identity: "strategy-local-1"}

	// With only one member (ourselves), strategy must select local.
	pid := isl.Get(ci)
	require.NotNil(t, pid, "should activate successfully")

	// The PID should be on the local address.
	localAddr := c.ActorSystem.Address()
	assert.Equal(t, localAddr, pid.Address,
		"activation should be on local member")
}

func TestIdentityStorageLookup_ClientWaitsForActivation(t *testing.T) {
	storageLookup := identitylookup.NewInMemoryStorageLookup()
	isl := storage.New(storageLookup)

	system := actor.NewActorSystem()
	kind := cluster.NewKind(testKind, actor.PropsFromFunc(func(ctx actor.Context) {}))
	provider := &testClusterProvider{}
	remoteCfg := remote.Configure("127.0.0.1", 0)
	cfg := cluster.Configure("test-cluster", provider, isl, remoteCfg, cluster.WithKinds(kind))
	c := cluster.NewCluster(system, cfg)
	c.Remote = remote.NewRemote(system, c.Config.RemoteConfig)
	err := c.Remote.Start()
	require.NoError(t, err)

	// Setup as CLIENT.
	isl.Setup(c, []string{testKind}, true)

	t.Cleanup(func() {
		isl.Shutdown()
		c.Remote.Shutdown(true)
	})

	ci := &cluster.ClusterIdentity{Kind: testKind, Identity: "client-wait-1"}

	// Start Get in background — it will block on WaitForActivation.
	var wg sync.WaitGroup
	wg.Add(1)
	var result *actor.PID
	go func() {
		defer wg.Done()
		result = isl.Get(ci)
	}()

	// Give it time to start waiting.
	time.Sleep(50 * time.Millisecond)

	// Simulate another node storing the activation.
	lock := storageLookup.TryAcquireLock(ci)
	require.NotNil(t, lock)
	storedPid := actor.NewPID("other-node:9999", testKind+"/client-wait-1")
	storageLookup.StoreActivation("other-member", lock, storedPid)

	// Wait for result with timeout.
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("client Get() did not return within 5 seconds")
	}

	require.NotNil(t, result, "client should get PID after activation is stored")
}

func TestIdentityStorageLookup_EndToEnd_ActivateAndRetrieve(t *testing.T) {
	c, isl, storageLookup := setupTestCluster(t)

	ci := &cluster.ClusterIdentity{Kind: testKind, Identity: "order-123"}

	// First Get — should activate via placement actor.
	pid1 := isl.Get(ci)
	require.NotNil(t, pid1, "first Get should return PID")

	// Verify the activation is stored in the backend.
	stored := storageLookup.TryGetExistingActivation(ci)
	require.NotNil(t, stored, "activation should be persisted in storage")
	assert.Equal(t, c.ActorSystem.ID, stored.MemberID, "member ID should be ours")

	// Second Get — should find existing activation (no new spawn).
	pid2 := isl.Get(ci)
	require.NotNil(t, pid2, "second Get should return PID")
	assert.True(t, pid1.Equal(pid2), "second Get should return same PID")
}

func TestIdentityStorageLookup_ListGrainsAfterActivation(t *testing.T) {
	_, isl, _ := setupTestCluster(t)

	// Activate two grains.
	ci1 := &cluster.ClusterIdentity{Kind: testKind, Identity: "list-a"}
	ci2 := &cluster.ClusterIdentity{Kind: testKind, Identity: "list-b"}
	pid1 := isl.Get(ci1)
	pid2 := isl.Get(ci2)
	require.NotNil(t, pid1)
	require.NotNil(t, pid2)

	// ListGrains should return both.
	grains, err := isl.ListGrains()
	require.NoError(t, err)
	assert.Len(t, grains, 2, "should list 2 grains")
}

func TestIdentityStorageLookup_DifferentIdentitiesNotCoalesced(t *testing.T) {
	_, isl, _ := setupTestCluster(t)

	ci1 := &cluster.ClusterIdentity{Kind: testKind, Identity: "identity-A"}
	ci2 := &cluster.ClusterIdentity{Kind: testKind, Identity: "identity-B"}

	var wg sync.WaitGroup
	var pid1, pid2 *actor.PID

	wg.Add(2)
	go func() {
		defer wg.Done()
		pid1 = isl.Get(ci1)
	}()
	go func() {
		defer wg.Done()
		pid2 = isl.Get(ci2)
	}()
	wg.Wait()

	require.NotNil(t, pid1, "identity-A should resolve")
	require.NotNil(t, pid2, "identity-B should resolve")
	assert.False(t, pid1.Equal(pid2),
		"different identities should produce different PIDs: got %v and %v", pid1, pid2)
}

func TestStorageLookup_Peek_Alive(t *testing.T) {
	c, isl, _ := setupTestCluster(t)

	ci := &cluster.ClusterIdentity{Kind: testKind, Identity: "peek-alive-1"}
	pid := isl.Get(ci)
	require.NotNil(t, pid)

	result, err := isl.Peek(ci)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusAlive, result.Status)
	assert.Equal(t, "peek-alive-1", result.Identity)
	assert.Equal(t, testKind, result.Kind)
	assert.Equal(t, pid, result.PID)

	_ = c
}

func TestStorageLookup_Peek_NotFound(t *testing.T) {
	c, isl, _ := setupTestCluster(t)

	ci := &cluster.ClusterIdentity{Kind: testKind, Identity: "never-activated"}
	result, err := isl.Peek(ci)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusNotFound, result.Status)

	_ = c
}

func TestStorageLookup_Peek_MemberDead(t *testing.T) {
	c, isl, storageLookup := setupTestCluster(t)

	ci := &cluster.ClusterIdentity{Kind: testKind, Identity: "dead-member-grain"}
	lock := storageLookup.TryAcquireLock(ci)
	require.NotNil(t, lock)

	fakePID := actor.NewPID("dead-host:9999", testKind+"/dead-member-grain")
	storageLookup.StoreActivation("dead-member-id", lock, fakePID)

	result, err := isl.Peek(ci)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusMemberDead, result.Status)
	assert.Equal(t, "dead-member-grain", result.Identity)

	_ = c
}

func TestStorageLookup_Peek_Stale(t *testing.T) {
	c, isl, _ := setupTestCluster(t)

	ci := &cluster.ClusterIdentity{Kind: testKind, Identity: "peek-stale-1"}
	pid := isl.Get(ci)
	require.NotNil(t, pid)

	c.ActorSystem.Root.Poison(pid)

	require.Eventually(t, func() bool {
		r, err := isl.Peek(ci)
		return err == nil && r.Status == cluster.PeekStatusStale
	}, 5*time.Second, 50*time.Millisecond)
}
