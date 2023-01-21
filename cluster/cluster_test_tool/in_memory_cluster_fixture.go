package cluster_test_tool

import (
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/test"
)

// NewBaseInMemoryClusterFixture creates a new in memory cluster fixture
func NewBaseInMemoryClusterFixture(clusterSize int, opts ...ClusterFixtureOption) *BaseClusterFixture {
	inMemAgent := test.NewInMemAgent()
	baseInMemoryOpts := []ClusterFixtureOption{
		WithGetClusterProvider(func() cluster.ClusterProvider {
			return test.NewTestProvider(inMemAgent)
		}),
	}
	baseInMemoryOpts = append(baseInMemoryOpts, opts...)

	return NewBaseClusterFixture(clusterSize, baseInMemoryOpts...)
}
