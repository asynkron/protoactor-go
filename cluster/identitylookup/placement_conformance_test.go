package identitylookup_test

import (
	"sync"
	"testing"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/cluster/clusterproviders/test"
	"github.com/awevoke/protoactor-go/cluster/identitylookup"
	"github.com/awevoke/protoactor-go/cluster/identitylookup/disthash"
	"github.com/awevoke/protoactor-go/cluster/identitylookup/storage"
	"github.com/awevoke/protoactor-go/remote"
	"github.com/stretchr/testify/require"
)

// TestDisthashPlacementConformance runs the placement conformance suite
// against the disthash identity lookup.
func TestDisthashPlacementConformance(t *testing.T) {
	suite := &identitylookup.PlacementConformanceSuite{
		NewCluster: func(t *testing.T, kinds []*cluster.Kind) (*cluster.Cluster, func()) {
			t.Helper()
			system := actor.NewActorSystem()
			agent := test.NewInMemAgent()
			provider := test.NewTestProvider(agent)
			lookup := disthash.New()
			opts := []cluster.ConfigOption{cluster.WithKinds(kinds...)}
			config := cluster.Configure("conformance-disthash", provider, lookup,
				remote.Configure("127.0.0.1", 0), opts...)
			c := cluster.NewCluster(system, config)
			err := c.StartMember()
			require.NoError(t, err)
			var once sync.Once
			shutdown := func() { once.Do(func() { c.Shutdown(true) }) }
			return c, shutdown
		},
	}
	suite.RunAll(t)
}

// TestStoragePlacementConformance runs the placement conformance suite
// against the IdentityStorageLookup with InMemoryStorageLookup backend.
func TestStoragePlacementConformance(t *testing.T) {
	suite := &identitylookup.PlacementConformanceSuite{
		NewCluster: func(t *testing.T, kinds []*cluster.Kind) (*cluster.Cluster, func()) {
			t.Helper()
			system := actor.NewActorSystem()
			agent := test.NewInMemAgent()
			provider := test.NewTestProvider(agent)
			mem := identitylookup.NewInMemoryStorageLookup()
			lookup := storage.New(mem)
			opts := []cluster.ConfigOption{cluster.WithKinds(kinds...)}
			config := cluster.Configure("conformance-storage", provider, lookup,
				remote.Configure("127.0.0.1", 0), opts...)
			c := cluster.NewCluster(system, config)
			err := c.StartMember()
			require.NoError(t, err)
			var once sync.Once
			shutdown := func() { once.Do(func() { c.Shutdown(true) }) }
			return c, shutdown
		},
	}
	suite.RunAll(t)
}
