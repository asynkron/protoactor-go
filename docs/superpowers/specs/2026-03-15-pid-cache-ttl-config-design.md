# PID Cache TTL Configuration

## Problem

The cluster's PID cache (`Cluster.PidCache`) is constructed via `NewPidCache()` which never expires entries. When a node crashes and gossip or topology events are delayed, stale PIDs persist in the local cache. Every `cluster.Get()` call returns the stale PID and routes to the dead node, causing timeouts until `ClusterTopology.Left` fires and triggers `RemoveByMember()`.

## Solution

Expose `PidCacheTTL` as a cluster config option so users can opt into TTL-based expiration as defense-in-depth. Default remains zero (no expiry) for backward compatibility.

## Design

### Production Code

Three changes:

1. **`cluster/config.go`** — Add field:
   ```go
   PidCacheTTL time.Duration
   ```

2. **`cluster/config_opts.go`** — Add option:
   ```go
   func WithPidCacheTTL(ttl time.Duration) ConfigOption {
       return func(c *Config) {
           c.PidCacheTTL = ttl
       }
   }
   ```

3. **`cluster/cluster.go`** — Change line 62 from:
   ```go
   c.PidCache = NewPidCache()
   ```
   to:
   ```go
   c.PidCache = NewPidCacheWithTTL(config.PidCacheTTL)
   ```
   `NewPidCacheWithTTL(0)` behaves identically to `NewPidCache()` (no expiry), so this is backward compatible.

### TDD Test Plan

#### Unit Tests (`cluster/`)

**Test 1: `TestWithPidCacheTTL_SetsConfigField`**
- Create a config with `WithPidCacheTTL(30 * time.Second)`
- Assert `config.PidCacheTTL == 30 * time.Second`

**Test 2: `TestNewCluster_DefaultPidCacheTTL`**
- Create a cluster with no PidCacheTTL option
- Set a PID in the cache, wait briefly, verify it's still present
- Confirms zero-TTL (no expiry) default behavior

**Test 3: `TestNewCluster_WithPidCacheTTL`**
- Create a cluster with `WithPidCacheTTL(100ms)`
- Set a PID in the cache, verify it's present
- Wait >100ms, verify it's gone
- Confirms the config option wires through to cache construction

#### Integration Test (`cluster/clusterproviders/natskv/`)

**Test 4: `TestPidCacheTTL_ExpiresStaleActivation`**
- Start NATS via testcontainers
- Start two cluster members with `WithPidCacheTTL(2s)` and short natskv TTLs
- Activate a grain on node 2 via `cluster.Request()` from node 1 (populates node 1's PID cache)
- Crash node 2 (no graceful shutdown — simulates real failure)
- Wait for TTL expiry
- Verify node 1's PID cache no longer returns the stale entry
- Verify subsequent `cluster.Request()` triggers fresh activation on node 1

### No Validation

No validation on the TTL value. Zero means no expiry (default). Any positive duration is valid. Negative durations cause instant expiry — a user footgun but consistent with how Go `time.Duration` works elsewhere in the config and not worth special-casing.

## Scope

- No default change — fully backward compatible, opt-in only
- No new dependencies
- Reuses existing `NewPidCacheWithTTL()` which is already implemented and tested
