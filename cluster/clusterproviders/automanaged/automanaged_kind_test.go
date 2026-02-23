package automanaged

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
)

// Compile-time check that AutoManagedProvider implements KindUpdater.
var _ cluster.KindUpdater = (*AutoManagedProvider)(nil)

func TestAutoManagedProvider_UpdateKinds(t *testing.T) {
	p := NewWithConfig(2*time.Second, 6330, "localhost:6330")
	p.knownKinds = []string{"kindA"}

	err := p.UpdateKinds([]string{"kindA", "kindB"})
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"kindA", "kindB"}, p.knownKinds)
}

func TestAutoManagedProvider_UpdateKinds_ReflectedInGetCurrentNode(t *testing.T) {
	p := NewWithConfig(2*time.Second, 6330, "localhost:6330")
	p.clusterName = "test-cluster"
	p.cluster = &cluster.Cluster{
		ActorSystem: actor.NewActorSystem(),
	}
	p.knownKinds = []string{"kindA"}

	err := p.UpdateKinds([]string{"kindA", "kindB"})
	assert.NoError(t, err)

	node := p.getCurrentNode()
	assert.ElementsMatch(t, []string{"kindA", "kindB"}, node.Kinds)
}

func TestAutoManagedProvider_UpdateKinds_Empty(t *testing.T) {
	p := NewWithConfig(2*time.Second, 6330, "localhost:6330")
	p.knownKinds = []string{"kindA", "kindB"}

	err := p.UpdateKinds([]string{})
	assert.NoError(t, err)
	assert.Empty(t, p.knownKinds)
}

func TestAutoManagedProvider_UpdateKinds_Replace(t *testing.T) {
	p := NewWithConfig(2*time.Second, 6330, "localhost:6330")
	p.knownKinds = []string{"kindA"}

	err := p.UpdateKinds([]string{"kindB", "kindC"})
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"kindB", "kindC"}, p.knownKinds)
}
