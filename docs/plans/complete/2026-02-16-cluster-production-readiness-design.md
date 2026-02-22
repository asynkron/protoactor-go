# Cluster Production Readiness Design

**Date:** 2026-02-16
**Approach:** Bottom-Up Fix-First
**Provider Focus:** Consul + Kubernetes (others best-effort)

## Background

The cluster package README states "alpha" status. A thorough analysis of the codebase
identified gaps in correctness, test coverage, provider hardening, and documentation
that must be addressed before production use.

## Analysis Summary

### Current State

The cluster package has significant implementation depth:
- 5 cluster providers (consul, etcd, zk, k8s, automanaged)
- 3 identity lookup strategies (disthash, redis, in-memory)
- Gossip-based state dissemination
- Virtual actor (grain) placement with PID caching
- Cluster-wide pub/sub with batching
- Gossip-based "consensus" checking
- Benchmarks for hot paths

### Critical Issues Found

| Issue | Location | Severity |
|-------|----------|----------|
| Race conditions on shutdown flags | All 5 providers | P0 |
| Gossip acks disabled | gossip_actor.go | P0 |
| context.TODO() blocking ops | etcd_provider.go (~8 locations) | P0 |
| Failing test regression | grain_context_test.go | P0 |
| Consensus reset is no-op | consensus.go:61-65 | P1 |
| PID cache no TTL | pid_cache.go | P1 |
| Pub/sub test suites disabled | pubsub_test.go, pubsub_member_test.go | P0 |
| Race condition test disabled | member_list_test.go | P0 |
| Flaky time-based test patterns | Multiple files | P1 |
| Panics instead of error returns | cluster.go:65, pubsub.go:33 | P1 |
| TOCTOU in disthash manager | disthash/manager.go:92-124 | P1 |
| Duplicate spawn possible | disthash/placement_actor.go:91-136 | P1 |

---

## Section 1: Critical Code Fixes

### 1.1 Race Conditions on Provider Shutdown Flags

**Affected:** consul, etcd, zk, k8s, automanaged providers
**Problem:** `p.shutdown` bool read/written across goroutines without synchronization.
**Fix:** Replace `bool` with `atomic.Bool`. Add done channel for goroutine coordination.

### 1.2 Gossip Acknowledgments Disabled

**File:** `gossip_actor.go`
**Problem:** Ack mechanism commented out ("turn off acking for now"). Gossip messages
can be silently lost with no detection.
**Fix:** Evaluate whether to re-enable acks or add gossip delivery monitoring. If
intentionally disabled, document rationale and add health metrics.

### 1.3 Etcd Provider context.TODO() Usage

**File:** `etcd_provider.go` (~8 locations: lines 124, 186, 210, 249, 257, 317, 381, 458)
**Problem:** No timeout enforcement. `keepAliveForever()` can hang indefinitely.
**Fix:** Replace with proper context derived from cluster shutdown context.

### 1.4 Fix Failing Test: TestVirtualActorContextHasClusterIdentity

**File:** `grain_context_test.go`
**Problem:** Fails with "probe context is nil" and repeated recovery attempts.
**Fix:** Investigate root cause in grain context initialization. Fix code or test setup.

### 1.5 Consensus TryResetConsensus Implementation

**File:** `consensus.go:61-65`
**Problem:** No-op function means consensus is never invalidated after topology changes.
**Fix:** Implement proper reset that clears consensus state when topology diverges.

### 1.6 PID Cache TTL/Invalidation

**File:** `pid_cache.go`
**Problem:** No expiration. Stale PIDs cause errors after member failures.
**Fix:** Add configurable TTL-based expiration (default 30s). Invalidate on topology change.

---

## Section 2: Test Coverage Restoration & Expansion

### 2.1 Re-enable Disabled Pub/Sub Test Suites

**Files:** `pubsub_test.go` (~206 lines), `pubsub_member_test.go` (~210 lines)
**Problem:** Entirely commented out. Tests cover: single messages, batching, unsubscribe,
PID subscriptions, member leave handling.
**Fix:** Fix the `PubSubClusterFixture` (which depends on in-memory cluster infrastructure),
then uncomment and update tests to match current API.

### 2.2 Re-enable TestPublishRaceCondition

**File:** `member_list_test.go`
**Problem:** Uses old 4-arg `Configure()` signature.
**Fix:** Rewrite using current `Configure(name, provider, lookup, remoteConfig)` API.

### 2.3 Fix Flaky Test Patterns

**Affected files:**
- `automanaged/member_list_broadcast_test.go` (500ms sleep + 1s timeout)
- `pubsub_cluster_fixture.go` (4s sleep)
- `pubsub_producer_test.go` (50ms, 500ms, 1s sleeps)

**Fix:** Replace `time.Sleep()` with `WaitUntil` condition-based waits or channel
synchronization.

### 2.4 Missing Unit Tests for Public APIs

Add tests for:
- `Cluster.StartClient()` - never tested
- `Cluster.Shutdown(graceful bool)` - only used in cleanup, not verified
- `Cluster.GetBlockedMembers()` - no tests
- `Cluster.VirtualActorCount()` - no tests
- `MemberSet` operations: `Except()`, `Union()`, `ExceptIds()`
- `GossipConsensusHandler` full lifecycle (set, get, reset)

### 2.5 Consul Provider: Expanded Integration Tests

**Current coverage:** Basic StartMember + topology update only.

**Add tests for:**
- Multi-member join/leave/rejoin scenarios
- Graceful vs forced shutdown behavior
- TTL expiration and recovery
- Client-only mode (`StartClient`)
- Error recovery on transient Consul failures

### 2.6 K8s Provider: Integration Test Infrastructure

**Current:** Tests skip if not in K8s. No mock alternative.

**Add:**
- Fake K8s client tests using `k8s.io/client-go/fake`
- Pod watch event simulation
- Label-based discovery verification
- Namespace handling edge cases (missing file, empty namespace)

### 2.7 Core Cluster Integration Tests

**Add tests for:**
- Multi-node gossip convergence under load
- Grain placement with topology changes
- Pub/sub delivery across cluster members
- Member failure detection and recovery
- Concurrent grain activation race detection

---

## Section 3: Provider Hardening (Consul + K8s)

### 3.1 Consul Provider

- **Shutdown:** `atomic.Bool` + done channel for clean goroutine lifecycle
- **Goroutine tracking:** `sync.WaitGroup` for `monitorMemberStatusChanges`
- **Context:** Proper context with cancellation for blocking Consul API calls
- **Error recovery:** Exponential backoff for transient Consul failures
- **Observability:** Metrics for TTL refresh latency, member count, topology changes

### 3.2 K8s Provider

- **Watch lifecycle:** Fix `watchPods()` goroutine leak on context cancellation
- **Async confirmation:** Request-response for `registerMemberAsync`/`startWatchingClusterAsync`
- **Namespace:** Fail explicitly if namespace file missing (not silent empty string)
- **Shutdown:** Same `atomic.Bool` + done channel pattern
- **Timeouts:** Configurable timeout for `replacePodLabels()` API calls

### 3.3 Provider Interface Contract Tests

Create a shared conformance test suite that any `ClusterProvider` must pass:
- StartMember, StartClient
- Member discovery and topology updates
- Graceful shutdown
- Error recovery patterns

---

## Section 4: Core Mechanism Improvements

### 4.1 Gossip Reliability

- Re-enable ack mechanism or add delivery monitoring
- Add gossip health metric (convergence percentage within N seconds)
- Add configurable gossip state size limits

### 4.2 Placement Safety

- Duplicate spawn prevention via spawn-in-progress map in `placement_actor.go`
- Fix TOCTOU in disthash manager (re-check topology after RPC)
- PID cache TTL with configurable duration

### 4.3 Pub/Sub Robustness

- Per-subscriber delivery timeout (prevent one slow subscriber blocking all)
- Delivery success/failure metrics
- Clear documentation of limitations (no persistence, no ordering, at-most-once)

### 4.4 Error Handling

- Replace `panic()` in `cluster.go:65` and `pubsub.go:33` with error returns
- Structured error types for common failures
- Correlation IDs for cluster requests

---

## Section 5: Documentation

### 5.1 Cluster README Update

- Remove "alpha" designation (after fixes verified)
- Show current `NewCluster()` API
- Quick-start for Consul and K8s
- Configuration reference with all options and defaults

### 5.2 Provider Documentation

- Per-provider README with setup and operational guidance
- Provider comparison table (when to use which)
- Provider-specific failure modes

### 5.3 Architecture Documentation

- Gossip protocol overview with consistency guarantees
- Grain placement algorithm
- Pub/sub model with clear limitations
- Member lifecycle diagram

### 5.4 Operations Guide

- Configuration tuning for different cluster sizes
- Monitoring and alerting recommendations
- Troubleshooting common issues

---

## Execution Order

1. **Phase 1 - Critical Fixes** (Section 1): Fix all P0 code bugs
2. **Phase 2 - Test Restoration** (Section 2.1-2.3): Re-enable disabled tests, fix flaky patterns
3. **Phase 3 - Test Expansion** (Section 2.4-2.7): Add missing unit + integration tests
4. **Phase 4 - Provider Hardening** (Section 3): Harden Consul + K8s
5. **Phase 5 - Core Improvements** (Section 4): Gossip, placement, pub/sub improvements
6. **Phase 6 - Documentation** (Section 5): Full documentation pass

Each phase builds on verified correctness from the previous phase.
