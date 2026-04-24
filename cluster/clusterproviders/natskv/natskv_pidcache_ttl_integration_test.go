//go:build integration

package natskv

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awevoke/protoactor-go/cluster"
	"google.golang.org/protobuf/types/known/emptypb"
)

// TestIntegration_PidCacheTTL_ExpiresStaleActivation starts two real cluster
// nodes with a short PID cache TTL, activates a grain on node 2, crashes
// node 2 (no graceful shutdown), and verifies that:
//  1. Node 1's PID cache entry for the grain expires after the TTL.
//  2. A subsequent cluster.Request() from node 1 triggers a fresh activation
//     on node 1 itself.
//
// This tests the defense-in-depth behavior: even if topology events are
// delayed (we use long member TTLs to simulate this), the PID cache TTL
// ensures stale entries self-evict.
func TestIntegration_PidCacheTTL_ExpiresStaleActivation(t *testing.T) {
	natsURL := startNATSContainer(t)

	// Long member TTL so topology-driven cleanup does NOT fire during the test.
	// This isolates the PID cache TTL as the only cleanup mechanism.
	providerOpts := []Option{
		WithMemberTTL(120 * time.Second),
		WithRefreshInterval(30 * time.Second),
		WithLeaderTTL(120 * time.Second),
	}

	pidCacheTTL := 2 * time.Second

	echo := echoKind()
	kinds := []*cluster.Kind{echo}

	p1, c1 := startFullCluster(t, natsURL, "integ-pidcache-ttl", kinds, providerOpts,
		cluster.WithPidCacheTTL(pidCacheTTL),
	)
	t.Cleanup(func() { c1.Shutdown(true) })

	time.Sleep(1 * time.Second)

	p2, c2 := startFullCluster(t, natsURL, "integ-pidcache-ttl", kinds, providerOpts,
		cluster.WithPidCacheTTL(pidCacheTTL),
	)

	waitForMutualDiscovery(t, p1, p2)

	// Force grain activation on node 2 by calling Get() directly on c2.
	// Using c1.Request() would be non-deterministic — the grain could land
	// on either node depending on consistent hashing.
	pid2 := c2.Get("grain-ttl-1", "echo")
	require.NotNil(t, pid2, "grain should activate on node 2")

	// Now request the grain from node 1 to populate node 1's PID cache
	// with node 2's PID.
	resp, err := c1.Request("grain-ttl-1", "echo", &emptypb.Empty{},
		cluster.WithTimeout(30*time.Second),
		cluster.WithRetryCount(5),
	)
	require.NoError(t, err, "initial request should succeed")
	require.NotNil(t, resp)

	// Verify node 1's PID cache has an entry pointing to node 2.
	cachedPid, cached := c1.PidCache.Get("grain-ttl-1", "echo")
	require.True(t, cached, "PID cache should have entry after activation")
	require.Equal(t, pid2.Address, cachedPid.Address,
		"cached PID should point to node 2")

	// Crash node 2 without graceful shutdown.
	// Identity claims remain in NATS KV. Topology events are delayed
	// because of the long member TTL.
	crashCluster(t, p2, c2)

	// Wait briefly for remote connections to notice the failure,
	// then wait for the PID cache TTL to expire.
	time.Sleep(pidCacheTTL + 500*time.Millisecond)

	// The stale PID cache entry should now be expired.
	_, cached = c1.PidCache.Get("grain-ttl-1", "echo")
	assert.False(t, cached, "PID cache entry should have expired after TTL")

	// A new request should trigger fresh activation on node 1.
	// The stale KV entry will cause one failed attempt (dead letter to crashed
	// node 2), which triggers RemovePid, clearing the KV. The retry then
	// spawns locally on node 1.
	resp, err = c1.Request("grain-ttl-1", "echo", &emptypb.Empty{},
		cluster.WithTimeout(90*time.Second),
		cluster.WithRetryCount(10),
	)
	require.NoError(t, err, "request should succeed after PID cache TTL expiry and reactivation")
	require.NotNil(t, resp)

	// Verify the grain is now on node 1.
	newPid, ok := c1.PidCache.Get("grain-ttl-1", "echo")
	require.True(t, ok, "new activation should populate PID cache")
	assert.Equal(t, c1.ActorSystem.Address(), newPid.Address,
		"grain should now be activated on surviving node 1")
}
