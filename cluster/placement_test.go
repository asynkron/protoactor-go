package cluster

import (
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// echoProps returns Props for a simple actor that responds to any message
// with the message itself. Used for testing the placement actor.
func echoProps() *actor.Props {
	return actor.PropsFromFunc(func(ctx actor.Context) {
		switch msg := ctx.Message().(type) {
		case *actor.Started, *actor.Stopping, *actor.Stopped:
			// lifecycle messages — ignore
		default:
			ctx.Respond(msg)
		}
	})
}

// slowStopProps returns Props for an actor that sleeps for the given
// duration in its Stopping handler. Used to test shutdown timeouts.
func slowStopProps(d time.Duration) *actor.Props {
	return actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *actor.Stopping:
			time.Sleep(d)
		}
	})
}

// crashOnStartProps returns Props for an actor that stops itself
// immediately after starting.
func crashOnStartProps() *actor.Props {
	return actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *actor.Started:
			ctx.Poison(ctx.Self())
		}
	})
}

// newTestClusterWithKind creates a Cluster suitable for placement actor
// tests. The cluster is started as a member. Returns the cluster and a
// cleanup function.
func newTestClusterWithKind(t *testing.T, kindName string, props *actor.Props) *Cluster {
	t.Helper()
	kind := NewKind(kindName, props)
	cp := newInmemoryProvider()
	c := newClusterForTest("placement-test", cp, WithKinds(kind))
	err := c.StartMember()
	require.NoError(t, err)
	cp.publishClusterTopologyEvent()
	t.Cleanup(func() { c.Shutdown(true) })
	return c
}

func TestPlacementActor_SpawnOnActivationRequest(t *testing.T) {
	c := newTestClusterWithKind(t, "testKind", echoProps())

	cfg := PlacementConfig{}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(placementPID)

	req := &ActivationRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "testKind", Identity: "actor1"},
		RequestId:       "req-1",
	}

	future := c.ActorSystem.Root.RequestFuture(placementPID, req, 5*time.Second)
	res, err := future.Result()
	require.NoError(t, err)

	resp, ok := res.(*ActivationResponse)
	require.True(t, ok, "expected *ActivationResponse, got %T", res)
	assert.False(t, resp.Failed, "activation should not fail")
	assert.NotNil(t, resp.Pid, "activation should return a PID")
}
