package disthash

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/cluster/clusterproviders/test"
	"github.com/awevoke/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// safeShutdown wraps Cluster.Shutdown in a sync.Once to prevent panics
// from double-shutdown (e.g., explicit shutdown + t.Cleanup).
type safeShutdown struct {
	once sync.Once
	c    *cluster.Cluster
}

func (s *safeShutdown) Shutdown() {
	s.once.Do(func() {
		s.c.Shutdown(true)
	})
}

// newTwoNodeCluster creates two cluster nodes sharing an InMemAgent for
// topology discovery. Both nodes support the given kinds. Returns the
// two clusters and safeShutdown wrappers. Cleanup is registered via t.Cleanup.
func newTwoNodeCluster(t *testing.T, kinds []*cluster.Kind) (*cluster.Cluster, *cluster.Cluster, *safeShutdown, *safeShutdown) {
	t.Helper()
	agent := test.NewInMemAgent()

	makeNode := func(name string) (*cluster.Cluster, *safeShutdown) {
		system := actor.NewActorSystem()
		provider := test.NewTestProvider(agent)
		lookup := New()
		opts := []cluster.ConfigOption{cluster.WithKinds(kinds...)}
		config := cluster.Configure(name, provider, lookup,
			remote.Configure("127.0.0.1", 0), opts...)
		c := cluster.NewCluster(system, config)
		err := c.StartMember()
		require.NoError(t, err)
		ss := &safeShutdown{c: c}
		return c, ss
	}

	c1, ss1 := makeNode("two-node-test")
	c2, ss2 := makeNode("two-node-test")

	// Wait for both nodes to see each other in the topology.
	require.Eventually(t, func() bool {
		members := c1.MemberList.Members()
		if members == nil {
			return false
		}
		return len(members.Members()) >= 2
	}, 10*time.Second, 200*time.Millisecond, "node 1 should see 2 members")

	require.Eventually(t, func() bool {
		members := c2.MemberList.Members()
		if members == nil {
			return false
		}
		return len(members.Members()) >= 2
	}, 10*time.Second, 200*time.Millisecond, "node 2 should see 2 members")

	t.Cleanup(func() {
		ss2.Shutdown()
		ss1.Shutdown()
	})

	return c1, c2, ss1, ss2
}

// TestTwoNode_ShutdownOnlyPoisonsLocalGrains verifies that shutting down
// one node does not affect grains on the other node.
func TestTwoNode_ShutdownOnlyPoisonsLocalGrains(t *testing.T) {
	kind := cluster.NewKind("test-kind", actor.PropsFromFunc(func(ctx actor.Context) {}))
	c1, c2, ss1, _ := newTwoNodeCluster(t, []*cluster.Kind{kind})

	// Activate grains via node 1. Disthash routes to the owner
	// determined by rendezvous hash, so grains may land on either node.
	var pids []*actor.PID
	for i := 0; i < 10; i++ {
		pid := c1.Get(fmt.Sprintf("grain-%d", i), "test-kind")
		if pid != nil {
			pids = append(pids, pid)
		}
	}
	require.NotEmpty(t, pids, "should have activated at least some grains")

	// Record which grains are on node 2 (by querying node 2's placement actor).
	// Review fix #1: use require.True instead of t.Skip — disthash implements GrainEnumerator.
	enum2, ok := c2.IdentityLookup.(cluster.GrainEnumerator)
	require.True(t, ok, "disthash identity lookup must implement GrainEnumerator")

	node2Grains, err := enum2.ListGrains()
	require.NoError(t, err)

	// Count grains owned by node 2 (its placement actor).
	var node2Count int
	for _, g := range node2Grains {
		if g.Kind == "test-kind" {
			node2Count++
		}
	}

	// Shutdown node 1 — this should NOT affect node 2's grains.
	// Use safeShutdown so t.Cleanup won't panic on double-shutdown.
	ss1.Shutdown()
	time.Sleep(1 * time.Second)

	// Verify node 2's grains are still alive.
	node2GrainsAfter, err := enum2.ListGrains()
	// If node 2 is still running, this should succeed.
	// Note: node 2 may have lost grains due to topology rebalance
	// (disthash rebalances when a member leaves), so we just verify
	// the grains that should stay on node 2 are still there.
	if err == nil {
		var node2CountAfter int
		for _, g := range node2GrainsAfter {
			if g.Kind == "test-kind" {
				node2CountAfter++
			}
		}
		// After node 1 leaves, node 2 is the only member.
		// All grains' rendezvous owner becomes node 2.
		// Grains that were on node 1 are gone (node 1 shut down).
		// Grains that were on node 2 stay.
		// New activations for the lost grains would go to node 2.
		// So node 2 should have AT LEAST the grains it had before.
		assert.GreaterOrEqual(t, node2CountAfter, node2Count,
			"node 2 grains should not decrease after node 1 shutdown")
	}
}

// TestTwoNode_RebalanceOnlyAffectsLocalGrains verifies that when a node
// joins, only the local placement actor poisons its own affected grains
// (it does not touch remote grains).
func TestTwoNode_RebalanceOnlyAffectsLocalGrains(t *testing.T) {
	kind := cluster.NewKind("test-kind", actor.PropsFromFunc(func(ctx actor.Context) {}))

	// Start with only node 1.
	agent := test.NewInMemAgent()
	system1 := actor.NewActorSystem()
	provider1 := test.NewTestProvider(agent)
	lookup1 := New()
	config1 := cluster.Configure("rebalance-test", provider1, lookup1,
		remote.Configure("127.0.0.1", 0),
		cluster.WithKinds(kind),
	)
	c1 := cluster.NewCluster(system1, config1)
	err := c1.StartMember()
	require.NoError(t, err)
	ss1 := &safeShutdown{c: c1}
	t.Cleanup(func() { ss1.Shutdown() })

	// Activate 10 grains — all on node 1 (only member).
	for i := 0; i < 10; i++ {
		pid := c1.Get(fmt.Sprintf("rebal-grain-%d", i), "test-kind")
		require.NotNil(t, pid, "grain %d should activate", i)
	}

	// Review fix #1: use require.True instead of t.Skip — disthash implements GrainEnumerator.
	enum1, ok := c1.IdentityLookup.(cluster.GrainEnumerator)
	require.True(t, ok, "disthash identity lookup must implement GrainEnumerator")

	grainsBefore, err := enum1.ListGrains()
	require.NoError(t, err)

	var countBefore int
	for _, g := range grainsBefore {
		if g.Kind == "test-kind" {
			countBefore++
		}
	}
	require.Equal(t, 10, countBefore, "all 10 grains should be on node 1")

	// Now add node 2 — this triggers rebalancing on node 1.
	system2 := actor.NewActorSystem()
	provider2 := test.NewTestProvider(agent)
	lookup2 := New()
	config2 := cluster.Configure("rebalance-test", provider2, lookup2,
		remote.Configure("127.0.0.1", 0),
		cluster.WithKinds(kind),
	)
	c2 := cluster.NewCluster(system2, config2)
	err = c2.StartMember()
	require.NoError(t, err)
	ss2 := &safeShutdown{c: c2}
	t.Cleanup(func() { ss2.Shutdown() })

	// Wait for topology to converge.
	require.Eventually(t, func() bool {
		members := c1.MemberList.Members()
		if members == nil {
			return false
		}
		return len(members.Members()) >= 2
	}, 10*time.Second, 200*time.Millisecond)

	// Review fix #2: use require.Eventually for rebalance verification
	// instead of time.Sleep(2s). Poll ListGrains until count drops below
	// countBefore, with a 15s timeout and 200ms poll interval.
	require.Eventually(t, func() bool {
		grainsAfter, err := enum1.ListGrains()
		if err != nil {
			return false
		}
		var countAfter int
		for _, g := range grainsAfter {
			if g.Kind == "test-kind" {
				countAfter++
			}
		}
		return countAfter < countBefore
	}, 15*time.Second, 200*time.Millisecond,
		"node 1 should have fewer grains after rebalance (had %d)", countBefore)
}
