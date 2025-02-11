package disthash

import (
	"sync"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/test"
	"github.com/stretchr/testify/assert"
)

// TestManagerConcurrentAccess verifies that concurrent access to the Manager
// doesn't trigger race conditions
func TestManagerConcurrentAccess(t *testing.T) {
	system := actor.NewActorSystem()
	provider := test.NewTestProvider(test.NewInMemAgent())
	config := cluster.Configure("test-cluster", provider, disthash.New())
	c := cluster.New(system, config)

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

// Integration test using ClusterFixture
type DistHashManagerTestSuite struct {
	suite.Suite
	fixture *cluster_test_tool.BaseClusterFixture
}

func (suite *DistHashManagerTestSuite) SetupTest() {
	suite.fixture = cluster_test_tool.NewBaseInMemoryClusterFixture(3)
	suite.fixture.Initialize()
}

func (suite *DistHashManagerTestSuite) TearDownTest() {
	suite.fixture.ShutDown()
}

func (suite *DistHashManagerTestSuite) TestConcurrentClusterOperations() {
	// Get the clusters
	clusters := suite.fixture.GetMembers()
	assert.Equal(suite.T(), 3, len(clusters))

	// Create multiple concurrent operations
	var wg sync.WaitGroup
	iterations := 100

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()

			// Randomly select a cluster
			cluster := clusters[iteration%len(clusters)]

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

func TestDistHashManager(t *testing.T) {
	suite.Run(t, new(DistHashManagerTestSuite))
}
