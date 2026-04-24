package cluster

import (
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

// Test that a virtual actor (cluster kind) can access its ClusterIdentity via the
// context extension and also obtain the Cluster instance.
func TestVirtualActorContextHasClusterIdentity(t *testing.T) {
	cp := newInmemoryProvider()

	kindName := "kind"
	actorID := "myactor"

	resultCh := make(chan any, 10)

	kind := NewKind(kindName, actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *ClusterInit:
			resultCh <- GetClusterIdentity(ctx)
			resultCh <- GetCluster(ctx.ActorSystem())
		}
	}))

	c := newClusterForTest("mycluster", cp, WithKinds(kind))
	err := c.StartMember()
	assert.NoError(t, err)
	cp.publishClusterTopologyEvent()

	pid := c.Get(actorID, kindName)
	assert.NotNil(t, pid)

	select {
	case val := <-resultCh:
		ci, ok := val.(*ClusterIdentity)
		assert.True(t, ok)
		assert.Equal(t, actorID, ci.Identity)
		assert.Equal(t, kindName, ci.Kind)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for ClusterIdentity")
	}

	select {
	case val := <-resultCh:
		cl, ok := val.(*Cluster)
		assert.True(t, ok)
		assert.Equal(t, c, cl)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Cluster")
	}
}
