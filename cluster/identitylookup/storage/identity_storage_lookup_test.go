package storage_test

import (
	"sync"
	"testing"

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

