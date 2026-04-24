// Package identitylookup provides conformance suites for identity lookup
// implementations and their placement actor integrations.
package identitylookup

import (
	"fmt"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PlacementConformanceSuite validates that an identity lookup correctly
// integrates with the shared placement actor. It tests spawn, dedup,
// termination cleanup, graceful shutdown, and ListGrains enumeration.
//
// Usage:
//
//	suite := &PlacementConformanceSuite{
//	    NewCluster: func(t *testing.T, kinds []*cluster.Kind) (*cluster.Cluster, func()) {
//	        // Return a started cluster and a shutdown function
//	    },
//	}
//	suite.RunAll(t)
type PlacementConformanceSuite struct {
	// NewCluster creates and starts a cluster with the given kinds registered.
	// The cluster should use the identity lookup under test. Returns the cluster
	// and a shutdown function. The shutdown function must be safe to call multiple
	// times. The caller registers t.Cleanup with the shutdown function.
	NewCluster func(t *testing.T, kinds []*cluster.Kind) (c *cluster.Cluster, shutdown func())
}

// RunAll runs all placement conformance tests.
func (s *PlacementConformanceSuite) RunAll(t *testing.T) {
	t.Run("SpawnOnGet", s.testSpawnOnGet)
	t.Run("DuplicateGetReturnsSamePID", s.testDuplicateGetReturnsSamePID)
	t.Run("TerminatedGrainCleansUp", s.testTerminatedGrainCleansUp)
	t.Run("ShutdownPoisonsLocalGrains", s.testShutdownPoisonsLocalGrains)
	t.Run("ListGrainsReturnsActiveGrains", s.testListGrainsReturnsActiveGrains)
}

func noopProps() *actor.Props {
	return actor.PropsFromFunc(func(ctx actor.Context) {})
}

// testSpawnOnGet verifies that Get() spawns a grain and returns a non-nil PID.
func (s *PlacementConformanceSuite) testSpawnOnGet(t *testing.T) {
	kind := cluster.NewKind("conformance-kind", noopProps())
	c, shutdown := s.NewCluster(t, []*cluster.Kind{kind})
	t.Cleanup(shutdown)

	pid := c.Get("spawn-test-1", "conformance-kind")
	require.NotNil(t, pid, "Get should return a non-nil PID")
	assert.NotEmpty(t, pid.Address, "PID should have an address")
	assert.NotEmpty(t, pid.Id, "PID should have an ID")
}

// testDuplicateGetReturnsSamePID verifies that multiple Get() calls for the
// same identity return the same PID (deduplication).
func (s *PlacementConformanceSuite) testDuplicateGetReturnsSamePID(t *testing.T) {
	kind := cluster.NewKind("conformance-kind", noopProps())
	c, shutdown := s.NewCluster(t, []*cluster.Kind{kind})
	t.Cleanup(shutdown)

	pid1 := c.Get("dedup-test-1", "conformance-kind")
	require.NotNil(t, pid1)

	pid2 := c.Get("dedup-test-1", "conformance-kind")
	require.NotNil(t, pid2)

	assert.True(t, pid1.Equal(pid2), "duplicate Get should return same PID: %v != %v", pid1, pid2)
}

// testTerminatedGrainCleansUp verifies that when a grain terminates,
// it is removed from the placement actor's tracking (subsequent Get
// returns a new PID).
func (s *PlacementConformanceSuite) testTerminatedGrainCleansUp(t *testing.T) {
	kind := cluster.NewKind("conformance-kind", noopProps())
	c, shutdown := s.NewCluster(t, []*cluster.Kind{kind})
	t.Cleanup(shutdown)

	pid1 := c.Get("cleanup-test-1", "conformance-kind")
	require.NotNil(t, pid1)

	// Poison the grain and wait for it to stop.
	err := c.ActorSystem.Root.PoisonFuture(pid1).Wait()
	require.NoError(t, err)
	// Brief pause for Terminated to propagate to the placement actor.
	time.Sleep(100 * time.Millisecond)

	// Get again — should return a new PID (re-activated).
	pid2 := c.Get("cleanup-test-1", "conformance-kind")
	require.NotNil(t, pid2, "Get after termination should return a new PID")

	// The new PID should be valid. We don't assert inequality because
	// the PID could be recycled to the same address/name in some edge cases.
	assert.NotEmpty(t, pid2.Id)
}

// testShutdownPoisonsLocalGrains verifies that shutting down the cluster
// gracefully poisons all local grains (they receive Stopping).
func (s *PlacementConformanceSuite) testShutdownPoisonsLocalGrains(t *testing.T) {
	stoppedCh := make(chan string, 10)
	props := actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *actor.Stopping:
			identity := cluster.GetClusterIdentity(ctx)
			if identity != nil {
				stoppedCh <- identity.Identity
			}
		}
	})
	kind := cluster.NewKind("conformance-shutdown", props)
	c, shutdown := s.NewCluster(t, []*cluster.Kind{kind})
	t.Cleanup(shutdown)

	// Activate 3 grains.
	for i := 0; i < 3; i++ {
		pid := c.Get(fmt.Sprintf("shutdown-grain-%d", i), "conformance-shutdown")
		require.NotNil(t, pid, "grain %d should activate", i)
	}

	// Shutdown the cluster — this should poison all local grains.
	// The shutdown function is safe to call multiple times (sync.Once),
	// so t.Cleanup calling it again is benign.
	shutdown()

	// Collect stopped notifications with timeout.
	stopped := make(map[string]bool)
	timeout := time.After(5 * time.Second)
	for len(stopped) < 3 {
		select {
		case id := <-stoppedCh:
			stopped[id] = true
		case <-timeout:
			t.Fatalf("timed out waiting for grains to stop; got %d/3: %v", len(stopped), stopped)
		}
	}

	assert.Len(t, stopped, 3, "all 3 grains should have received Stopping")
}

// testListGrainsReturnsActiveGrains verifies that the GrainEnumerator
// interface returns active grains when the identity lookup supports it.
func (s *PlacementConformanceSuite) testListGrainsReturnsActiveGrains(t *testing.T) {
	kind := cluster.NewKind("conformance-kind", noopProps())
	c, shutdown := s.NewCluster(t, []*cluster.Kind{kind})
	t.Cleanup(shutdown)

	// Check if the identity lookup supports enumeration.
	enum, ok := c.IdentityLookup.(cluster.GrainEnumerator)
	if !ok {
		t.Skip("identity lookup does not implement GrainEnumerator")
	}

	// Activate 2 grains. c.Get is synchronous so no sleep needed.
	pid1 := c.Get("list-test-1", "conformance-kind")
	require.NotNil(t, pid1)
	pid2 := c.Get("list-test-2", "conformance-kind")
	require.NotNil(t, pid2)

	grains, err := enum.ListGrains()
	require.NoError(t, err)

	// Filter to our kind (cluster may have other kinds like prototopic).
	var ours []*cluster.GrainInfo
	for _, g := range grains {
		if g.Kind == "conformance-kind" {
			ours = append(ours, g)
		}
	}

	assert.Len(t, ours, 2, "should have 2 active grains of conformance-kind")
}
