package cluster

import (
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/testkit"
	"github.com/stretchr/testify/assert"
)

// Test that a virtual actor (cluster kind) can access its ClusterIdentity via the
// context extension and also obtain the Cluster instance.
func TestVirtualActorContextHasClusterIdentity(t *testing.T) {
	cp := newInmemoryProvider()
	probe := testkit.NewTestProbe()

	kindName := "kind"
	actorID := "myactor"

	// Actor stores the ClusterIdentity and Cluster when it receives the
	// system generated ClusterInit message.
	kind := NewKind(kindName, actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *ClusterInit:
			probe.Context().Send(probe.Context().Self(), GetClusterIdentity(ctx))
			probe.Context().Send(probe.Context().Self(), GetCluster(ctx.ActorSystem()))
		}
	}))

	c := newClusterForTest("mycluster", cp, WithKinds(kind))
	c.StartMember()
	cp.publishClusterTopologyEvent()

	c.ActorSystem.Root.Spawn(actor.PropsFromProducer(func() actor.Actor { return probe }))

	pid := c.Get(actorID, kindName)
	assert.NotNil(t, pid)

	ci, err := testkit.GetNextMessageOf[*ClusterIdentity](probe, time.Second)
	assert.NoError(t, err)
	assert.NotNil(t, ci)
	assert.Equal(t, actorID, ci.Identity)
	assert.Equal(t, kindName, ci.Kind)

	cl, err := testkit.GetNextMessageOf[*Cluster](probe, time.Second)
	assert.NoError(t, err)
	assert.NotNil(t, cl)
	assert.Equal(t, c, cl)
}
