# PID Cache TTL Configuration Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow users to configure a TTL for the cluster PID cache so stale entries auto-evict, providing defense-in-depth against delayed topology cleanup.

**Architecture:** Add a `PidCacheTTL` field to the existing `Config` struct with a `WithPidCacheTTL()` functional option. Change `NewCluster()` to call `NewPidCacheWithTTL(config.PidCacheTTL)` instead of `NewPidCache()`. The `NewPidCacheWithTTL` function is already implemented and tested. Default is zero (no expiry, backward compatible).

**Tech Stack:** Go, testify (assert/require), testcontainers (NATS), protoactor-go cluster package

**Spec:** `docs/superpowers/specs/2026-03-15-pid-cache-ttl-config-design.md`

---

## Chunk 1: Unit Tests and Production Code

### Task 1: Red — Config option test

**Files:**
- Modify: `cluster/config_test.go` (add test at end of file, after line 170)

- [ ] **Step 1: Write the failing test**

Add to `cluster/config_test.go`:

```go
func TestClusterConfig_WithPidCacheTTL(t *testing.T) {
	provider := newInmemoryProvider()
	lookup := &fakeIdentityLookup{}
	rc := remote.Configure("localhost", 0)
	config := Configure("test-cluster", provider, lookup, rc,
		WithPidCacheTTL(30*time.Second),
	)
	assert.Equal(t, 30*time.Second, config.PidCacheTTL)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./cluster/ -run TestClusterConfig_WithPidCacheTTL -v`

Expected: Compilation error — `WithPidCacheTTL` undefined, `PidCacheTTL` unknown field.

---

### Task 2: Green — Config field and option

**Files:**
- Modify: `cluster/config.go:33` (add field after `GrainMetricsEnabled`)
- Modify: `cluster/config_opts.go` (add option at end of file, after line 106)

- [ ] **Step 3: Add `PidCacheTTL` field to Config**

In `cluster/config.go`, add after line 33 (`GrainMetricsEnabled bool`):

```go
	// PidCacheTTL sets the time-to-live for PID cache entries. Expiration is
	// lazy (on-read): stale entries are evicted when Get() is called after the
	// TTL has elapsed, not by a background reaper. Zero means no expiry.
	PidCacheTTL time.Duration
```

- [ ] **Step 4: Add `WithPidCacheTTL` option**

Append to `cluster/config_opts.go`:

```go
// WithPidCacheTTL sets the time-to-live for PID cache entries.
// Zero (the default) means entries never expire.
func WithPidCacheTTL(ttl time.Duration) ConfigOption {
	return func(c *Config) {
		c.PidCacheTTL = ttl
	}
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./cluster/ -run TestClusterConfig_WithPidCacheTTL -v`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add cluster/config.go cluster/config_opts.go cluster/config_test.go
git commit -m "feat(cluster): add PidCacheTTL config field and WithPidCacheTTL option"
```

---

### Task 3: Characterization — Default cluster has no-expiry PID cache

**Files:**
- Modify: `cluster/cluster_test.go` (add test at end of file, after line 225)

- [ ] **Step 7: Write the failing test**

Add to `cluster/cluster_test.go`:

```go
func TestNewCluster_DefaultPidCacheTTL_NoExpiry(t *testing.T) {
	c := newClusterForTest("test-default-ttl", newInmemoryProvider())

	pid := actor.NewPID("localhost:8080", "test/grain-1")
	c.PidCache.Set("grain-1", "test", pid)

	// With zero TTL, entry should persist indefinitely.
	time.Sleep(50 * time.Millisecond)
	got, ok := c.PidCache.Get("grain-1", "test")
	assert.True(t, ok, "entry should still exist with zero TTL (default)")
	assert.True(t, pid.Equal(got), "cached PID should match")
}
```

- [ ] **Step 8: Run test to verify it passes (green already — confirms backward compat)**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./cluster/ -run TestNewCluster_DefaultPidCacheTTL_NoExpiry -v`

Expected: PASS — This test should already pass because `NewPidCache()` creates a zero-TTL cache. This is a characterization test confirming the current default before we change the wiring. We keep it as our safety net.

- [ ] **Step 9: Commit**

```bash
git add cluster/cluster_test.go
git commit -m "test(cluster): add characterization test for default PID cache no-expiry behavior"
```

---

### Task 4: Red — Cluster with PidCacheTTL expires entries

**Files:**
- Modify: `cluster/cluster_test.go` (add test after the one just added)

- [ ] **Step 10: Write the failing test**

Add to `cluster/cluster_test.go`:

```go
func TestNewCluster_WithPidCacheTTL_ExpiresCachedEntries(t *testing.T) {
	c := newClusterForTest("test-ttl-expiry", newInmemoryProvider(),
		WithPidCacheTTL(100*time.Millisecond),
	)

	pid := actor.NewPID("localhost:8080", "test/grain-1")
	c.PidCache.Set("grain-1", "test", pid)

	// Entry should exist immediately.
	got, ok := c.PidCache.Get("grain-1", "test")
	assert.True(t, ok, "entry should exist before TTL")
	assert.True(t, pid.Equal(got))

	// After TTL elapses, entry should be gone.
	time.Sleep(150 * time.Millisecond)
	_, ok = c.PidCache.Get("grain-1", "test")
	assert.False(t, ok, "entry should have expired after TTL")
}
```

- [ ] **Step 11: Run test to verify it fails**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./cluster/ -run TestNewCluster_WithPidCacheTTL_ExpiresCachedEntries -v`

Expected: FAIL — The entry won't expire because `NewCluster()` still calls `NewPidCache()` (zero TTL) regardless of the config option. The second assertion (`assert.False(t, ok, ...)`) will fail.

---

### Task 5: Green — Wire PidCacheTTL into NewCluster

**Files:**
- Modify: `cluster/cluster.go:62`

- [ ] **Step 12: Change `NewPidCache()` to `NewPidCacheWithTTL(config.PidCacheTTL)`**

In `cluster/cluster.go`, change line 62 from:

```go
	c.PidCache = NewPidCache()
```

to:

```go
	c.PidCache = NewPidCacheWithTTL(config.PidCacheTTL)
```

- [ ] **Step 13: Run all cluster tests to verify green**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./cluster/ -v -race -count=1`

Expected: ALL PASS — both the default no-expiry test and the TTL expiry test should pass.

- [ ] **Step 14: Commit**

```bash
git add cluster/cluster.go cluster/cluster_test.go
git commit -m "feat(cluster): wire PidCacheTTL config into NewCluster PID cache construction"
```

---

## Chunk 2: Integration Test

### Task 6: Update `startFullCluster` helper to accept cluster config options

**Files:**
- Modify: `cluster/clusterproviders/natskv/natskv_identity_reactivation_integration_test.go:35-56`

- [ ] **Step 15: Add `clusterOpts` parameter to `startFullCluster`**

Change the `startFullCluster` function signature and body. Current (lines 35-56):

```go
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
```

New version — add a `clusterOpts` parameter:

```go
func startFullCluster(t *testing.T, natsURL, clusterName string, kinds []*cluster.Kind, opts []Option, clusterOpts ...cluster.ConfigOption) (*Provider, *cluster.Cluster) {
	t.Helper()

	nc, err := nats.Connect(natsURL)
	require.NoError(t, err)
	t.Cleanup(func() { nc.Close() })

	p, err := New(nc, opts...)
	require.NoError(t, err)

	system := actor.NewActorSystem()
	remoteCfg := remote.Configure("127.0.0.1", 0)
	allClusterOpts := append([]cluster.ConfigOption{cluster.WithKinds(kinds...)}, clusterOpts...)
	clusterCfg := cluster.Configure(clusterName, p, p.IdentityLookup(), remoteCfg, allClusterOpts...)
	c := cluster.NewCluster(system, clusterCfg)

	err = c.StartMember()
	require.NoError(t, err)

	return p, c
}
```

**Important:** This changes the `opts` parameter from variadic (`...Option`) to a slice (`[]Option`). All existing callers pass provider options as a named slice variable already (e.g., `opts := []Option{...}`; `startFullCluster(t, natsURL, name, kinds, opts...)`), so each call site needs the trailing `...` removed. Update all callers:

In `natskv_identity_reactivation_integration_test.go`, find every call to `startFullCluster` and change `opts...` to `opts`. There are 7 call sites (lines 121, 126, 207, 212, 349, 354, 405). For example, change:

```go
p1, c1 := startFullCluster(t, natsURL, "integ-crash-react", []*cluster.Kind{echo}, opts...)
```

to:

```go
p1, c1 := startFullCluster(t, natsURL, "integ-crash-react", []*cluster.Kind{echo}, opts)
```

- [ ] **Step 16: Verify existing integration tests still compile**

First, check for all callers of `startFullCluster` across the natskv package:

Run: `cd /home/cchamplin/development/protoactor-go && grep -rn 'startFullCluster' cluster/clusterproviders/natskv/`

Update any additional callers the same way (change variadic `opts...` to slice `opts`).

Then verify compilation (must use `-tags=integration` since the function and callers are in integration-tagged files):

Run: `cd /home/cchamplin/development/protoactor-go && go test -tags=integration -run='^$' -count=0 ./cluster/clusterproviders/natskv/`

Expected: Compiles successfully (no tests run, just compilation check).

- [ ] **Step 17: Commit**

```bash
git add cluster/clusterproviders/natskv/natskv_identity_reactivation_integration_test.go
git commit -m "refactor(natskv): add cluster config options parameter to startFullCluster helper"
```

---

### Task 7: Red — Integration test for PID cache TTL with real cluster

**Files:**
- Create: `cluster/clusterproviders/natskv/natskv_pidcache_ttl_integration_test.go`

- [ ] **Step 18: Write the failing integration test**

Create `cluster/clusterproviders/natskv/natskv_pidcache_ttl_integration_test.go`:

```go
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
//   1. Node 1's PID cache entry for the grain expires after the TTL.
//   2. A subsequent cluster.Request() from node 1 triggers a fresh activation
//      on node 1 itself.
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
```

- [ ] **Step 19: Run test to verify it fails (red)**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./cluster/clusterproviders/natskv/ -tags=integration -run TestIntegration_PidCacheTTL_ExpiresStaleActivation -v -race -timeout 300s`

Expected: If the production code from Task 5 is already in place, this test should PASS (green). If running this task independently before Task 5, it would fail because the PID cache TTL wouldn't be wired through.

Since we're following the plan sequentially and Task 5 is already done, this should be green immediately — it's an integration-level verification of the unit-tested behavior.

- [ ] **Step 20: Commit**

```bash
git add cluster/clusterproviders/natskv/natskv_pidcache_ttl_integration_test.go
git commit -m "test(natskv): add integration test for PID cache TTL expiry after node crash"
```

---

### Task 8: Run full test suite

- [ ] **Step 21: Run all cluster unit tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./cluster/ -v -race -count=1`

Expected: ALL PASS

- [ ] **Step 22: Run natskv integration tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./cluster/clusterproviders/natskv/ -tags=integration -v -race -timeout 300s`

Expected: ALL PASS (existing tests + new TTL test)

- [ ] **Step 23: Run go vet on changed packages**

Run: `cd /home/cchamplin/development/protoactor-go && go vet ./cluster/ ./cluster/clusterproviders/natskv/`

Expected: No issues
