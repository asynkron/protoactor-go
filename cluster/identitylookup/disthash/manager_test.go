package disthash

import (
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/test"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
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
	c := cluster.New(system, config)

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

		c := cluster.New(system, config)
		c.StartMember()
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

func TestDistHashManager(t *testing.T) {
	suite.Run(t, new(DistHashManagerTestSuite))
}
