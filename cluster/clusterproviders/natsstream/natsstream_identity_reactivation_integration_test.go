//go:build integration

package natsstream

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/remote"
	"google.golang.org/protobuf/types/known/emptypb"
)

// echoKind creates a Kind whose actor responds to emptypb.Empty with emptypb.Empty.
func echoKind() *cluster.Kind {
	return cluster.NewKind("echo", actor.PropsFromFunc(func(ctx actor.Context) {
		if _, ok := ctx.Message().(*emptypb.Empty); ok {
			ctx.Respond(&emptypb.Empty{})
		}
	}))
}

// startFullCluster creates and starts a complete cluster node including
// remote (gRPC), gossip, pubsub, identity lookup, and provider.
func startFullCluster(t *testing.T, natsURL, clusterName string, kinds []*cluster.Kind, opts ...Option) (*Provider, *cluster.Cluster) {
	t.Helper()

	nc, err := nats.Connect(natsURL)
	require.NoError(t, err)
	t.Cleanup(func() { nc.Close() })

	p, err := New(nc, opts...)
	require.NoError(t, err)

	system := actor.NewActorSystem()
	remoteCfg := remote.Configure("127.0.0.1", 0)
	clusterCfg := cluster.Configure(clusterName, p, p.IdentityLookup(), remoteCfg,
		cluster.WithKinds(kinds...),
	)
	c := cluster.NewCluster(system, clusterCfg)

	err = c.StartMember()
	require.NoError(t, err)

	return p, c
}

// crashCluster simulates a hard crash: shuts down remote transport and kills
// the provider WITHOUT calling IdentityLookup.Shutdown(). This leaves stale
// identity claims in the NATS stream, exactly like a real process crash.
func crashCluster(t *testing.T, p *Provider, c *cluster.Cluster) {
	t.Helper()

	c.Remote.Shutdown(false)
	c.ActorSystem.Shutdown()

	p.shutdown.Store(true)
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
}

// waitForMutualDiscovery blocks until both providers see each other.
func waitForMutualDiscovery(t *testing.T, p1, p2 *Provider) {
	t.Helper()
	require.Eventually(t, func() bool {
		p1.membersMu.RLock()
		_, found := p1.members[p2.self.ID]
		p1.membersMu.RUnlock()
		return found
	}, 15*time.Second, 200*time.Millisecond, "node 1 should discover node 2")

	require.Eventually(t, func() bool {
		p2.membersMu.RLock()
		_, found := p2.members[p1.self.ID]
		p2.membersMu.RUnlock()
		return found
	}, 15*time.Second, 200*time.Millisecond, "node 2 should discover node 1")
}

// TestIntegration_CrashReactivation_FullCluster starts two real cluster nodes,
// activates a grain on node 2, crashes node 2 without graceful shutdown
// (leaving stale identity claims in the NATS stream), and verifies that
// node 1 can reactivate the grain via cluster.Request().
//
// Without the RemovePid fix: stale activation persists, all retries fail.
// With the fix: dead letter triggers RemovePid, stale cleared, retry spawns locally.
func TestIntegration_CrashReactivation_FullCluster(t *testing.T) {
	natsURL := startNATSContainer(t)

	// Long timeouts so topology events don't fire during the test.
	// This isolates the RemovePid code path.
	opts := []Option{
		WithHeartbeatInterval(30 * time.Second),
		WithHeartbeatTTL(120 * time.Second),
		WithMemberTimeout(120 * time.Second),
		WithCheckInterval(30 * time.Second),
		WithLeaderTTL(120 * time.Second),
	}

	echo := echoKind()

	p1, c1 := startFullCluster(t, natsURL, "integ-crash-react", []*cluster.Kind{echo}, opts...)
	t.Cleanup(func() { c1.Shutdown(true) })

	time.Sleep(1 * time.Second)

	p2, c2 := startFullCluster(t, natsURL, "integ-crash-react", []*cluster.Kind{echo}, opts...)

	waitForMutualDiscovery(t, p1, p2)

	// Activate grain on node 2 via identity lookup.
	pid2 := c2.Get("grain-crash-1", "echo")
	require.NotNil(t, pid2, "grain should activate on node 2")

	// Verify activation is stored in the identity stream (visible from node 1).
	ctx := context.Background()
	ci := cluster.NewClusterIdentity("grain-crash-1", "echo")
	rec := p1.IdentityLookup().getExistingActivation(ctx, ci)
	require.NotNil(t, rec, "activation should be visible from node 1")
	require.Equal(t, pid2.Address, rec.PidAddress)

	// Crash node 2 — identity claims persist in the stream.
	crashCluster(t, p2, c2)
	time.Sleep(1 * time.Second)

	// From node 1, request the grain. Should succeed after RemovePid clears stale.
	resp, err := c1.Request("grain-crash-1", "echo", &emptypb.Empty{},
		cluster.WithTimeout(90*time.Second),
		cluster.WithRetryCount(10),
	)

	require.NoError(t, err,
		"cluster.Request must succeed after RemovePid clears the stale activation; "+
			"if this fails, DefaultContext.Request is not calling RemovePid on dead letter")
	require.NotNil(t, resp)
	_, ok := resp.(*emptypb.Empty)
	require.True(t, ok, "response should be *emptypb.Empty")

	// Verify grain is now on node 1.
	newRec := p1.IdentityLookup().getExistingActivation(ctx, ci)
	require.NotNil(t, newRec)
	assert.Equal(t, c1.ActorSystem.Address(), newRec.PidAddress,
		"grain should now be activated on node 1")
}

// TestIntegration_ConcurrentRequests_AfterCrash_NoDoubleActivation launches
// multiple concurrent Request() calls after a node crash to verify:
// - Concurrent RemovePid calls (from dead letters) are safe under -race
// - Exactly one activation exists after recovery (no double-activation)
// - PID validation in RemovePid prevents wiping fresh activations
func TestIntegration_ConcurrentRequests_AfterCrash_NoDoubleActivation(t *testing.T) {
	natsURL := startNATSContainer(t)

	opts := []Option{
		WithHeartbeatInterval(30 * time.Second),
		WithHeartbeatTTL(120 * time.Second),
		WithMemberTimeout(120 * time.Second),
		WithCheckInterval(30 * time.Second),
		WithLeaderTTL(120 * time.Second),
	}

	echo := echoKind()

	p1, c1 := startFullCluster(t, natsURL, "integ-concurrent-crash", []*cluster.Kind{echo}, opts...)
	t.Cleanup(func() { c1.Shutdown(true) })

	time.Sleep(1 * time.Second)

	p2, c2 := startFullCluster(t, natsURL, "integ-concurrent-crash", []*cluster.Kind{echo}, opts...)

	waitForMutualDiscovery(t, p1, p2)

	// Activate grain on node 2.
	pid2 := c2.Get("grain-concurrent-1", "echo")
	require.NotNil(t, pid2)

	// Crash node 2.
	crashCluster(t, p2, c2)
	time.Sleep(1 * time.Second)

	// Launch N concurrent requests from node 1.
	const numGoroutines = 10
	var wg sync.WaitGroup
	var successCount atomic.Int32
	var failCount atomic.Int32
	pids := make([]*actor.PID, numGoroutines)
	var pidsMu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			resp, err := c1.Request("grain-concurrent-1", "echo", &emptypb.Empty{},
				cluster.WithTimeout(90*time.Second),
				cluster.WithRetryCount(10),
			)
			if err == nil && resp != nil {
				successCount.Add(1)
				if pid, ok := c1.PidCache.Get("grain-concurrent-1", "echo"); ok {
					pidsMu.Lock()
					pids[idx] = pid
					pidsMu.Unlock()
				}
			} else {
				failCount.Add(1)
			}
		}(i)
	}

	wg.Wait()

	assert.Greater(t, successCount.Load(), int32(0),
		"at least one concurrent request should succeed after crash recovery")

	// Verify exactly ONE activation exists (no double-activation).
	ctx := context.Background()
	ci := cluster.NewClusterIdentity("grain-concurrent-1", "echo")
	rec := p1.IdentityLookup().getExistingActivation(ctx, ci)
	require.NotNil(t, rec)
	assert.Equal(t, c1.ActorSystem.Address(), rec.PidAddress,
		"the single activation should be on node 1")

	// All successful requests should use the same PID.
	pidsMu.Lock()
	var firstPID *actor.PID
	for _, pid := range pids {
		if pid != nil {
			if firstPID == nil {
				firstPID = pid
			} else {
				assert.True(t, firstPID.Equal(pid),
					"all successful requests must use the same PID (no double-activation)")
			}
		}
	}
	pidsMu.Unlock()
}

// TestIntegration_GracefulShutdown_Reactivation verifies the baseline:
// graceful shutdown cleans up identity claims, and reactivation works immediately.
func TestIntegration_GracefulShutdown_Reactivation(t *testing.T) {
	natsURL := startNATSContainer(t)

	opts := []Option{
		WithHeartbeatInterval(30 * time.Second),
		WithHeartbeatTTL(120 * time.Second),
		WithMemberTimeout(120 * time.Second),
		WithCheckInterval(30 * time.Second),
		WithLeaderTTL(120 * time.Second),
	}

	echo := echoKind()

	p1, c1 := startFullCluster(t, natsURL, "integ-graceful-react", []*cluster.Kind{echo}, opts...)
	t.Cleanup(func() { c1.Shutdown(true) })

	time.Sleep(1 * time.Second)

	_, c2 := startFullCluster(t, natsURL, "integ-graceful-react", []*cluster.Kind{echo}, opts...)

	time.Sleep(3 * time.Second)

	// Activate grain on node 2.
	pid2 := c2.Get("grain-graceful-1", "echo")
	require.NotNil(t, pid2)

	// Graceful shutdown cleans up identity claims.
	c2.Shutdown(true)
	time.Sleep(1 * time.Second)

	// Stale activation should be gone.
	ctx := context.Background()
	ci := cluster.NewClusterIdentity("grain-graceful-1", "echo")
	rec := p1.IdentityLookup().getExistingActivation(ctx, ci)
	assert.Nil(t, rec, "graceful shutdown should clean up identity claims")

	// Node 1 should reactivate the grain.
	resp, err := c1.Request("grain-graceful-1", "echo", &emptypb.Empty{},
		cluster.WithTimeout(10*time.Second),
		cluster.WithRetryCount(5),
	)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

// TestIntegration_RemovePid_PidMismatch_PreservesFreshActivation verifies
// that RemovePid with a stale PID does NOT delete a fresh activation.
// This is the TOCTOU race guard.
func TestIntegration_RemovePid_PidMismatch_PreservesFreshActivation(t *testing.T) {
	natsURL := startNATSContainer(t)

	opts := []Option{
		WithHeartbeatInterval(30 * time.Second),
		WithHeartbeatTTL(120 * time.Second),
		WithMemberTimeout(120 * time.Second),
		WithCheckInterval(30 * time.Second),
		WithLeaderTTL(120 * time.Second),
	}

	echo := echoKind()

	p1, c1 := startFullCluster(t, natsURL, "integ-pid-mismatch", []*cluster.Kind{echo}, opts...)
	t.Cleanup(func() { c1.Shutdown(true) })

	time.Sleep(2 * time.Second)

	// Activate grain on node 1.
	pid1 := c1.Get("grain-mismatch-1", "echo")
	require.NotNil(t, pid1)

	// Verify activation exists.
	ctx := context.Background()
	ci := cluster.NewClusterIdentity("grain-mismatch-1", "echo")
	il := p1.IdentityLookup()
	rec := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec)
	require.Equal(t, pid1.Address, rec.PidAddress)

	// Call RemovePid with a DIFFERENT PID (simulating stale cleanup from
	// a concurrent node that had an old PID).
	stalePid := actor.NewPID("dead-host:9999", "echo/grain-mismatch-1")
	il.RemovePid(ci, stalePid)

	// Fresh activation must survive.
	recAfter := il.getExistingActivation(ctx, ci)
	require.NotNil(t, recAfter,
		"RemovePid with mismatched PID must NOT delete a fresh activation")
	assert.Equal(t, pid1.Address, recAfter.PidAddress)
	assert.Equal(t, pid1.Id, recAfter.PidID)
}
