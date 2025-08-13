package cluster

import (
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

// Test that a virtual actor (cluster kind) can access its ClusterIdentity via the
// context extension and also obtain the Cluster instance.
func TestVirtualActorContextHasClusterIdentity(t *testing.T) {
	cp := newInmemoryProvider()
	identityCh := make(chan *ClusterIdentity, 1)
	clusterCh := make(chan *Cluster, 1)

	kindName := "kind"
	actorID := "myactor"

	// Actor stores the ClusterIdentity and Cluster when it receives the
	// system generated ClusterInit message.
	kind := NewKind(kindName, actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *ClusterInit:
			identityCh <- GetClusterIdentity(ctx)
			clusterCh <- GetCluster(ctx.ActorSystem())
		}
	}))

	c := newClusterForTest("mycluster", cp, WithKinds(kind))
	c.StartMember()
	cp.publishClusterTopologyEvent()

	pid := c.Get(actorID, kindName)
	assert.NotNil(t, pid)

	select {
	case ci := <-identityCh:
		assert.NotNil(t, ci)
		assert.Equal(t, actorID, ci.Identity)
		assert.Equal(t, kindName, ci.Kind)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for ClusterIdentity")
	}

	select {
	case cl := <-clusterCh:
		assert.NotNil(t, cl)
		assert.Equal(t, c, cl)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for Cluster instance")
	}
}
