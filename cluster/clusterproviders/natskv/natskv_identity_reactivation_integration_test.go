//go:build integration

package natskv

import (
	"context"
	"fmt"
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
// This is a real cluster node that can communicate with other nodes.
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
// identity claims in NATS KV, exactly like a real process crash.
func crashCluster(t *testing.T, p *Provider, c *cluster.Cluster) {
	t.Helper()

	// Shut down remote so the node is unreachable for gRPC messages.
	c.Remote.Shutdown(false)

	// Shut down the actor system so local actors stop.
	c.ActorSystem.Shutdown()

	// Kill the provider without graceful cleanup.
	// This does NOT call IdentityLookup.Shutdown(), so identity claims persist.
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
		_, p1SeesP2 := p1.members[p2.self.ID]
		p1.membersMu.RUnlock()
		return p1SeesP2
	}, 15*time.Second, 200*time.Millisecond, "node 1 should discover node 2")

	require.Eventually(t, func() bool {
		p2.membersMu.RLock()
		_, p2SeesP1 := p2.members[p1.self.ID]
		p2.membersMu.RUnlock()
		return p2SeesP1
	}, 15*time.Second, 200*time.Millisecond, "node 2 should discover node 1")
}

// TestIntegration_CrashReactivation_FullCluster starts two real cluster nodes,
// activates a grain on node 2 via the identity lookup, crashes node 2 without
// graceful shutdown (leaving stale identity claims), and then verifies that
// node 1 can reactivate the grain by calling cluster.Request().
//
// Without the RemovePid fix in DefaultContext.Request(), the stale activation
// persists in NATS KV, every retry returns the dead PID, and Request fails.
//
// With the fix, the first dead letter triggers RemovePid which clears the
// stale activation, allowing the retry to spawn a new actor on node 1.
func TestIntegration_CrashReactivation_FullCluster(t *testing.T) {
	natsURL := startNATSContainer(t)

	// Use a long member TTL so topology events do NOT clean up stale
	// activations during the test. This forces the test to rely entirely
	// on the RemovePid path (triggered by DefaultContext on dead letter).
	opts := []Option{
		WithMemberTTL(120 * time.Second),
		WithRefreshInterval(30 * time.Second),
		WithLeaderTTL(120 * time.Second),
	}

	echo := echoKind()

	p1, c1 := startFullCluster(t, natsURL, "integ-crash-react", []*cluster.Kind{echo}, opts...)
	t.Cleanup(func() { c1.Shutdown(true) })

	time.Sleep(1 * time.Second)

	p2, c2 := startFullCluster(t, natsURL, "integ-crash-react", []*cluster.Kind{echo}, opts...)
	// No Cleanup for c2 — we crash it manually below.

	waitForMutualDiscovery(t, p1, p2)

	// Activate the grain on node 2 via its identity lookup.
	// Get() acquires the lock and spawns the actor locally on node 2.
	pid2 := c2.Get("grain-crash-1", "echo")
	require.NotNil(t, pid2, "grain should activate on node 2")

	// Verify the activation is stored in NATS KV (visible from node 1).
	ctx := context.Background()
	ci := cluster.NewClusterIdentity("grain-crash-1", "echo")
	rec := p1.IdentityLookup().getExistingActivation(ctx, ci)
	require.NotNil(t, rec, "activation should be visible in NATS KV from node 1")
	require.Equal(t, pid2.Address, rec.PidAddress, "stored PID should point to node 2")

	// Crash node 2 without graceful shutdown.
	// Identity claims remain in NATS KV.
	crashCluster(t, p2, c2)

	// Wait briefly for remote connections to detect the failure.
	time.Sleep(1 * time.Second)

	// From node 1, request the grain. The stale activation points to dead node 2.
	//
	// Expected behavior WITH the RemovePid fix:
	//   1. DefaultContext.Request → getPid → IdentityLookup.Get → returns stale PID
	//   2. RequestFuture to stale PID → dead letter / timeout
	//   3. Retry: clears PID cache, calls RemovePid (clears stale from NATS KV)
	//   4. Next getPid → IdentityLookup.Get → no activation → acquires lock → spawns locally
	//   5. Request to local actor → success
	//
	// Expected behavior WITHOUT the fix:
	//   1-2. Same as above
	//   3. Retry: clears PID cache only (RemovePid never called)
	//   4. Next getPid → IdentityLookup.Get → stale activation still there → returns dead PID
	//   5. Repeat until retries exhausted → failure
	// Use a long timeout because DefaultContext uses the same timeout for
	// both the outer context and each individual RequestFuture. The first
	// attempt to the dead node blocks until the remote endpoint writer
	// exhausts its connection retries (~15-20s). We need enough remaining
	// time for RemovePid + retry + local spawn.
	resp, err := c1.Request("grain-crash-1", "echo", &emptypb.Empty{},
		cluster.WithTimeout(90*time.Second),
		cluster.WithRetryCount(10),
	)

	require.NoError(t, err,
		"cluster.Request must succeed after RemovePid clears the stale activation; "+
			"if this fails, DefaultContext.Request is not calling RemovePid on dead letter")
	require.NotNil(t, resp, "response must not be nil")
	_, ok := resp.(*emptypb.Empty)
	require.True(t, ok, "response should be *emptypb.Empty")

	// Verify the grain is now activated on node 1 (not the crashed node 2).
	newRec := p1.IdentityLookup().getExistingActivation(ctx, ci)
	require.NotNil(t, newRec, "new activation should exist in NATS KV")
	assert.Equal(t, c1.ActorSystem.Address(), newRec.PidAddress,
		"grain should now be activated on node 1 (the surviving node)")
}

// TestIntegration_ConcurrentRequests_AfterCrash_NoDoubleActivation starts
// two real cluster nodes, activates a grain on node 2, crashes node 2, then
// launches multiple concurrent Request() calls from node 1 for the same grain.
//
// This tests that:
// - Concurrent RemovePid calls (triggered by dead letters) are safe under -race
// - Exactly one new activation exists after all requests complete (no double-activation)
// - The PID validation in RemovePid prevents wiping a fresh activation
func TestIntegration_ConcurrentRequests_AfterCrash_NoDoubleActivation(t *testing.T) {
	natsURL := startNATSContainer(t)

	opts := []Option{
		WithMemberTTL(120 * time.Second),
		WithRefreshInterval(30 * time.Second),
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

	// Launch concurrent requests from node 1 for the same grain.
	// All goroutines share the same endpoint writer to the dead node.
	// After the endpoint writer exhausts retries, all futures receive
	// dead letter/timeout errors, triggering RemovePid + retry.
	const numGoroutines = 5
	var wg sync.WaitGroup
	var successCount atomic.Int32
	var failCount atomic.Int32
	errors := make([]error, numGoroutines)
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
			if err != nil {
				failCount.Add(1)
				errors[idx] = err
				return
			}
			if resp == nil {
				failCount.Add(1)
				errors[idx] = fmt.Errorf("nil response")
				return
			}
			successCount.Add(1)
			if pid, ok := c1.PidCache.Get("grain-concurrent-1", "echo"); ok {
				pidsMu.Lock()
				pids[idx] = pid
				pidsMu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// Log diagnostic info for any failures.
	for i, err := range errors {
		if err != nil {
			t.Logf("goroutine %d failed: %v", i, err)
		}
	}
	t.Logf("success=%d fail=%d", successCount.Load(), failCount.Load())

	// Inspect raw NATS KV state for the identity key.
	ctx := context.Background()
	ci := cluster.NewClusterIdentity("grain-concurrent-1", "echo")
	key := kvKey(ci)
	il := p1.IdentityLookup()

	// Direct KV Get (bypasses getExistingActivation's PID check).
	rawEntry, rawErr := il.identities.Get(ctx, key)
	if rawErr != nil {
		t.Logf("raw KV Get(%q): error=%v", key, rawErr)
	} else {
		t.Logf("raw KV Get(%q): value=%s revision=%d op=%v", key, string(rawEntry.Value()), rawEntry.Revision(), rawEntry.Operation())
	}

	// List ALL keys in the identities bucket to see full state.
	allKeys, keysErr := il.identities.Keys(ctx)
	if keysErr != nil {
		t.Logf("identities.Keys(): error=%v", keysErr)
	} else {
		t.Logf("identities.Keys(): %v", allKeys)
	}

	// Also check member tracker for both members.
	for _, mid := range []string{il.memberID, "other"} {
		trackEntry, trackErr := il.memberTracker.Get(ctx, mid)
		if trackErr != nil {
			t.Logf("memberTracker.Get(%q): error=%v", mid, trackErr)
		} else {
			t.Logf("memberTracker.Get(%q): value=%s", mid, string(trackEntry.Value()))
		}
	}

	rec := il.getExistingActivation(ctx, ci)
	t.Logf("getExistingActivation: %+v", rec)

	// At least one request must succeed.
	require.Greater(t, successCount.Load(), int32(0),
		"at least one concurrent request should succeed after crash recovery")

	// Verify exactly ONE activation exists in NATS KV (no double-activation).
	require.NotNil(t, rec, "activation should exist after concurrent recovery")
	assert.Equal(t, c1.ActorSystem.Address(), rec.PidAddress,
		"the single activation should be on node 1")

	// All successful requests must use the same PID (no double-activation).
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
// when a node shuts down gracefully, IdentityLookup.Shutdown() cleans up
// identity claims, and the surviving node can reactivate grains immediately.
// This should work regardless of the RemovePid fix.
func TestIntegration_GracefulShutdown_Reactivation(t *testing.T) {
	natsURL := startNATSContainer(t)

	opts := []Option{
		WithMemberTTL(120 * time.Second),
		WithRefreshInterval(30 * time.Second),
		WithLeaderTTL(120 * time.Second),
	}

	echo := echoKind()

	p1, c1 := startFullCluster(t, natsURL, "integ-graceful-react", []*cluster.Kind{echo}, opts...)
	t.Cleanup(func() { c1.Shutdown(true) })

	time.Sleep(1 * time.Second)

	p2, c2 := startFullCluster(t, natsURL, "integ-graceful-react", []*cluster.Kind{echo}, opts...)

	waitForMutualDiscovery(t, p1, p2)
	_ = p2 // used only for discovery

	// Activate grain on node 2.
	pid2 := c2.Get("grain-graceful-1", "echo")
	require.NotNil(t, pid2)

	// Graceful shutdown of node 2 — this calls IdentityLookup.Shutdown()
	// which cleans up all identity claims.
	c2.Shutdown(true)
	time.Sleep(1 * time.Second)

	// Verify stale activation is gone (cleaned by graceful shutdown).
	ctx := context.Background()
	ci := cluster.NewClusterIdentity("grain-graceful-1", "echo")
	rec := p1.IdentityLookup().getExistingActivation(ctx, ci)
	assert.Nil(t, rec, "graceful shutdown should clean up identity claims")

	// Node 1 should be able to activate the grain.
	resp, err := c1.Request("grain-graceful-1", "echo", &emptypb.Empty{},
		cluster.WithTimeout(10*time.Second),
		cluster.WithRetryCount(5),
	)
	require.NoError(t, err, "request should succeed after graceful shutdown of hosting node")
	require.NotNil(t, resp)
}

// TestIntegration_RemovePid_PidMismatch_PreservesFreshActivation verifies
// that when RemovePid is called with a stale PID but the identity store
// already contains a FRESH activation (different PID), the fresh activation
// is NOT deleted.
//
// This is the TOCTOU race guard: concurrent crash recovery must not wipe
// a freshly-spawned activation.
//
// This test uses the identity lookup directly (not cluster.Request) because
// the race window requires precise control over timing. However, the
// concurrent full-cluster test above exercises this implicitly.
func TestIntegration_RemovePid_PidMismatch_PreservesFreshActivation(t *testing.T) {
	natsURL := startNATSContainer(t)

	opts := []Option{
		WithMemberTTL(120 * time.Second),
		WithRefreshInterval(30 * time.Second),
		WithLeaderTTL(120 * time.Second),
	}

	echo := echoKind()

	p1, c1 := startFullCluster(t, natsURL, "integ-pid-mismatch", []*cluster.Kind{echo}, opts...)
	t.Cleanup(func() { c1.Shutdown(true) })

	time.Sleep(2 * time.Second)

	// Activate a grain on node 1 (the only node).
	pid1 := c1.Get("grain-mismatch-1", "echo")
	require.NotNil(t, pid1, "grain should activate on node 1")

	// Verify the activation exists.
	ctx := context.Background()
	ci := cluster.NewClusterIdentity("grain-mismatch-1", "echo")
	il := p1.IdentityLookup()
	rec := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec)
	require.Equal(t, pid1.Address, rec.PidAddress)

	// Simulate a stale RemovePid call with a DIFFERENT PID (as if another
	// node was trying to clean up an old activation that no longer exists).
	stalePid := actor.NewPID("dead-host:9999", "echo/grain-mismatch-1")
	il.RemovePid(ci, stalePid)

	// The fresh activation (pid1) must survive because the PID doesn't match.
	recAfter := il.getExistingActivation(ctx, ci)
	require.NotNil(t, recAfter,
		"RemovePid with mismatched PID must NOT delete a fresh activation; "+
			"this prevents the TOCTOU race where concurrent crash recovery "+
			"could wipe a just-spawned grain")
	assert.Equal(t, pid1.Address, recAfter.PidAddress,
		"the fresh activation's PID must be preserved")
	assert.Equal(t, pid1.Id, recAfter.PidID,
		"the fresh activation's PID ID must be preserved")
}
