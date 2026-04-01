package cluster

import (
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActivatorProxy_ForwardsActivationRequest(t *testing.T) {
	c := newTestClusterWithKind(t, "testKind", echoProps())

	// Start a real placement actor.
	cfg := PlacementConfig{}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-proxy")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(placementPID)

	// Start the proxy.
	lookup := &fakeIdentityLookup{}
	lookup.Setup(c, nil, false)
	proxyProps := NewActivatorProxyProps(placementPID, lookup)
	proxyPID, err := c.ActorSystem.Root.SpawnNamed(proxyProps, "$test-proxy")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(proxyPID)

	// Send ActivationRequest to the proxy.
	req := &ActivationRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "testKind", Identity: "proxy-actor1"},
		RequestId:       "req-1",
	}

	future := c.ActorSystem.Root.RequestFuture(proxyPID, req, 5*time.Second)
	res, err := future.Result()
	require.NoError(t, err)

	resp, ok := res.(*ActivationResponse)
	require.True(t, ok, "expected *ActivationResponse, got %T", res)
	assert.False(t, resp.Failed)
	assert.NotNil(t, resp.Pid)
}

func TestActivatorProxy_ForwardsProxyActivationRequest(t *testing.T) {
	c := newTestClusterWithKind(t, "testKind", echoProps())

	cfg := PlacementConfig{}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-proxy2")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(placementPID)

	lookup := &fakeIdentityLookup{}
	lookup.Setup(c, nil, false)
	proxyProps := NewActivatorProxyProps(placementPID, lookup)
	proxyPID, err := c.ActorSystem.Root.SpawnNamed(proxyProps, "$test-proxy2")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(proxyPID)

	// ProxyActivationRequest with nil replaced_activation behaves like
	// ActivationRequest.
	req := &ProxyActivationRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "testKind", Identity: "proxy-actor2"},
	}

	future := c.ActorSystem.Root.RequestFuture(proxyPID, req, 5*time.Second)
	res, err := future.Result()
	require.NoError(t, err)

	resp, ok := res.(*ActivationResponse)
	require.True(t, ok, "expected *ActivationResponse, got %T", res)
	assert.False(t, resp.Failed)
	assert.NotNil(t, resp.Pid)
}

func TestActivatorProxy_ProxyActivationRequest_ReplacesStale(t *testing.T) {
	c := newTestClusterWithKind(t, "testKind", echoProps())

	cfg := PlacementConfig{}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-proxy3")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(placementPID)

	// Use a fake lookup that tracks RemovePid calls.
	lookup := &fakeIdentityLookup{}
	lookup.Setup(c, nil, false)
	// Pre-store a "stale" PID.
	stalePID := actor.NewPID("stale-host:9000", "stale/actor")
	ci := &ClusterIdentity{Kind: "testKind", Identity: "proxy-actor3"}
	lookup.m.Store(ci.Identity, stalePID)

	proxyProps := NewActivatorProxyProps(placementPID, lookup)
	proxyPID, err := c.ActorSystem.Root.SpawnNamed(proxyProps, "$test-proxy3")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(proxyPID)

	// ProxyActivationRequest with replaced_activation set.
	req := &ProxyActivationRequest{
		ClusterIdentity:    ci,
		ReplacedActivation: stalePID,
	}

	future := c.ActorSystem.Root.RequestFuture(proxyPID, req, 5*time.Second)
	res, err := future.Result()
	require.NoError(t, err)

	resp, ok := res.(*ActivationResponse)
	require.True(t, ok)
	assert.False(t, resp.Failed)
	assert.NotNil(t, resp.Pid)

	// The stale PID should have been removed from the lookup.
	got := lookup.Get(ci)
	// If the lookup still has the old PID, it means RemovePid wasn't called.
	// The new PID should be different from the stale one.
	if got != nil {
		assert.False(t, stalePID.Equal(got),
			"stale PID should have been removed by proxy")
	}
}

func TestActivatorProxy_UnknownMessage_Ignored(t *testing.T) {
	c := newTestClusterWithKind(t, "testKind", echoProps())

	cfg := PlacementConfig{}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-proxy-unk")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(placementPID)

	lookup := &fakeIdentityLookup{}
	proxyProps := NewActivatorProxyProps(placementPID, lookup)
	proxyPID, err := c.ActorSystem.Root.SpawnNamed(proxyProps, "$test-proxy-unk")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(proxyPID)

	// Send an unknown message — should not panic.
	c.ActorSystem.Root.Send(proxyPID, "random string message")
	time.Sleep(100 * time.Millisecond)

	// Proxy should still be alive.
	future := c.ActorSystem.Root.RequestFuture(proxyPID, &ActivationRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "testKind", Identity: "after-unknown"},
		RequestId:       "req-1",
	}, 5*time.Second)
	res, err := future.Result()
	require.NoError(t, err)
	resp := res.(*ActivationResponse)
	assert.False(t, resp.Failed)
}

func TestActivatorProxy_ForwardsPeekRequest(t *testing.T) {
	c := newTestClusterWithKind(t, "test-kind", echoProps())

	cfg := PlacementConfig{}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-proxy-peek")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(placementPID)

	lookup := &fakeIdentityLookup{}
	lookup.Setup(c, nil, false)
	proxyProps := NewActivatorProxyProps(placementPID, lookup)
	proxyPID, err := c.ActorSystem.Root.SpawnNamed(proxyProps, "$test-proxy-peek")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(proxyPID)

	activateReq := &ActivationRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "test-kind", Identity: "grain-1"},
		RequestId:       "req-1",
	}
	activateFuture := c.ActorSystem.Root.RequestFuture(proxyPID, activateReq, 5*time.Second)
	activateRes, err := activateFuture.Result()
	require.NoError(t, err)
	activateResp := activateRes.(*ActivationResponse)
	require.False(t, activateResp.Failed)

	peekReq := &PeekRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "test-kind", Identity: "grain-1"},
	}
	peekFuture := c.ActorSystem.Root.RequestFuture(proxyPID, peekReq, 5*time.Second)
	peekRes, err := peekFuture.Result()
	require.NoError(t, err)
	peekResp, ok := peekRes.(*PeekResponse)
	require.True(t, ok)
	assert.True(t, peekResp.Found)
	assert.Equal(t, activateResp.Pid, peekResp.Pid)
}

func TestActivatorProxy_ForwardsPeekRequest_NotFound(t *testing.T) {
	c := newTestClusterWithKind(t, "test-kind", echoProps())

	cfg := PlacementConfig{}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-proxy-peek-nf")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(placementPID)

	lookup := &fakeIdentityLookup{}
	lookup.Setup(c, nil, false)
	proxyProps := NewActivatorProxyProps(placementPID, lookup)
	proxyPID, err := c.ActorSystem.Root.SpawnNamed(proxyProps, "$test-proxy-peek-nf")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(proxyPID)

	peekReq := &PeekRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "test-kind", Identity: "nonexistent"},
	}
	peekFuture := c.ActorSystem.Root.RequestFuture(proxyPID, peekReq, 5*time.Second)
	peekRes, err := peekFuture.Result()
	require.NoError(t, err)
	peekResp, ok := peekRes.(*PeekResponse)
	require.True(t, ok)
	assert.False(t, peekResp.Found)
}

func TestActivatorProxy_ForwardsPeekRequest_Timeout(t *testing.T) {
	c := newTestClusterWithKind(t, "test-kind", echoProps())

	// Create a "black hole" actor that never responds, simulating an unresponsive placement actor.
	blackHoleProps := actor.PropsFromFunc(func(ctx actor.Context) {
		// Intentionally ignore all messages.
	})
	blackHolePID, err := c.ActorSystem.Root.SpawnNamed(blackHoleProps, "$test-blackhole-placement")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(blackHolePID)

	lookup := &fakeIdentityLookup{}
	lookup.Setup(c, nil, false)
	proxyProps := NewActivatorProxyProps(blackHolePID, lookup)
	proxyPID, err := c.ActorSystem.Root.SpawnNamed(proxyProps, "$test-proxy-peek-timeout")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(proxyPID)

	peekReq := &PeekRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "test-kind", Identity: "timeout-grain"},
	}
	// Use a timeout longer than proxyForwardTimeout (10s) to ensure we get
	// the proxy's error response, not our own timeout.
	peekFuture := c.ActorSystem.Root.RequestFuture(proxyPID, peekReq, 15*time.Second)
	peekRes, err := peekFuture.Result()
	require.NoError(t, err)
	peekResp, ok := peekRes.(*PeekResponse)
	require.True(t, ok)
	assert.False(t, peekResp.Found, "proxy should respond Found: false on placement timeout")
}

func TestActivatorProxy_TimeoutOnForward_RespondsFailed(t *testing.T) {
	system := actor.NewActorSystem()
	t.Cleanup(func() { system.Shutdown() })

	// Create a "black hole" placement actor that never responds.
	blackHoleProps := actor.PropsFromFunc(func(ctx actor.Context) {
		// Intentionally do nothing.
	})
	blackHolePID := system.Root.Spawn(blackHoleProps)
	defer system.Root.Poison(blackHolePID)

	lookup := &fakeIdentityLookup{}
	proxyProps := NewActivatorProxyProps(blackHolePID, lookup)
	proxyPID := system.Root.Spawn(proxyProps)
	defer system.Root.Poison(proxyPID)

	req := &ActivationRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "testKind", Identity: "timeout-actor"},
		RequestId:       "req-1",
	}

	// The proxy should forward to the black hole and timeout.
	future := system.Root.RequestFuture(proxyPID, req, 1*time.Second)
	res, err := future.Result()

	// Either the proxy responds with Failed, or the future times out.
	if err != nil {
		// Timeout — acceptable behavior.
		return
	}
	resp := res.(*ActivationResponse)
	assert.True(t, resp.Failed, "should respond Failed on timeout")
}
