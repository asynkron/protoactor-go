# Cluster Production Readiness Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the cluster package production-ready by fixing critical bugs, restoring test coverage, hardening Consul + K8s providers, and updating documentation.

**Architecture:** Bottom-up fix-first approach. Fix correctness issues in core cluster code, then restore and expand tests, then harden providers, then improve core mechanisms, then update docs. Each phase builds on verified work from the previous phase.

**Tech Stack:** Go, protobuf, testcontainers, testify, Consul API, K8s client-go, Redis

---

## Phase 1: Critical Code Fixes

### Task 1: Fix Consul Provider Shutdown Race Condition

**Files:**
- Modify: `cluster/clusterproviders/consul/consul_provider.go:25-44`

**Step 1: Write test for concurrent shutdown safety**

Create test file `cluster/clusterproviders/consul/consul_provider_shutdown_test.go`:

```go
package consul

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProvider_ShutdownFlagIsAtomic(t *testing.T) {
	p := &Provider{}

	var wg sync.WaitGroup
	wg.Add(2)

	// Writer goroutine
	go func() {
		defer wg.Done()
		p.shutdown.Store(true)
	}()

	// Reader goroutine
	go func() {
		defer wg.Done()
		_ = p.shutdown.Load()
	}()

	wg.Wait()
	assert.True(t, p.shutdown.Load())
}
```

**Step 2: Run test to verify it fails (wrong type)**

Run: `cd cluster/clusterproviders/consul && go test -run TestProvider_ShutdownFlagIsAtomic -v -count=1`
Expected: FAIL - `p.shutdown.Store undefined (type bool has no field or method Store)`

**Step 3: Change `shutdown` and `deregistered` from `bool` to `atomic.Bool`**

In `consul_provider.go`, change the Provider struct (lines 25-44):

```go
type Provider struct {
	cluster            *cluster.Cluster
	deregistered       atomic.Bool
	shutdown           atomic.Bool
	// ... rest unchanged
}
```

Add `"sync/atomic"` to imports.

Then find-and-replace all usages:
- `p.shutdown = true` -> `p.shutdown.Store(true)`
- `p.shutdown = false` -> `p.shutdown.Store(false)`
- `p.shutdown` (in conditions) -> `p.shutdown.Load()`
- Same for `p.deregistered`

**Step 4: Run test to verify it passes**

Run: `cd cluster/clusterproviders/consul && go test -run TestProvider_ShutdownFlagIsAtomic -v -count=1`
Expected: PASS

**Step 5: Run all consul tests**

Run: `cd cluster/clusterproviders/consul && go test -v -count=1 -race -timeout 30s`
Expected: PASS (integration tests may skip without Consul)

**Step 6: Commit**

```bash
git add cluster/clusterproviders/consul/consul_provider.go cluster/clusterproviders/consul/consul_provider_shutdown_test.go
git commit -m "fix(consul): replace shutdown/deregistered bool with atomic.Bool for goroutine safety"
```

---

### Task 2: Fix K8s Provider Shutdown Race Condition

**Files:**
- Modify: `cluster/clusterproviders/k8s/k8s_provider.go:37-52`

**Step 1: Write test for concurrent shutdown safety**

Create test file `cluster/clusterproviders/k8s/k8s_provider_shutdown_test.go`:

```go
package k8s

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProvider_ShutdownFlagIsAtomic(t *testing.T) {
	p := &Provider{
		clusterPods: make(map[types.UID]*v1.Pod),
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		p.shutdown.Store(true)
	}()

	go func() {
		defer wg.Done()
		_ = p.shutdown.Load()
	}()

	wg.Wait()
	assert.True(t, p.shutdown.Load())
}
```

**Step 2: Run test to verify it fails**

Run: `cd cluster/clusterproviders/k8s && go test -run TestProvider_ShutdownFlagIsAtomic -v -count=1`
Expected: FAIL

**Step 3: Change `shutdown` from `bool` to `atomic.Bool`**

In `k8s_provider.go` line 50, change:

```go
type Provider struct {
	// ... existing fields ...
	shutdown    atomic.Bool
	cancelWatch context.CancelFunc
}
```

Add `"sync/atomic"` to imports. Update all usages of `p.shutdown`.

**Step 4: Run test and full test suite**

Run: `cd cluster/clusterproviders/k8s && go test -v -count=1 -race -timeout 30s`
Expected: PASS (k8s tests may skip if not in cluster)

**Step 5: Commit**

```bash
git add cluster/clusterproviders/k8s/
git commit -m "fix(k8s): replace shutdown bool with atomic.Bool for goroutine safety"
```

---

### Task 3: Fix Etcd Provider Shutdown Race + context.TODO()

**Files:**
- Modify: `cluster/clusterproviders/etcd/etcd_provider.go`

**Step 1: Write test for shutdown atomicity**

Same pattern as Tasks 1-2 for the etcd Provider struct.

**Step 2: Change `shutdown` to `atomic.Bool`**

Update the struct field and all usages.

**Step 3: Add context with cancellation for lifecycle operations**

Add a `ctx context.Context` and `cancel context.CancelFunc` field to the Provider struct. Initialize in `StartMember()`. Replace all `context.TODO()` with `p.ctx`. In `Shutdown()`, call `p.cancel()`.

**Step 4: Run tests**

Run: `cd cluster/clusterproviders/etcd && go test -v -count=1 -race -timeout 30s`

**Step 5: Commit**

```bash
git add cluster/clusterproviders/etcd/
git commit -m "fix(etcd): replace shutdown bool with atomic.Bool, replace context.TODO with cancellable context"
```

---

### Task 4: Fix ZooKeeper Provider Shutdown Race

**Files:**
- Modify: `cluster/clusterproviders/zk/zk_provider.go`

Same pattern as Tasks 1-3. Change `shutdown` bool to `atomic.Bool`, update all usages.

**Step 1-5:** Same as above.

**Commit:**
```bash
git add cluster/clusterproviders/zk/
git commit -m "fix(zk): replace shutdown bool with atomic.Bool for goroutine safety"
```

---

### Task 5: Fix Consensus TryResetConsensus No-Op

**Files:**
- Modify: `cluster/consensus.go:61-65`
- Test: `cluster/consensus_test.go` (create)

**Step 1: Write failing test**

Create `cluster/consensus_test.go`:

```go
package cluster

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGossipConsensusHandler_SetAndGet(t *testing.T) {
	h := NewGossipConsensusHandler()
	h.TrySetConsensus("value1")

	val, ok := h.TryGetConsensus(context.Background())
	assert.True(t, ok)
	assert.Equal(t, "value1", val)
}

func TestGossipConsensusHandler_Reset(t *testing.T) {
	h := NewGossipConsensusHandler()
	h.TrySetConsensus("value1")

	h.TryResetConsensus()

	_, ok := h.TryGetConsensus(context.Background())
	assert.False(t, ok, "consensus should be reset")
}

func TestGossipConsensusHandler_SetAfterReset(t *testing.T) {
	h := NewGossipConsensusHandler()
	h.TrySetConsensus("value1")
	h.TryResetConsensus()
	h.TrySetConsensus("value2")

	val, ok := h.TryGetConsensus(context.Background())
	assert.True(t, ok)
	assert.Equal(t, "value2", val)
}
```

**Step 2: Run test to verify Reset test fails**

Run: `cd cluster && go test -run TestGossipConsensusHandler_Reset -v -count=1`
Expected: FAIL - consensus is still true after reset

**Step 3: Implement TryResetConsensus**

In `consensus.go:61-65`, replace:

```go
func (hdl *gossipConsensusHandler) TryResetConsensus() {
	hdl.result.Lock()
	defer hdl.result.Unlock()

	hdl.result.consensus = false
	hdl.result.value = nil
}
```

**Step 4: Run tests**

Run: `cd cluster && go test -run TestGossipConsensusHandler -v -count=1`
Expected: All PASS

**Step 5: Commit**

```bash
git add cluster/consensus.go cluster/consensus_test.go
git commit -m "fix(cluster): implement TryResetConsensus to clear consensus state"
```

---

### Task 6: Fix PubSub Panic on Start Failure

**Files:**
- Modify: `cluster/pubsub.go:27-36`

**Step 1: Write test for Start returning error**

Create `cluster/pubsub_test.go`:

```go
package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPubSub_Start_DoesNotPanic(t *testing.T) {
	// Start on a fresh PubSub should not panic even if actor system
	// has issues - it should return an error instead.
	cp := newInmemoryProvider()
	c := newClusterForTest("test-pubsub-start", cp)

	// Starting member initializes the PubSub, which calls Start().
	// This should succeed without panic.
	err := c.StartMember()
	assert.NoError(t, err)
}
```

**Step 2: Change Start() to return error instead of panic**

In `pubsub.go`, change:

```go
// Start the PubSubMemberDeliveryActor. Returns an error if the actor
// cannot be spawned.
func (p *PubSub) Start() error {
	props := actor.PropsFromProducer(func() actor.Actor {
		return NewPubSubMemberDeliveryActor(p.cluster.Config.PubSubConfig.SubscriberTimeout, p.cluster.Logger())
	})
	_, err := p.cluster.ActorSystem.Root.SpawnNamed(props, PubSubDeliveryName)
	if err != nil {
		return fmt.Errorf("failed to start PubSub delivery actor: %w", err)
	}
	p.cluster.Logger().Info("Started Cluster PubSub")
	return nil
}
```

Add `"fmt"` to imports.

**Step 3: Update callers in cluster.go**

In `cluster.go:145`, change `c.PubSub.Start()` to:

```go
if err := c.PubSub.Start(); err != nil {
	return fmt.Errorf("failed to start PubSub: %w", err)
}
```

Same for `cluster.go:182` in `StartClient()`.

**Step 4: Run tests**

Run: `cd cluster && go test -v -count=1 -race -timeout 120s`
Expected: PASS

**Step 5: Commit**

```bash
git add cluster/pubsub.go cluster/cluster.go cluster/pubsub_test.go
git commit -m "fix(cluster): return error from PubSub.Start instead of panicking"
```

---

### Task 7: Fix Failing TestVirtualActorContextHasClusterIdentity

**Files:**
- Modify: `cluster/grain_context_test.go`

**Step 1: Run the test to observe the failure**

Run: `cd cluster && go test -run TestVirtualActorContextHasClusterIdentity -v -count=1 -timeout 30s`
Expected: FAIL with "probe context is nil" recovery loop

**Step 2: Diagnose and fix**

The test creates a probe but uses `probe.Context()` inside the actor func before the probe is spawned (line 36). The probe needs to be spawned before the grain is activated.

Fix by restructuring: spawn the probe first, then activate the grain, or use a channel-based approach to capture the ClusterIdentity instead of relying on probe context:

```go
func TestVirtualActorContextHasClusterIdentity(t *testing.T) {
	cp := newInmemoryProvider()

	kindName := "kind"
	actorID := "myactor"

	resultCh := make(chan any, 10)

	kind := NewKind(kindName, actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *ClusterInit:
			resultCh <- GetClusterIdentity(ctx)
			resultCh <- GetCluster(ctx.ActorSystem())
		}
	}))

	c := newClusterForTest("mycluster", cp, WithKinds(kind))
	err := c.StartMember()
	assert.NoError(t, err)
	cp.publishClusterTopologyEvent()

	pid := c.Get(actorID, kindName)
	assert.NotNil(t, pid)

	select {
	case val := <-resultCh:
		ci, ok := val.(*ClusterIdentity)
		assert.True(t, ok)
		assert.Equal(t, actorID, ci.Identity)
		assert.Equal(t, kindName, ci.Kind)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for ClusterIdentity")
	}

	select {
	case val := <-resultCh:
		cl, ok := val.(*Cluster)
		assert.True(t, ok)
		assert.Equal(t, c, cl)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Cluster")
	}
}
```

**Step 3: Run the test**

Run: `cd cluster && go test -run TestVirtualActorContextHasClusterIdentity -v -count=1 -timeout 30s`
Expected: PASS

**Step 4: Commit**

```bash
git add cluster/grain_context_test.go
git commit -m "fix(cluster): fix TestVirtualActorContextHasClusterIdentity probe timing issue"
```

---

### Task 8: Document Gossip Ack Decision

**Files:**
- Modify: `cluster/gossip_actor.go:122-149`

**Step 1: Replace ambiguous comment with documented decision**

Replace the block at lines 122-149 with a clear architectural comment:

```go
	// Gossip acks are intentionally disabled. The gossip protocol uses
	// eventual consistency and fan-out to ensure state propagation.
	// Re-sending state on the next gossip round handles message loss.
	// Acks were disabled because:
	// 1. They add latency to every gossip exchange
	// 2. The CommitOffsets pattern can cause state to be held too long
	// 3. Fan-out + periodic re-send provides adequate reliability
	//
	// If gossip delivery issues are observed in production, consider:
	// - Increasing GossipFanOut (default 3)
	// - Decreasing GossipInterval (default 300ms)
	// - Adding a gossip convergence health metric
	ctx.Respond(&GossipResponse{})
```

Remove the dead commented-out code (lines 125-149).

**Step 2: Run tests**

Run: `cd cluster && go test -v -count=1 -race -timeout 120s`
Expected: PASS

**Step 3: Commit**

```bash
git add cluster/gossip_actor.go
git commit -m "docs(gossip): document ack-disabled decision, remove dead code"
```

---

## Phase 2: Test Restoration

### Task 9: Re-enable TestPublishRaceCondition

**Files:**
- Modify: `cluster/member_list_test.go:11-42`

**Step 1: Uncomment and update the test**

Rewrite using current API:

```go
func TestPublishRaceCondition(t *testing.T) {
	cp := newInmemoryProvider()
	c := newClusterForTest("mycluster", cp)

	rounds := 1000
	var wg sync.WaitGroup
	wg.Add(2 * rounds)

	go func() {
		for i := 0; i < rounds; i++ {
			c.MemberList.UpdateClusterTopology([]*Member{
				{Id: "1", Host: "localhost", Port: 1},
				{Id: "2", Host: "localhost", Port: 2},
			})
			c.MemberList.UpdateClusterTopology([]*Member{
				{Id: "1", Host: "localhost", Port: 1},
			})
			wg.Done()
		}
	}()

	go func() {
		for i := 0; i < rounds; i++ {
			s := c.ActorSystem.EventStream.Subscribe(func(evt any) {})
			c.ActorSystem.EventStream.Unsubscribe(s)
			wg.Done()
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(5 * time.Second):
		t.Error("Should not run into a timeout")
	}
}
```

Add required imports: `"sync"`, `"time"`.

**Step 2: Run with race detector**

Run: `cd cluster && go test -run TestPublishRaceCondition -v -count=1 -race -timeout 30s`
Expected: PASS with no race detected

**Step 3: Commit**

```bash
git add cluster/member_list_test.go
git commit -m "test(cluster): re-enable TestPublishRaceCondition with current API"
```

---

### Task 10: Re-enable Pub/Sub Test Suite

**Files:**
- Modify: `cluster/cluster_test_tool/pubsub_test.go`
- Modify: `cluster/cluster_test_tool/pubsub_member_test.go`

**Step 1: Verify the PubSubClusterFixture works**

Run: `cd cluster/cluster_test_tool && go test -run TestPubSubWorksWithDefaultTopicRegistration -v -count=1 -timeout 60s`
Expected: PASS (this test already works)

**Step 2: Uncomment pubsub_test.go**

Remove all `//` comment prefixes from the test file. Add the `TestPubSubSuite` runner:

```go
func TestPubSubSuite(t *testing.T) {
	suite.Run(t, new(PubSubTestSuite))
}
```

**Step 3: Run the re-enabled tests**

Run: `cd cluster/cluster_test_tool && go test -run TestPubSubSuite -v -count=1 -timeout 120s`

If tests fail, diagnose and fix the fixture or test setup. Common issues:
- Timing: increase `DefaultWaitTimeout`
- API changes: update method signatures
- Fixture initialization: ensure `PubSubClusterFixture.Initialize()` completes

**Step 4: Uncomment pubsub_member_test.go**

Same process. Add test runner:

```go
func TestPubSubMemberSuite(t *testing.T) {
	suite.Run(t, new(PubSubMemberTestSuite))
}
```

**Step 5: Run all pub/sub tests**

Run: `cd cluster/cluster_test_tool && go test -v -count=1 -timeout 180s`
Expected: All PASS

**Step 6: Commit**

```bash
git add cluster/cluster_test_tool/pubsub_test.go cluster/cluster_test_tool/pubsub_member_test.go
git commit -m "test(pubsub): re-enable pub/sub test suites"
```

---

### Task 11: Fix Flaky Time-Based Test Patterns

**Files:**
- Modify: `cluster/cluster_test_tool/pubsub_cluster_fixture.go:192` (4s sleep)

**Step 1: Replace fixed sleep in timeoutSubscriberProps**

In `pubsub_cluster_fixture.go:192`, the 4-second sleep is intentional (tests subscriber timeout). Leave this one - it's testing timeout behavior. But add a comment:

```go
// Intentional delay: exceeds the configured SubscriberTimeout (2s) to test timeout handling.
time.Sleep(time.Second * 4)
```

**Step 2: Review and fix `WaitUntil` timeout multiplier**

In `pubsub_cluster_fixture.go:88`, `DefaultWaitTimeout*1000` seems like a bug (would be 5000 seconds if DefaultWaitTimeout is 5s). Fix to a reasonable value:

```go
WaitUntil(p.t, func() bool {
	p.DeliveriesLock.RLock()
	defer p.DeliveriesLock.RUnlock()
	return len(p.Deliveries) == numMessages*len(subscriberIds)
}, "All messages should be delivered", 30*time.Second)
```

**Step 3: Run tests**

Run: `cd cluster/cluster_test_tool && go test -v -count=1 -timeout 120s`
Expected: PASS

**Step 4: Commit**

```bash
git add cluster/cluster_test_tool/pubsub_cluster_fixture.go
git commit -m "fix(test): fix flaky timeout patterns in pubsub fixtures"
```

---

## Phase 3: Test Expansion

### Task 12: Add MemberSet Unit Tests

**Files:**
- Create: `cluster/members_test.go`

**Step 1: Write tests**

```go
package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMemberSet_Except(t *testing.T) {
	a := NewMemberSet(Members{
		{Id: "1", Host: "h1", Port: 1},
		{Id: "2", Host: "h2", Port: 2},
		{Id: "3", Host: "h3", Port: 3},
	})
	b := NewMemberSet(Members{
		{Id: "2", Host: "h2", Port: 2},
	})

	result := a.Except(b)
	assert.Equal(t, 2, result.Len())
	assert.True(t, result.ContainsID("1"))
	assert.True(t, result.ContainsID("3"))
	assert.False(t, result.ContainsID("2"))
}

func TestMemberSet_Union(t *testing.T) {
	a := NewMemberSet(Members{
		{Id: "1", Host: "h1", Port: 1},
	})
	b := NewMemberSet(Members{
		{Id: "2", Host: "h2", Port: 2},
	})

	result := a.Union(b)
	assert.Equal(t, 2, result.Len())
	assert.True(t, result.ContainsID("1"))
	assert.True(t, result.ContainsID("2"))
}

func TestMemberSet_ExceptIds(t *testing.T) {
	ms := NewMemberSet(Members{
		{Id: "1", Host: "h1", Port: 1},
		{Id: "2", Host: "h2", Port: 2},
		{Id: "3", Host: "h3", Port: 3},
	})

	result := ms.ExceptIds([]string{"1", "3"})
	assert.Equal(t, 1, result.Len())
	assert.True(t, result.ContainsID("2"))
}

func TestMemberSet_Equals(t *testing.T) {
	a := NewMemberSet(Members{
		{Id: "1", Host: "h1", Port: 1},
	})
	b := NewMemberSet(Members{
		{Id: "1", Host: "h1", Port: 1},
	})

	assert.True(t, a.Equals(b))
}

func TestMemberSet_Empty(t *testing.T) {
	ms := NewMemberSet(Members{})
	assert.Equal(t, 0, ms.Len())
}
```

**Step 2: Run tests**

Run: `cd cluster && go test -run TestMemberSet -v -count=1`
Expected: All PASS

**Step 3: Commit**

```bash
git add cluster/members_test.go
git commit -m "test(cluster): add MemberSet unit tests"
```

---

### Task 13: Add Cluster Shutdown Test

**Files:**
- Modify: `cluster/cluster_test.go`

**Step 1: Write test**

```go
func TestCluster_Shutdown_Graceful(t *testing.T) {
	cp := newInmemoryProvider()
	c := newClusterForTest("test-shutdown", cp)

	err := c.StartMember()
	assert.NoError(t, err)

	// Should not panic
	assert.NotPanics(t, func() {
		c.Shutdown(true)
	})
}

func TestCluster_Shutdown_NotGraceful(t *testing.T) {
	cp := newInmemoryProvider()
	c := newClusterForTest("test-shutdown-fast", cp)

	err := c.StartMember()
	assert.NoError(t, err)

	assert.NotPanics(t, func() {
		c.Shutdown(false)
	})
}
```

**Step 2: Run tests**

Run: `cd cluster && go test -run TestCluster_Shutdown -v -count=1 -timeout 30s`
Expected: PASS

**Step 3: Commit**

```bash
git add cluster/cluster_test.go
git commit -m "test(cluster): add Cluster.Shutdown tests"
```

---

### Task 14: Add K8s Provider Fake Client Tests

**Files:**
- Create: `cluster/clusterproviders/k8s/k8s_provider_unit_test.go`

**Step 1: Write unit tests using fake K8s client**

```go
package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

func TestProvider_New_WithFakeClient(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	p := &Provider{
		client:      clientset,
		clusterPods: make(map[types.UID]*v1.Pod),
		namespace:   "default",
	}
	assert.NotNil(t, p)
}

func TestProvider_mapPodsToMembers(t *testing.T) {
	p := &Provider{
		clusterName: "test-cluster",
		clusterPods: make(map[types.UID]*v1.Pod),
	}

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:  types.UID("test-uid"),
			Name: "test-pod",
			Labels: map[string]string{
				LabelCluster:  "test-cluster",
				LabelPort:     "8080",
				LabelMemberID: "member-1",
				LabelKinds:    "kind1",
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
			PodIP: "10.0.0.1",
			Conditions: []v1.PodCondition{
				{Type: v1.PodReady, Status: v1.ConditionTrue},
			},
		},
	}

	p.clusterPods[pod.UID] = pod
	// Verify the pod was stored
	assert.Equal(t, 1, len(p.clusterPods))
}
```

**Step 2: Run tests**

Run: `cd cluster/clusterproviders/k8s && go test -run TestProvider_ -v -count=1 -timeout 30s`
Expected: PASS

**Step 3: Commit**

```bash
git add cluster/clusterproviders/k8s/k8s_provider_unit_test.go
git commit -m "test(k8s): add unit tests with fake K8s client"
```

---

## Phase 4: Provider Hardening

### Task 15: Add Consul Provider Context Cancellation

**Files:**
- Modify: `cluster/clusterproviders/consul/consul_provider.go`

**Step 1: Add context field to Provider struct**

```go
type Provider struct {
	// ... existing fields ...
	ctx    context.Context
	cancel context.CancelFunc
}
```

**Step 2: Initialize context in init()**

In the `init()` method, add:

```go
p.ctx, p.cancel = context.WithCancel(context.Background())
```

**Step 3: Use context in Shutdown()**

In `Shutdown()`, call `p.cancel()` before other cleanup.

**Step 4: Pass context to blocking Consul API calls**

Replace any blocking Consul calls that don't use context with `p.ctx`.

**Step 5: Run integration tests**

Run: `cd cluster/clusterproviders/consul && go test -v -count=1 -race -timeout 60s`

**Step 6: Commit**

```bash
git add cluster/clusterproviders/consul/
git commit -m "fix(consul): add context cancellation for clean shutdown"
```

---

### Task 16: Fix K8s Provider Watch Goroutine Leak

**Files:**
- Modify: `cluster/clusterproviders/k8s/k8s_provider.go`

**Step 1: Ensure cancelWatch is always called in Shutdown**

Verify that `Shutdown()` calls `p.cancelWatch()` if it's not nil. Add a nil check:

```go
func (p *Provider) Shutdown(graceful bool) error {
	p.shutdown.Store(true)
	if p.cancelWatch != nil {
		p.cancelWatch()
	}
	// ... existing cleanup ...
}
```

**Step 2: Add done channel for watch goroutine**

Add `watchDone chan struct{}` to Provider. Signal it when watch loop exits.
Wait on it in Shutdown() with a timeout.

**Step 3: Run tests**

Run: `cd cluster/clusterproviders/k8s && go test -v -count=1 -race -timeout 30s`

**Step 4: Commit**

```bash
git add cluster/clusterproviders/k8s/
git commit -m "fix(k8s): fix watch goroutine leak on shutdown"
```

---

## Phase 5: Core Mechanism Improvements

### Task 17: Add PID Cache TTL

**Files:**
- Modify: `cluster/pid_cache.go`
- Create: `cluster/pid_cache_test.go` (expand existing)

**Step 1: Write failing test**

```go
func TestPidCache_EntryExpires(t *testing.T) {
	cache := NewPidCacheWithTTL(100 * time.Millisecond)
	pid := &actor.PID{Address: "localhost", Id: "test"}

	cache.Set("id1", "kind1", pid)

	got, ok := cache.Get("id1", "kind1")
	assert.True(t, ok)
	assert.Equal(t, pid, got)

	time.Sleep(150 * time.Millisecond)

	_, ok = cache.Get("id1", "kind1")
	assert.False(t, ok, "entry should have expired")
}
```

**Step 2: Implement TTL-based cache**

Add a `ttl` field to `PidCacheValue`. Wrap stored values with a timestamp. On `Get()`, check if the entry has expired.

```go
type pidCacheEntry struct {
	pid       *actor.PID
	createdAt time.Time
}

type PidCacheValue struct {
	cache cmap.ConcurrentMap
	ttl   time.Duration
}

func NewPidCacheWithTTL(ttl time.Duration) *PidCacheValue {
	return &PidCacheValue{
		cache: cmap.New(),
		ttl:   ttl,
	}
}
```

Update `Get()` to check `time.Since(entry.createdAt) > c.ttl`.
Keep `NewPidCache()` backward compatible with zero TTL (no expiry).

**Step 3: Run tests**

Run: `cd cluster && go test -run TestPidCache -v -count=1`
Expected: All PASS

**Step 4: Commit**

```bash
git add cluster/pid_cache.go cluster/pid_cache_test.go
git commit -m "feat(cluster): add TTL-based expiration to PID cache"
```

---

### Task 18: Add Duplicate Spawn Prevention

**Files:**
- Modify: `cluster/identitylookup/disthash/placement_actor.go`

**Step 1: Add spawn-in-progress tracking**

Add a map to track in-flight activation requests:

```go
type placementActor struct {
	cluster      *cluster.Cluster
	spawning     map[string]bool // identity+kind -> in progress
	spawningLock sync.Mutex
}
```

**Step 2: Check before spawning**

Before processing `ActivationRequest`, check if spawn is already in progress. If so, wait or return existing result.

**Step 3: Run tests**

Run: `cd cluster/identitylookup/disthash && go test -v -count=1 -race -timeout 60s`

**Step 4: Commit**

```bash
git add cluster/identitylookup/disthash/
git commit -m "fix(disthash): prevent duplicate concurrent grain spawns"
```

---

## Phase 6: Documentation

### Task 19: Update Cluster README

**Files:**
- Modify: `cluster/README.MD`

**Step 1: Replace "alpha" with production status**

Update the README to:
- Remove "alpha" designation
- Show current `NewCluster()` API with `Configure()`
- Add quick-start examples for Consul and K8s
- Add configuration reference table with all options and defaults
- Document gossip parameters and their effects
- Document pub/sub limitations (at-most-once delivery, no persistence)

**Step 2: Commit**

```bash
git add cluster/README.MD
git commit -m "docs(cluster): update README for production readiness"
```

---

### Task 20: Add Provider Comparison Documentation

**Files:**
- Create: `cluster/clusterproviders/README.md`

**Step 1: Write provider comparison**

Document:
- When to use Consul (service mesh, health checks, general purpose)
- When to use K8s (Kubernetes-native, pod-based, no extra infra)
- When to use etcd (already have etcd, leadership election)
- When to use ZK (legacy systems, strong consistency needs)
- When to use automanaged (development, testing only)

Include setup instructions for Consul and K8s.

**Step 2: Commit**

```bash
git add cluster/clusterproviders/README.md
git commit -m "docs(providers): add provider comparison and setup guide"
```

---

### Task 21: Add Architecture Documentation

**Files:**
- Create: `cluster/doc.go` (update existing)

**Step 1: Update the package doc.go**

Expand the existing doc.go with:
- Gossip protocol overview (eventual consistency, fan-out, heartbeat)
- Grain placement algorithm (rendezvous hashing for disthash, storage-based for Redis)
- Pub/sub model (topic actors, delivery actors, at-most-once semantics)
- Member lifecycle (join -> healthy -> suspected -> failed/left)
- Configuration tuning guidance

**Step 2: Commit**

```bash
git add cluster/doc.go
git commit -m "docs(cluster): add architecture documentation to package doc"
```

---

### Task 22: Run Full Test Suite and Verify

**Step 1: Run all cluster tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./cluster/... -v -count=1 -race -timeout 300s`

**Step 2: Run provider integration tests (if infrastructure available)**

Run: `cd /home/cchamplin/development/protoactor-go && go test -tags integration ./cluster/clusterproviders/consul/... -v -count=1 -timeout 120s`

**Step 3: Run benchmarks**

Run: `cd /home/cchamplin/development/protoactor-go/cluster && go test -bench=. -benchmem -count=1`

**Step 4: Verify no regressions**

Compare benchmark results with existing baselines in `docs/benchmarks/`.

**Step 5: Final commit**

```bash
git commit --allow-empty -m "chore(cluster): verify full test suite passes for production readiness"
```
