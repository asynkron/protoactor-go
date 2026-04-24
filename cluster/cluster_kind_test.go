package cluster

import (
	"fmt"
	"sync"
	"testing"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

type testActor struct{}

func (t *testActor) Receive(_ actor.Context) {}

func newTestCluster() *Cluster {
	system := actor.NewActorSystem()
	cfg := &Config{
		Name:                   "test-cluster",
		ClusterContextProducer: newDefaultClusterContext,
		PubSubConfig:           newPubSubConfig(),
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

func TestCluster_RegisterKind_Success(t *testing.T) {
	c := newTestCluster()

	kind := NewKind("newKind", actor.PropsFromProducer(func() actor.Actor {
		return &testActor{}
	}))

	err := c.RegisterKind(kind)
	assert.NoError(t, err)

	ak, ok := c.TryGetClusterKind("newKind")
	assert.True(t, ok)
	assert.NotNil(t, ak)
	assert.Equal(t, "newKind", ak.Kind)
}

func TestCluster_RegisterKind_Duplicate(t *testing.T) {
	c := newTestCluster()
	kind := NewKind("dup", actor.PropsFromProducer(func() actor.Actor {
		return &testActor{}
	}))

	err := c.RegisterKind(kind)
	assert.NoError(t, err)

	err = c.RegisterKind(kind)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestCluster_DeregisterKind_Success(t *testing.T) {
	c := newTestCluster()
	kind := NewKind("removable", actor.PropsFromProducer(func() actor.Actor {
		return &testActor{}
	}))

	err := c.RegisterKind(kind)
	assert.NoError(t, err)

	err = c.DeregisterKind("removable")
	assert.NoError(t, err)

	_, ok := c.TryGetClusterKind("removable")
	assert.False(t, ok)
}

func TestCluster_DeregisterKind_Reserved(t *testing.T) {
	c := newTestCluster()

	err := c.DeregisterKind(TopicActorKind)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reserved")
}

func TestCluster_DeregisterKind_NotFound(t *testing.T) {
	c := newTestCluster()

	err := c.DeregisterKind("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")
}

func TestCluster_RegisterKind_NotifiesProvider(t *testing.T) {
	c := newTestCluster()

	var calledWithKinds []string
	c.provider = &mockKindUpdaterProvider{
		updateKinds: func(kinds []string) error {
			calledWithKinds = kinds
			return nil
		},
	}

	kind := NewKind("notified", actor.PropsFromProducer(func() actor.Actor {
		return &testActor{}
	}))

	err := c.RegisterKind(kind)
	assert.NoError(t, err)
	assert.Contains(t, calledWithKinds, "notified")
}

func TestCluster_RegisterKind_ConcurrentAccess(t *testing.T) {
	c := newTestCluster()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			kind := NewKind(
				fmt.Sprintf("kind-%d", i),
				actor.PropsFromProducer(func() actor.Actor {
					return &testActor{}
				}),
			)
			_ = c.RegisterKind(kind)
		}()
	}

	// Concurrent reads while writing
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.GetClusterKinds()
		}()
	}
	wg.Wait()

	kinds := c.GetClusterKinds()
	assert.Len(t, kinds, 50)
}

// Mock provider that implements both ClusterProvider and KindUpdater
type mockKindUpdaterProvider struct {
	updateKinds func(kinds []string) error
}

func (m *mockKindUpdaterProvider) StartMember(_ *Cluster) error { return nil }
func (m *mockKindUpdaterProvider) StartClient(_ *Cluster) error { return nil }
func (m *mockKindUpdaterProvider) Shutdown(_ bool) error        { return nil }
func (m *mockKindUpdaterProvider) UpdateKinds(kinds []string) error {
	if m.updateKinds != nil {
		return m.updateKinds(kinds)
	}
	return nil
}
