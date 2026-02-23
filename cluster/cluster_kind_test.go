package cluster

import (
	"sync"
	"testing"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

func newTestCluster() *Cluster {
	system := actor.NewActorSystem()
	cfg := &Config{
		Name:               "test-cluster",
		ClusterContextProducer: newDefaultClusterContext,
		PubSubConfig:       newPubSubConfig(),
	}
	c := &Cluster{
		ActorSystem: system,
		Config:      cfg,
		kinds:       map[string]*ActivatedKind{},
	}
	return c
}

func TestCluster_GetClusterKinds_ThreadSafe(t *testing.T) {
	c := newTestCluster()
	c.kinds["existing"] = &ActivatedKind{Kind: "existing"}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			kinds := c.GetClusterKinds()
			assert.NotNil(t, kinds)
		}()
	}
	wg.Wait()
}

func TestCluster_GetClusterKind_ThreadSafe(t *testing.T) {
	c := newTestCluster()
	c.kinds["testKind"] = &ActivatedKind{Kind: "testKind"}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			k := c.GetClusterKind("testKind")
			assert.NotNil(t, k)
		}()
	}
	wg.Wait()
}

func TestCluster_TryGetClusterKind_ThreadSafe(t *testing.T) {
	c := newTestCluster()
	c.kinds["testKind"] = &ActivatedKind{Kind: "testKind"}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			k, ok := c.TryGetClusterKind("testKind")
			assert.True(t, ok)
			assert.NotNil(t, k)
		}()
	}
	wg.Wait()
}
