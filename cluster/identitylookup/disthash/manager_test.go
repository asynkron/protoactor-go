package disthash

import (
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/test"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"sync"
	"testing"
	"time"
)

func TestPlacementActorUnknownKind(t *testing.T) {
	system := actor.NewActorSystem()
	provider := test.NewTestProvider(test.NewInMemAgent())
	lookup := New()
	config := cluster.Configure("test-cluster", provider, lookup, remote.Configure("127.0.0.1", 0))
	c := cluster.NewCluster(system, config)

	manager := newPartitionManager(c)
	manager.Start()
	defer manager.Stop()

	identity := &cluster.ClusterIdentity{Identity: "abc", Kind: "unknown"}
	req := &cluster.ActivationRequest{ClusterIdentity: identity}
	future := system.Root.RequestFuture(manager.placementActor, req, time.Second)
	res, err := future.Result()
	assert.NoError(t, err)
	resp, ok := res.(*cluster.ActivationResponse)
	assert.True(t, ok)
	assert.True(t, resp.Failed)
	assert.Nil(t, resp.Pid)
}

func TestManagerConcurrentAccess(t *testing.T) {
	system := actor.NewActorSystem()
	provider := test.NewTestProvider(test.NewInMemAgent())
	lookup := New()
	config := cluster.Configure("test-cluster", provider, lookup, remote.Configure("127.0.0.1", 0))
	c := cluster.NewCluster(system, config)

	manager := newPartitionManager(c)
	manager.Start()
	defer manager.Stop()

	// Create a WaitGroup to synchronize goroutines
	var wg sync.WaitGroup
	iterations := 1000

	// Simulate concurrent topology updates and lookups
	wg.Add(2)

	// Goroutine 1: Continuously update topology
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			members := []*cluster.Member{
				{Id: "1", Host: "localhost", Port: 1},
				{Id: "2", Host: "localhost", Port: 2},
			}
			topology := &cluster.ClusterTopology{
				Members:      members,
				TopologyHash: uint64(i),
			}
			manager.onClusterTopology(topology)
		}
	}()

	// Goroutine 2: Continuously perform lookups
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			identity := &cluster.ClusterIdentity{
				Identity: "test",
				Kind:     "test",
			}
			_ = manager.Get(identity)
		}
	}()

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Test completed successfully
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out")
	}
}

// Integration test suite
type DistHashManagerTestSuite struct {
	suite.Suite
	clusters []*cluster.Cluster
}

func (suite *DistHashManagerTestSuite) SetupTest() {
	// Create 3 cluster nodes for testing
	suite.clusters = make([]*cluster.Cluster, 3)
	inMemAgent := test.NewInMemAgent()

	for i := 0; i < 3; i++ {
		system := actor.NewActorSystem()
		provider := test.NewTestProvider(inMemAgent)
		config := cluster.Configure("test-cluster",
			provider,
			New(),
			remote.Configure("localhost", 0),
		)

		c := cluster.NewCluster(system, config)
		if err := c.StartMember(); err != nil {
			suite.T().Fatalf("failed to start member %d: %v", i, err)
		}
		suite.clusters[i] = c
	}
}

func (suite *DistHashManagerTestSuite) TearDownTest() {
	for _, c := range suite.clusters {
		c.Shutdown(true)
	}
}

func (suite *DistHashManagerTestSuite) TestConcurrentClusterOperations() {
	assert.Equal(suite.T(), 3, len(suite.clusters))

	// Create multiple concurrent operations
	var wg sync.WaitGroup
	iterations := 100

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()

			// Randomly select a cluster
			cluster := suite.clusters[iteration%len(suite.clusters)]

			// Perform a Get operation
			identity := fmt.Sprintf("test-%d", iteration)
			pid := cluster.Get(identity, "test-kind")

			// Verify the operation completed without panicking
			assert.NotPanics(suite.T(), func() {
				if pid != nil {
					// Optionally verify the PID properties
					assert.NotEmpty(suite.T(), pid.Address)
					assert.NotEmpty(suite.T(), pid.Id)
				}
			})
		}(i)
	}

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Test completed successfully
	case <-time.After(10 * time.Second):
		suite.T().Fatal("Test timed out")
	}
}

// TestGetDoesNotBlockTopologyUpdate verifies that concurrent Get calls do
// not hold the rdvMutex across the blocking RPC, which would prevent
// topology updates from proceeding.  Before the fix, Get held an RLock
// for the entire duration of the request future (up to 5 s), starving
// onClusterTopology which needs a write lock.
func TestGetDoesNotBlockTopologyUpdate(t *testing.T) {
	system := actor.NewActorSystem()
	provider := test.NewTestProvider(test.NewInMemAgent())
	lookup := New()
	config := cluster.Configure("test-cluster", provider, lookup, remote.Configure("127.0.0.1", 0))
	c := cluster.NewCluster(system, config)

	manager := newPartitionManager(c)
	manager.Start()
	defer manager.Stop()

	// Seed the rendezvous with an initial topology so Get performs a
	// real lookup that will block waiting for a response.
	members := []*cluster.Member{
		{Id: "1", Host: "127.0.0.1", Port: 9999, Kinds: []string{"test"}},
	}
	manager.onClusterTopology(&cluster.ClusterTopology{
		Members:      members,
		TopologyHash: 1,
	})

	// Launch several concurrent Get calls.  Each one will block on
	// RequestFuture (nobody is listening at 127.0.0.1:9999 for the
	// activation request, so these will time out after 5 s).  If the
	// old code is in place, the RLock would be held during this wait.
	const concurrency = 5
	var getWg sync.WaitGroup
	getWg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func(n int) {
			defer getWg.Done()
			identity := &cluster.ClusterIdentity{
				Identity: fmt.Sprintf("actor-%d", n),
				Kind:     "test",
			}
			_ = manager.Get(identity)
		}(i)
	}

	// Give the Get goroutines a moment to start and enter the RPC wait.
	time.Sleep(50 * time.Millisecond)

	// Now try to push a topology update.  With the fix, this should
	// succeed almost immediately because Get no longer holds the RLock
	// across the blocking call.  With the old code, this would have to
	// wait up to 5 s for every in-flight Get to finish.
	topologyDone := make(chan struct{})
	go func() {
		manager.onClusterTopology(&cluster.ClusterTopology{
			Members: []*cluster.Member{
				{Id: "1", Host: "127.0.0.1", Port: 9999, Kinds: []string{"test"}},
				{Id: "2", Host: "127.0.0.1", Port: 9998, Kinds: []string{"test"}},
			},
			TopologyHash: 2,
		})
		close(topologyDone)
	}()

	select {
	case <-topologyDone:
		// Topology update completed promptly - TOCTOU fix is working.
	case <-time.After(2 * time.Second):
		t.Fatal("topology update blocked for >2 s; Get is likely still holding rdvMutex across the RPC call")
	}

	// Wait for the Get goroutines to finish (they will time out).
	getWg.Wait()
}

// TestPlacementActorDeduplicatesSpawns verifies that sending multiple
// activation requests for the same identity+kind returns the same PID
// rather than spawning duplicate actors.
func TestPlacementActorDeduplicatesSpawns(t *testing.T) {
	system := actor.NewActorSystem()
	provider := test.NewTestProvider(test.NewInMemAgent())
	lookup := New()

	// Register a kind so the placement actor can spawn it.
	kind := cluster.NewKind("test-kind", actor.PropsFromFunc(func(ctx actor.Context) {}))
	config := cluster.Configure("test-cluster", provider, lookup,
		remote.Configure("127.0.0.1", 0),
		cluster.WithKinds(kind),
	)
	c := cluster.NewCluster(system, config)

	// initKinds is called during StartMember, but we are testing the
	// placement actor in isolation. Manually build the kind so
	// GetClusterKind returns a non-nil value.
	c.InitKindsForTest(kind)

	manager := newPartitionManager(c)
	manager.Start()
	defer manager.Stop()

	identity := &cluster.ClusterIdentity{Identity: "actor-1", Kind: "test-kind"}
	req := &cluster.ActivationRequest{ClusterIdentity: identity}

	// First request should spawn a new actor.
	future1 := system.Root.RequestFuture(manager.placementActor, req, time.Second)
	res1, err := future1.Result()
	assert.NoError(t, err)
	resp1, ok := res1.(*cluster.ActivationResponse)
	assert.True(t, ok)
	assert.False(t, resp1.Failed, "first activation should succeed")
	assert.NotNil(t, resp1.Pid, "first activation should return a PID")

	// Second request for the same identity should return the same PID
	// (looked up from the actors map, not re-spawned).
	future2 := system.Root.RequestFuture(manager.placementActor, req, time.Second)
	res2, err := future2.Result()
	assert.NoError(t, err)
	resp2, ok := res2.(*cluster.ActivationResponse)
	assert.True(t, ok)
	assert.False(t, resp2.Failed, "second activation should succeed")
	assert.Equal(t, resp1.Pid, resp2.Pid, "both activations should return the same PID")
}

// TestPlacementActorConcurrentSameIdentity sends many concurrent
// activation requests for the same identity and verifies all responses
// reference the same PID (no duplicates spawned).
func TestPlacementActorConcurrentSameIdentity(t *testing.T) {
	system := actor.NewActorSystem()
	provider := test.NewTestProvider(test.NewInMemAgent())
	lookup := New()

	kind := cluster.NewKind("test-kind", actor.PropsFromFunc(func(ctx actor.Context) {}))
	config := cluster.Configure("test-cluster", provider, lookup,
		remote.Configure("127.0.0.1", 0),
		cluster.WithKinds(kind),
	)
	c := cluster.NewCluster(system, config)
	c.InitKindsForTest(kind)

	manager := newPartitionManager(c)
	manager.Start()
	defer manager.Stop()

	identity := &cluster.ClusterIdentity{Identity: "actor-concurrent", Kind: "test-kind"}
	const concurrency = 20

	// Fire off many requests concurrently.
	futures := make([]actor.Future, concurrency)
	for i := 0; i < concurrency; i++ {
		req := &cluster.ActivationRequest{ClusterIdentity: identity}
		futures[i] = system.Root.RequestFuture(manager.placementActor, req, 2*time.Second)
	}

	// Collect results.
	var firstPid *actor.PID
	successCount := 0
	for i, f := range futures {
		res, err := f.Result()
		assert.NoError(t, err, "request %d should not error", i)
		resp, ok := res.(*cluster.ActivationResponse)
		assert.True(t, ok, "request %d should return ActivationResponse", i)

		if resp.Failed {
			// A "failed" response from the spawning-guard is acceptable
			// for concurrent duplicates -- the caller will retry.
			continue
		}

		successCount++
		if firstPid == nil {
			firstPid = resp.Pid
		} else {
			assert.Equal(t, firstPid, resp.Pid,
				"request %d returned a different PID; duplicate spawn detected", i)
		}
	}

	assert.NotNil(t, firstPid, "at least one activation should succeed")
	// The first request always succeeds; subsequent ones either find it
	// in the actors map (success, same PID) or hit the spawning guard.
	assert.GreaterOrEqual(t, successCount, 1,
		"at least one activation should succeed")
}

func TestPlacementActorRebalancesOnTopology(t *testing.T) {
	system := actor.NewActorSystem()
	provider := test.NewTestProvider(test.NewInMemAgent())
	lookup := New()

	kind := cluster.NewKind("test-kind", actor.PropsFromFunc(func(ctx actor.Context) {}))
	config := cluster.Configure("test-cluster", provider, lookup,
		remote.Configure("127.0.0.1", 0),
		cluster.WithKinds(kind),
	)
	c := cluster.NewCluster(system, config)
	c.InitKindsForTest(kind)

	manager := newPartitionManager(c)
	manager.Start()
	defer manager.Stop()

	// Seed topology with a single member (this node) so all actors are local.
	// The rebalance callback compares rdv.GetByIdentity(key) against
	// pm.cluster.ActorSystem.Address(). Use GetHostPort to match exactly.
	host, port, _ := c.ActorSystem.GetHostPort()
	manager.onClusterTopology(&cluster.ClusterTopology{
		Members: []*cluster.Member{
			{Id: "node-1", Host: host, Port: int32(port), Kinds: []string{"test-kind"}},
		},
		TopologyHash: 1,
	})

	// Activate some grains.
	for i := 0; i < 5; i++ {
		ci := &cluster.ClusterIdentity{Kind: "test-kind", Identity: fmt.Sprintf("grain-%d", i)}
		req := &cluster.ActivationRequest{ClusterIdentity: ci}
		future := system.Root.RequestFuture(manager.placementActor, req, 2*time.Second)
		res, err := future.Result()
		require.NoError(t, err)
		resp := res.(*cluster.ActivationResponse)
		require.False(t, resp.Failed, "grain %d activation should succeed", i)
	}

	// Verify all 5 grains exist.
	listFuture := system.Root.RequestFuture(manager.placementActor, &cluster.ListGrainsRequest{}, 2*time.Second)
	listRes, err := listFuture.Result()
	require.NoError(t, err)
	listResp := listRes.(*cluster.ListGrainsResponse)
	require.Len(t, listResp.Grains, 5)

	// Add a second member — some grains' rendezvous owner will change.
	// The placement actor should poison those grains.
	manager.onClusterTopology(&cluster.ClusterTopology{
		Members: []*cluster.Member{
			{Id: "node-1", Host: host, Port: int32(port), Kinds: []string{"test-kind"}},
			{Id: "node-2", Host: "10.0.0.2", Port: 8080, Kinds: []string{"test-kind"}},
		},
		TopologyHash: 2,
	})

	// Wait for poisons to process.
	time.Sleep(500 * time.Millisecond)

	// List grains again — some should have been rebalanced (poisoned).
	listFuture2 := system.Root.RequestFuture(manager.placementActor, &cluster.ListGrainsRequest{}, 2*time.Second)
	listRes2, err := listFuture2.Result()
	require.NoError(t, err)
	listResp2 := listRes2.(*cluster.ListGrainsResponse)

	// At least one grain should have been removed (rebalanced to node-2).
	assert.Less(t, len(listResp2.Grains), 5,
		"some grains should have been rebalanced away after topology change")
}

func TestPlacementActorCleansUpOnTermination(t *testing.T) {
	system := actor.NewActorSystem()
	provider := test.NewTestProvider(test.NewInMemAgent())
	lookup := New()

	kind := cluster.NewKind("test-kind", actor.PropsFromFunc(func(ctx actor.Context) {}))
	config := cluster.Configure("test-cluster", provider, lookup,
		remote.Configure("127.0.0.1", 0),
		cluster.WithKinds(kind),
	)
	c := cluster.NewCluster(system, config)
	c.InitKindsForTest(kind)

	manager := newPartitionManager(c)
	manager.Start()
	defer manager.Stop()

	// Activate a grain.
	ci := &cluster.ClusterIdentity{Kind: "test-kind", Identity: "cleanup-1"}
	req := &cluster.ActivationRequest{ClusterIdentity: ci}
	future := system.Root.RequestFuture(manager.placementActor, req, 2*time.Second)
	res, err := future.Result()
	require.NoError(t, err)
	resp := res.(*cluster.ActivationResponse)
	require.False(t, resp.Failed)
	require.NotNil(t, resp.Pid)

	// Verify the grain is tracked.
	listFuture := system.Root.RequestFuture(manager.placementActor, &cluster.ListGrainsRequest{}, 2*time.Second)
	listRes, err := listFuture.Result()
	require.NoError(t, err)
	listResp := listRes.(*cluster.ListGrainsResponse)
	require.Len(t, listResp.Grains, 1)

	// Stop the grain — this should trigger the Terminated handler,
	// which calls RemoveActivation and removes from the local map.
	system.Root.Poison(resp.Pid)
	time.Sleep(500 * time.Millisecond)

	// Verify the grain was removed from the placement actor's tracking.
	listFuture2 := system.Root.RequestFuture(manager.placementActor, &cluster.ListGrainsRequest{}, 2*time.Second)
	listRes2, err := listFuture2.Result()
	require.NoError(t, err)
	listResp2 := listRes2.(*cluster.ListGrainsResponse)
	assert.Empty(t, listResp2.Grains, "grain should be removed after termination")
}

func TestDistHashManager(t *testing.T) {
	suite.Run(t, new(DistHashManagerTestSuite))
}

// setupDisthashPeekTest creates a cluster, partition manager, and wires the
// lookup so that Peek() works. The lookup's partitionManager is set directly
// since we bypass Setup() in unit tests (same as existing manager tests).
func setupDisthashPeekTest(t *testing.T) (*IdentityLookup, *Manager, *actor.ActorSystem, *cluster.Cluster) {
	t.Helper()
	system := actor.NewActorSystem()
	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	kind := cluster.NewKind("TestKind", props)
	provider := test.NewTestProvider(test.NewInMemAgent())
	lookup := New().(*IdentityLookup)
	remoteCfg := remote.Configure("127.0.0.1", 0)
	config := cluster.Configure("test-cluster", provider, lookup, remoteCfg, cluster.WithKinds(kind))
	c := cluster.NewCluster(system, config)

	// Start remote so MemberList.UpdateClusterTopology can access the block list.
	c.Remote = remote.NewRemote(system, c.Config.RemoteConfig)
	err := c.Remote.Start()
	require.NoError(t, err)

	c.InitKindsForTest(kind)

	manager := newPartitionManager(c)
	manager.Start()

	// Wire the partition manager into the lookup so Peek() can access it.
	lookup.partitionManager = manager

	t.Cleanup(func() {
		manager.Stop()
		c.Remote.Shutdown(true)
	})
	return lookup, manager, system, c
}

func TestDisthash_Peek_ActiveGrain(t *testing.T) {
	lookup, manager, system, c := setupDisthashPeekTest(t)

	host, port, err := system.GetHostPort()
	require.NoError(t, err)
	self := &cluster.Member{Host: host, Port: int32(port), Id: system.ID, Kinds: []string{"TestKind"}}
	manager.onClusterTopology(&cluster.ClusterTopology{
		Members: cluster.Members{self},
	})
	c.MemberList.UpdateClusterTopology(cluster.Members{self})

	identity := &cluster.ClusterIdentity{Identity: "abc", Kind: "TestKind"}
	req := &cluster.ActivationRequest{ClusterIdentity: identity}
	future := system.Root.RequestFuture(manager.placementActor, req, 5*time.Second)
	res, err := future.Result()
	require.NoError(t, err)
	resp := res.(*cluster.ActivationResponse)
	require.False(t, resp.Failed)
	require.NotNil(t, resp.Pid)

	peekResult, err := lookup.Peek(identity)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusAlive, peekResult.Status)
	assert.Equal(t, "abc", peekResult.Identity)
	assert.Equal(t, "TestKind", peekResult.Kind)
	assert.Equal(t, resp.Pid, peekResult.PID)
	assert.Equal(t, system.ID, peekResult.MemberID)
}

func TestDisthash_Peek_NoActivation(t *testing.T) {
	lookup, manager, system, c := setupDisthashPeekTest(t)

	host, port, err := system.GetHostPort()
	require.NoError(t, err)
	self := &cluster.Member{Host: host, Port: int32(port), Id: system.ID, Kinds: []string{"TestKind"}}
	manager.onClusterTopology(&cluster.ClusterTopology{
		Members: cluster.Members{self},
	})
	c.MemberList.UpdateClusterTopology(cluster.Members{self})

	identity := &cluster.ClusterIdentity{Identity: "nonexistent", Kind: "TestKind"}
	peekResult, err := lookup.Peek(identity)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusNotFound, peekResult.Status)
}

func TestDisthash_Peek_AfterTermination(t *testing.T) {
	lookup, manager, system, c := setupDisthashPeekTest(t)

	host, port, err := system.GetHostPort()
	require.NoError(t, err)
	self := &cluster.Member{Host: host, Port: int32(port), Id: system.ID, Kinds: []string{"TestKind"}}
	manager.onClusterTopology(&cluster.ClusterTopology{
		Members: cluster.Members{self},
	})
	c.MemberList.UpdateClusterTopology(cluster.Members{self})

	identity := &cluster.ClusterIdentity{Identity: "will-die", Kind: "TestKind"}
	req := &cluster.ActivationRequest{ClusterIdentity: identity}
	future := system.Root.RequestFuture(manager.placementActor, req, 5*time.Second)
	res, err := future.Result()
	require.NoError(t, err)
	resp := res.(*cluster.ActivationResponse)
	require.False(t, resp.Failed)

	system.Root.Poison(resp.Pid)
	require.Eventually(t, func() bool {
		r, err := lookup.Peek(identity)
		return err == nil && r.Status == cluster.PeekStatusNotFound
	}, 5*time.Second, 50*time.Millisecond)
}

func TestDisthash_Peek_MemberDead(t *testing.T) {
	lookup, manager, _, c := setupDisthashPeekTest(t)

	deadMember := &cluster.Member{Host: "dead-host", Port: 9999, Id: "dead-member-id", Kinds: []string{"TestKind"}}
	manager.onClusterTopology(&cluster.ClusterTopology{
		Members: cluster.Members{deadMember},
	})
	// MemberList has NO members — the dead member is not in it.
	c.MemberList.UpdateClusterTopology(cluster.Members{})

	identity := &cluster.ClusterIdentity{Identity: "abc", Kind: "TestKind"}
	peekResult, err := lookup.Peek(identity)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusMemberDead, peekResult.Status)
	assert.Equal(t, "abc", peekResult.Identity)
	assert.Equal(t, "TestKind", peekResult.Kind)
}

func TestDisthash_Peek_NoMembers(t *testing.T) {
	lookup, _, _, _ := setupDisthashPeekTest(t)

	identity := &cluster.ClusterIdentity{Identity: "abc", Kind: "TestKind"}
	peekResult, err := lookup.Peek(identity)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusNotFound, peekResult.Status)
}
