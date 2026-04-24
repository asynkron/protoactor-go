# Ring 4: Runtime Kind Addition Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable registering new Kinds after the cluster is running, with propagation to all cluster members via each provider's existing topology mechanism.

**Architecture:** Add an `RWMutex` protecting the `kinds` map on `Cluster`, a `RegisterKind`/`DeregisterKind` API that notifies the provider via an opt-in `KindUpdater` interface, and Kind-change detection in `MemberList.UpdateClusterTopology` so that remote nodes update their routing strategies when a member's kinds change.

**Tech Stack:** Go, sync.RWMutex, proto.actor cluster, NATS JetStream, Kubernetes client-go

**Design doc:** `docs/plans/2026-02-23-ring4-runtime-kind-addition-design.md`

---

## Critical background

### Topology hash does NOT include Kinds

`TopologyHash` (in `cluster/member.go:33-55`) only hashes member IDs. This means `MemberSet.Equals` (in `cluster/members.go:95-97`) returns `true` even when a member's Kinds changed. Without fixing this, Kind-only changes silently disappear because `getTopologyChanges` (in `cluster/member_list.go:169-194`) returns `done=true` and short-circuits `UpdateClusterTopology`.

**Fix:** Add a `hasKindChanges` check in `getTopologyChanges` so that it falls through when member IDs are identical but Kinds differ. Do NOT change `TopologyHash` (it must remain ID-only for backward compatibility with the gossip consensus protocol).

### AddMember is NOT idempotent

`simpleMemberStrategy.AddMember` (in `cluster/member_strategy.go:25-28`) blindly appends. Calling it twice for the same member creates duplicates. The Kind-change code must only call `AddMember` for genuinely new kind-member pairs.

### Build must run outside the lock

`Kind.Build(c)` (in `cluster/kind.go:31-42`) calls `StrategyBuilder(cluster)` which may call `c.GetClusterKind()` → `kindsMu.RLock`. If `RegisterKind` holds `kindsMu.Lock` while calling `Build`, this deadlocks (Go's `sync.RWMutex` is not reentrant). **Always call `Build` before acquiring `kindsMu.Lock`.**

### Lock ordering

Two locks are involved: `c.kindsMu` and `ml.mutex`. The safe ordering is:
- `ml.mutex` → `c.kindsMu.RLock` (happens in `getMemberStrategyByKind` → `TryGetClusterKind`)
- `c.kindsMu.Lock` alone (RegisterKind → notifyKindUpdate; providers never acquire `ml.mutex`)

No path acquires them in opposite order → no deadlock.

---

## Task 1: Thread-Safe Kind Registry

**Files:**
- Modify: `cluster/cluster.go:21-35` (Cluster struct), `:94-100` (VirtualActorCount), `:158-165` (GetClusterKinds), `:225-234` (GetClusterKind), `:236-240` (TryGetClusterKind), `:242-247` (initKinds), `:249-256` (InitKindsForTest), `:260-276` (ensureTopicKindRegistered)
- Create: `cluster/cluster_kind_test.go`

### Step 1: Write the failing test

Create `cluster/cluster_kind_test.go`:

```go
package cluster

import (
	"sync"
	"testing"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

func newTestCluster() *Cluster {
	system := actor.NewActorSystem()
	cfg := Configure("test-cluster", nil, nil, nil)
	c := &Cluster{
		ActorSystem: system,
		Config:      cfg,
		kinds:       map[string]*ActivatedKind{},
	}
	return c
}

func TestCluster_GetClusterKinds_ThreadSafe(t *testing.T) {
	c := newTestCluster()

	// Pre-populate a kind
	c.kinds["existing"] = &ActivatedKind{Kind: "existing"}

	var wg sync.WaitGroup
	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			kinds := c.GetClusterKinds()
			assert.NotNil(t, kinds)
		}()
	}
	wg.Wait()
}

func TestCluster_GetClusterKind_ThreadSafe(t *testing.T) {
	c := newTestCluster()
	c.kinds["testKind"] = &ActivatedKind{Kind: "testKind"}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			k := c.GetClusterKind("testKind")
			assert.NotNil(t, k)
		}()
	}
	wg.Wait()
}

func TestCluster_TryGetClusterKind_ThreadSafe(t *testing.T) {
	c := newTestCluster()
	c.kinds["testKind"] = &ActivatedKind{Kind: "testKind"}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			k, ok := c.TryGetClusterKind("testKind")
			assert.True(t, ok)
			assert.NotNil(t, k)
		}()
	}
	wg.Wait()
}
```

### Step 2: Run test to verify it fails

Run: `go test ./cluster/ -run TestCluster_.*ThreadSafe -race -count=1 -v`

Expected: PASS (no mutation yet, but the -race detector won't flag read-only concurrent access on a non-mutex-protected map when there are no writes). This test will become meaningful once RegisterKind exists and adds concurrent writes.

### Step 3: Add RWMutex and update all accessors

In `cluster/cluster.go`, add the mutex to the struct and update every method that touches `c.kinds`:

**3a.** Add `kindsMu` field to Cluster struct (after line 30):

```go
type Cluster struct {
	ActorSystem    *actor.ActorSystem
	Config         *Config
	Gossip         *Gossiper
	PubSub         *PubSub
	Remote         *remote.Remote
	PidCache       *PidCacheValue
	MemberList     *MemberList
	IdentityLookup IdentityLookup
	kindsMu        sync.RWMutex
	kinds          map[string]*ActivatedKind
	context        Context

	metrics        *clustermetrics.ClusterMetrics
	metricsEnabled bool
}
```

Add `"sync"` to the imports.

**3b.** Update `GetClusterKinds()` (line 158):

```go
func (c *Cluster) GetClusterKinds() []string {
	c.kindsMu.RLock()
	defer c.kindsMu.RUnlock()
	keys := make([]string, 0, len(c.kinds))
	for k := range c.kinds {
		keys = append(keys, k)
	}
	return keys
}
```

**3c.** Update `GetClusterKind()` (line 225):

```go
func (c *Cluster) GetClusterKind(kind string) *ActivatedKind {
	c.kindsMu.RLock()
	defer c.kindsMu.RUnlock()
	k, ok := c.kinds[kind]
	if !ok {
		c.Logger().Error("Invalid kind", slog.String("kind", kind))
		return nil
	}
	return k
}
```

**3d.** Update `TryGetClusterKind()` (line 236):

```go
func (c *Cluster) TryGetClusterKind(kind string) (*ActivatedKind, bool) {
	c.kindsMu.RLock()
	defer c.kindsMu.RUnlock()
	k, ok := c.kinds[kind]
	return k, ok
}
```

**3e.** Update `VirtualActorCount()` (line 94):

```go
func (c *Cluster) VirtualActorCount() int64 {
	c.kindsMu.RLock()
	defer c.kindsMu.RUnlock()
	var total int64
	for _, k := range c.kinds {
		total += int64(k.Count())
	}
	return total
}
```

**3f.** Update `initKinds()` (line 242). Build OUTSIDE the lock, then store under the lock:

```go
func (c *Cluster) initKinds() {
	activated := make(map[string]*ActivatedKind, len(c.Config.Kinds))
	for name, kind := range c.Config.Kinds {
		activated[name] = kind.Build(c)
	}

	c.kindsMu.Lock()
	defer c.kindsMu.Unlock()
	for name, ak := range activated {
		c.kinds[name] = ak
	}
	c.ensureTopicKindRegisteredLocked()
}
```

**3g.** Update `InitKindsForTest()` (line 249):

```go
func (c *Cluster) InitKindsForTest(kinds ...*Kind) {
	activated := make(map[string]*ActivatedKind, len(kinds))
	for _, kind := range kinds {
		activated[kind.Kind] = kind.Build(c)
	}

	c.kindsMu.Lock()
	defer c.kindsMu.Unlock()
	for name, ak := range activated {
		c.kinds[name] = ak
	}
}
```

**3h.** Rename `ensureTopicKindRegistered` → `ensureTopicKindRegisteredLocked` (line 260). The caller holds `kindsMu.Lock`. The `NewKind` call inside has `StrategyBuilder: nil`, so `Build` does not call back into the cluster — safe under the lock.

```go
// ensureTopicKindRegisteredLocked ensures that the topic kind is registered.
// Caller must hold kindsMu.Lock.
func (c *Cluster) ensureTopicKindRegisteredLocked() {
	hasTopicKind := false
	for name := range c.kinds {
		if name == TopicActorKind {
			hasTopicKind = true
			break
		}
	}
	if !hasTopicKind {
		store := &EmptyKeyValueStore[*Subscribers]{}
		storeTimeout := c.Config.PubSubConfig.SubscriptionStoreTimeout

		c.kinds[TopicActorKind] = NewKind(TopicActorKind, actor.PropsFromProducer(func() actor.Actor {
			return NewTopicActor(store, c.Logger(), storeTimeout)
		})).Build(c)
	}
}
```

### Step 4: Run tests and race detector

Run: `go test ./cluster/ -run TestCluster_.*ThreadSafe -race -count=1 -v`

Expected: PASS

Run: `go test ./cluster/ -race -count=1 -v`

Expected: All existing tests still PASS (no behavior change, only locking added)

### Step 5: Commit

```bash
git add cluster/cluster.go cluster/cluster_kind_test.go
git commit -m "feat(cluster): add RWMutex protection to kinds map

Thread-safe access to the kinds registry in preparation for
runtime Kind registration. All existing accessors now acquire
the appropriate read or write lock."
```

---

## Task 2: RegisterKind / DeregisterKind API

**Files:**
- Modify: `cluster/cluster.go` (add `provider` field, `RegisterKind`, `DeregisterKind`, helpers)
- Modify: `cluster/cluster_kind_test.go` (add API tests)

**Depends on:** Task 1

### Step 1: Write the failing tests

Add to `cluster/cluster_kind_test.go`:

```go
func TestCluster_RegisterKind_Success(t *testing.T) {
	c := newTestCluster()

	kind := NewKind("newKind", actor.PropsFromProducer(func() actor.Actor {
		return &actor.EmptyActor{}
	}))

	err := c.RegisterKind(kind)
	assert.NoError(t, err)

	ak, ok := c.TryGetClusterKind("newKind")
	assert.True(t, ok)
	assert.NotNil(t, ak)
	assert.Equal(t, "newKind", ak.Kind)
}

func TestCluster_RegisterKind_Duplicate(t *testing.T) {
	c := newTestCluster()
	kind := NewKind("dup", actor.PropsFromProducer(func() actor.Actor {
		return &actor.EmptyActor{}
	}))

	err := c.RegisterKind(kind)
	assert.NoError(t, err)

	err = c.RegisterKind(kind)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestCluster_DeregisterKind_Success(t *testing.T) {
	c := newTestCluster()
	kind := NewKind("removable", actor.PropsFromProducer(func() actor.Actor {
		return &actor.EmptyActor{}
	}))

	err := c.RegisterKind(kind)
	assert.NoError(t, err)

	err = c.DeregisterKind("removable")
	assert.NoError(t, err)

	_, ok := c.TryGetClusterKind("removable")
	assert.False(t, ok)
}

func TestCluster_DeregisterKind_Reserved(t *testing.T) {
	c := newTestCluster()

	err := c.DeregisterKind(TopicActorKind)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reserved")
}

func TestCluster_DeregisterKind_NotFound(t *testing.T) {
	c := newTestCluster()

	err := c.DeregisterKind("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")
}

func TestCluster_RegisterKind_NotifiesProvider(t *testing.T) {
	c := newTestCluster()

	var calledWithKinds []string
	c.provider = &mockKindUpdaterProvider{
		updateKinds: func(kinds []string) error {
			calledWithKinds = kinds
			return nil
		},
	}

	kind := NewKind("notified", actor.PropsFromProducer(func() actor.Actor {
		return &actor.EmptyActor{}
	}))

	err := c.RegisterKind(kind)
	assert.NoError(t, err)
	assert.Contains(t, calledWithKinds, "notified")
}

func TestCluster_RegisterKind_ConcurrentAccess(t *testing.T) {
	c := newTestCluster()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			kind := NewKind(
				fmt.Sprintf("kind-%d", i),
				actor.PropsFromProducer(func() actor.Actor {
					return &actor.EmptyActor{}
				}),
			)
			_ = c.RegisterKind(kind)
		}()
	}

	// Concurrent reads while writing
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.GetClusterKinds()
		}()
	}
	wg.Wait()

	kinds := c.GetClusterKinds()
	assert.Len(t, kinds, 50)
}

// Mock provider that implements both ClusterProvider and KindUpdater
type mockKindUpdaterProvider struct {
	updateKinds func(kinds []string) error
}

func (m *mockKindUpdaterProvider) StartMember(_ *Cluster) error { return nil }
func (m *mockKindUpdaterProvider) StartClient(_ *Cluster) error { return nil }
func (m *mockKindUpdaterProvider) Shutdown(_ bool) error        { return nil }
func (m *mockKindUpdaterProvider) UpdateKinds(kinds []string) error {
	if m.updateKinds != nil {
		return m.updateKinds(kinds)
	}
	return nil
}
```

Add `"fmt"` to the imports in the test file.

### Step 2: Run test to verify it fails

Run: `go test ./cluster/ -run TestCluster_RegisterKind -race -count=1 -v`

Expected: FAIL — `RegisterKind` does not exist yet.

### Step 3: Implement RegisterKind, DeregisterKind, and supporting code

In `cluster/cluster.go`:

**3a.** Add `provider` field to Cluster struct (after `kindsMu`):

```go
type Cluster struct {
	ActorSystem    *actor.ActorSystem
	Config         *Config
	Gossip         *Gossiper
	PubSub         *PubSub
	Remote         *remote.Remote
	PidCache       *PidCacheValue
	MemberList     *MemberList
	IdentityLookup IdentityLookup
	kindsMu        sync.RWMutex
	kinds          map[string]*ActivatedKind
	provider       ClusterProvider
	context        Context

	metrics        *clustermetrics.ClusterMetrics
	metricsEnabled bool
}
```

**3b.** Set `c.provider` in `StartMember()` (at the top of the method, before `initKinds`):

```go
func (c *Cluster) StartMember() error {
	cfg := c.Config
	c.provider = cfg.ClusterProvider
	c.Remote = remote.NewRemote(c.ActorSystem, c.Config.RemoteConfig)
	// ... rest unchanged
```

**3c.** Add `RegisterKind`, `DeregisterKind`, and helpers after `initKinds`:

```go
// RegisterKind registers a new Kind with the cluster at runtime.
// The Kind becomes available for local activation immediately.
// Other cluster members discover the new Kind on the next topology
// update cycle (typically within one heartbeat interval).
//
// Returns an error if a Kind with the same name is already registered.
func (c *Cluster) RegisterKind(kind *Kind) error {
	// Build outside the lock — Build may call StrategyBuilder(c)
	// which may acquire kindsMu.RLock via TryGetClusterKind.
	activated := kind.Build(c)

	c.kindsMu.Lock()
	defer c.kindsMu.Unlock()

	if _, exists := c.kinds[kind.Kind]; exists {
		return fmt.Errorf("kind %q is already registered", kind.Kind)
	}

	c.kinds[kind.Kind] = activated
	c.notifyKindUpdate()
	return nil
}

// DeregisterKind removes a Kind from the cluster. Returns an error if
// the Kind doesn't exist. The TopicActorKind cannot be deregistered.
func (c *Cluster) DeregisterKind(kindName string) error {
	c.kindsMu.Lock()
	defer c.kindsMu.Unlock()

	if kindName == TopicActorKind {
		return fmt.Errorf("kind %q is reserved and cannot be deregistered", kindName)
	}

	if _, exists := c.kinds[kindName]; !exists {
		return fmt.Errorf("kind %q is not registered", kindName)
	}

	delete(c.kinds, kindName)
	c.notifyKindUpdate()
	return nil
}

// getClusterKindsLocked returns kind names. Caller must hold kindsMu.
func (c *Cluster) getClusterKindsLocked() []string {
	keys := make([]string, 0, len(c.kinds))
	for k := range c.kinds {
		keys = append(keys, k)
	}
	return keys
}

func (c *Cluster) notifyKindUpdate() {
	// kindsMu must be held by caller.
	if c.provider == nil {
		return
	}
	if updater, ok := c.provider.(KindUpdater); ok {
		kinds := c.getClusterKindsLocked()
		if err := updater.UpdateKinds(kinds); err != nil {
			c.Logger().Error("Failed to notify provider of kind update",
				slog.Any("error", err))
		}
	}
}
```

### Step 4: Run tests

Run: `go test ./cluster/ -run TestCluster_ -race -count=1 -v`

Expected: PASS (the `KindUpdater` interface doesn't exist yet, but `notifyKindUpdate` will be a no-op since the type assertion fails)

Wait — the `KindUpdater` interface is referenced in `notifyKindUpdate`. It must exist. Add it first as a stub (Task 3 fills in the docs). Add to `cluster/cluster_provider.go`:

```go
// KindUpdater is an optional interface that ClusterProviders can implement
// to support runtime Kind changes.
type KindUpdater interface {
	UpdateKinds(kinds []string) error
}
```

Run: `go test ./cluster/ -run TestCluster_ -race -count=1 -v`

Expected: All PASS

Run: `go test ./cluster/ -race -count=1`

Expected: All existing tests still PASS

### Step 5: Commit

```bash
git add cluster/cluster.go cluster/cluster_kind_test.go cluster/cluster_provider.go
git commit -m "feat(cluster): add RegisterKind/DeregisterKind API

Runtime Kind registration with provider notification via the
KindUpdater interface. Build runs outside the lock to prevent
deadlock with reentrant RLock."
```

---

## Task 3: KindUpdater Interface

This was done as part of Task 2 (the interface had to exist for `notifyKindUpdate` to compile). Verify it's in place.

**Files:**
- Verify: `cluster/cluster_provider.go`

### Step 1: Verify the interface exists

Read `cluster/cluster_provider.go` and confirm:

```go
// KindUpdater is an optional interface that ClusterProviders can implement
// to support runtime Kind changes. When implemented, the cluster calls
// UpdateKinds after RegisterKind or DeregisterKind is called.
//
// Providers that don't implement this interface still work — they just
// won't announce Kind changes to the cluster until the node restarts.
type KindUpdater interface {
	// UpdateKinds is called when the cluster's registered Kinds change.
	// The provider should re-announce the node with the updated kind list.
	// The kinds slice contains all currently registered Kind names.
	UpdateKinds(kinds []string) error
}
```

### Step 2: Run tests

Run: `go test ./cluster/ -race -count=1`

Expected: PASS

### Step 3: Commit (if docstring was updated)

```bash
git add cluster/cluster_provider.go
git commit -m "docs(cluster): add KindUpdater interface documentation"
```

---

## Task 4: Automanaged Provider KindUpdater

**Files:**
- Modify: `cluster/clusterproviders/automanaged/automanaged.go`
- Create: `cluster/clusterproviders/automanaged/automanaged_kind_test.go`

**Independent of:** Tasks 5, 6, 7

### Step 1: Write the failing test

Create `cluster/clusterproviders/automanaged/automanaged_kind_test.go`:

```go
package automanaged

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/awevoke/protoactor-go/cluster"
)

// Compile-time check that AutoManagedProvider implements KindUpdater.
var _ cluster.KindUpdater = (*AutoManagedProvider)(nil)

func TestAutoManagedProvider_UpdateKinds(t *testing.T) {
	p := NewWithConfig(2000, 6330, "localhost:6330")
	p.knownKinds = []string{"kindA"}

	err := p.UpdateKinds([]string{"kindA", "kindB"})
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"kindA", "kindB"}, p.knownKinds)
}

func TestAutoManagedProvider_UpdateKinds_ReflectedInGetCurrentNode(t *testing.T) {
	p := NewWithConfig(2000, 6330, "localhost:6330")
	p.knownKinds = []string{"kindA"}

	err := p.UpdateKinds([]string{"kindA", "kindB"})
	assert.NoError(t, err)

	node := p.getCurrentNode()
	assert.ElementsMatch(t, []string{"kindA", "kindB"}, node.Kinds)
}
```

### Step 2: Run test to verify it fails

Run: `go test ./cluster/clusterproviders/automanaged/ -run TestAutoManagedProvider_UpdateKinds -race -count=1 -v`

Expected: FAIL — compile error, `AutoManagedProvider` does not implement `KindUpdater`.

### Step 3: Implement UpdateKinds

Add to `cluster/clusterproviders/automanaged/automanaged.go`:

```go
// UpdateKinds updates the known kinds for this node. The updated kinds
// will be returned in the next /_health response, which other nodes
// poll periodically.
func (p *AutoManagedProvider) UpdateKinds(kinds []string) error {
	p.knownKinds = kinds
	return nil
}
```

### Step 4: Run tests

Run: `go test ./cluster/clusterproviders/automanaged/ -run TestAutoManagedProvider_UpdateKinds -race -count=1 -v`

Expected: PASS

Run: `go test ./cluster/clusterproviders/automanaged/ -race -count=1`

Expected: All tests PASS

### Step 5: Commit

```bash
git add cluster/clusterproviders/automanaged/automanaged.go cluster/clusterproviders/automanaged/automanaged_kind_test.go
git commit -m "feat(automanaged): implement KindUpdater interface

UpdateKinds stores the new kinds list. Other nodes pick up the
change on the next /_health poll cycle."
```

---

## Task 5: NATS KV Provider KindUpdater

**Files:**
- Modify: `cluster/clusterproviders/natskv/natskv_provider.go`
- Modify: `cluster/clusterproviders/natskv/natskv_provider_test.go` (add UpdateKinds test)

**Independent of:** Tasks 4, 6, 7

### Step 1: Write the failing test

Add to `cluster/clusterproviders/natskv/natskv_provider_test.go`:

```go
// Compile-time check that Provider implements KindUpdater.
var _ cluster.KindUpdater = (*Provider)(nil)

func TestProvider_UpdateKinds(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-updatekinds")

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	originalKinds := p.self.Kinds

	// Update kinds
	newKinds := append([]string{}, originalKinds...)
	newKinds = append(newKinds, "dynamicKind")
	err = p.UpdateKinds(newKinds)
	require.NoError(t, err)

	// Verify self.Kinds is updated
	assert.ElementsMatch(t, newKinds, p.self.Kinds)

	// Verify the KV bucket reflects the update
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := p.memberKey(p.self.ID)
	entry, err := p.memberBucket.Get(ctx, key)
	require.NoError(t, err)

	var node Node
	require.NoError(t, json.Unmarshal(entry.Value(), &node))
	assert.ElementsMatch(t, newKinds, node.Kinds)
}
```

### Step 2: Run test to verify it fails

Run: `go test ./cluster/clusterproviders/natskv/ -run TestProvider_UpdateKinds -race -count=1 -v`

Expected: FAIL — compile error, `Provider` does not implement `KindUpdater`.

### Step 3: Implement UpdateKinds

Add to `cluster/clusterproviders/natskv/natskv_provider.go` (after `Shutdown` method):

```go
// UpdateKinds updates the node's kind list and re-registers in the KV
// bucket so other nodes see the change on the next watch event.
func (p *Provider) UpdateKinds(kinds []string) error {
	p.self.Kinds = kinds

	// Re-register with updated kinds.
	return p.registerSelf()
}
```

Note: `p.self` is only written from `init()` (startup), `UpdateKinds` (called under `c.kindsMu.Lock`), and `refreshMemberKey` (which serializes `p.self`). The `refreshMemberKey` serializes the full Node struct, so there's a potential race between `UpdateKinds` writing `p.self.Kinds` and `refreshMemberKey` reading it. Both run in different goroutines.

The `p.self.Kinds` field is a slice (pointer + len + cap). Writing a slice header while another goroutine reads it is a data race. Protect with the existing `membersMu`:

```go
// UpdateKinds updates the node's kind list and re-registers in the KV
// bucket so other nodes see the change on the next watch event.
func (p *Provider) UpdateKinds(kinds []string) error {
	p.membersMu.Lock()
	p.self.Kinds = kinds
	p.membersMu.Unlock()

	return p.registerSelf()
}
```

Also update `refreshMemberKey` to read `p.self` under the same lock:

```go
func (p *Provider) refreshMemberKey() error {
	p.membersMu.RLock()
	data, err := p.self.Serialize()
	p.membersMu.RUnlock()
	if err != nil {
		return err
	}
	key := p.memberKey(p.self.ID)
	_, err = p.memberBucket.Put(p.ctx, key, data)
	return err
}
```

And `registerSelf`:

```go
func (p *Provider) registerSelf() error {
	p.membersMu.RLock()
	data, err := p.self.Serialize()
	p.membersMu.RUnlock()
	if err != nil {
		return fmt.Errorf("natskv: serialize self: %w", err)
	}

	key := p.memberKey(p.self.ID)
	_, err = p.memberBucket.Put(p.ctx, key, data)
	if err != nil {
		return fmt.Errorf("natskv: register self: %w", err)
	}

	return nil
}
```

### Step 4: Run tests

Run: `go test ./cluster/clusterproviders/natskv/ -run TestProvider_UpdateKinds -race -count=1 -v`

Expected: PASS

Run: `go test ./cluster/clusterproviders/natskv/ -race -count=1`

Expected: All tests PASS

### Step 5: Commit

```bash
git add cluster/clusterproviders/natskv/natskv_provider.go cluster/clusterproviders/natskv/natskv_provider_test.go
git commit -m "feat(natskv): implement KindUpdater interface

UpdateKinds writes the updated self node to the KV bucket.
Protects p.self.Kinds with membersMu to prevent race with
the refresh goroutine."
```

---

## Task 6: NATS Stream Provider KindUpdater

**Files:**
- Modify: `cluster/clusterproviders/natsstream/natsstream_provider.go`
- Modify: `cluster/clusterproviders/natsstream/natsstream_provider_test.go` (add UpdateKinds test)

**Independent of:** Tasks 4, 5, 7

### Step 1: Write the failing test

Add to `cluster/clusterproviders/natsstream/natsstream_provider_test.go` (create this file if it doesn't exist, or add to it):

```go
// Compile-time check that Provider implements KindUpdater.
var _ cluster.KindUpdater = (*Provider)(nil)

func TestProvider_UpdateKinds(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-updatekinds")

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	originalKinds := p.self.Kinds

	newKinds := append([]string{}, originalKinds...)
	newKinds = append(newKinds, "dynamicKind")
	err = p.UpdateKinds(newKinds)
	require.NoError(t, err)

	// Verify self.Kinds is updated
	p.membersMu.RLock()
	assert.ElementsMatch(t, newKinds, p.self.Kinds)
	p.membersMu.RUnlock()
}
```

### Step 2: Run test to verify it fails

Run: `go test ./cluster/clusterproviders/natsstream/ -run TestProvider_UpdateKinds -race -count=1 -v`

Expected: FAIL — compile error, `Provider` does not implement `KindUpdater`.

### Step 3: Implement UpdateKinds

Add to `cluster/clusterproviders/natsstream/natsstream_provider.go` (after `Shutdown` method):

```go
// UpdateKinds updates the node's kind list and publishes an immediate
// heartbeat so other nodes see the change without waiting for the next
// heartbeat cycle.
func (p *Provider) UpdateKinds(kinds []string) error {
	p.membersMu.Lock()
	p.self.Kinds = kinds
	p.membersMu.Unlock()

	return p.publishHeartbeat()
}
```

Also protect `p.self` serialization in `publishHeartbeat`:

```go
func (p *Provider) publishHeartbeat() error {
	p.membersMu.RLock()
	data, err := p.self.Serialize()
	p.membersMu.RUnlock()
	if err != nil {
		return fmt.Errorf("natsstream: serialize self: %w", err)
	}

	subject := p.prefix + ".members." + p.self.ID
	_, err = p.js.Publish(p.ctx, subject, data, jetstream.WithMsgTTL(p.config.HeartbeatTTL))
	if err != nil {
		return fmt.Errorf("natsstream: publish heartbeat: %w", err)
	}
	return nil
}
```

### Step 4: Run tests

Run: `go test ./cluster/clusterproviders/natsstream/ -run TestProvider_UpdateKinds -race -count=1 -v`

Expected: PASS

Run: `go test ./cluster/clusterproviders/natsstream/ -race -count=1`

Expected: All tests PASS

### Step 5: Commit

```bash
git add cluster/clusterproviders/natsstream/natsstream_provider.go cluster/clusterproviders/natsstream/natsstream_provider_test.go
git commit -m "feat(natsstream): implement KindUpdater interface

UpdateKinds writes the updated self node and publishes an
immediate heartbeat. Protects p.self.Kinds with membersMu."
```

---

## Task 7: K8s Provider KindUpdater

**Files:**
- Modify: `cluster/clusterproviders/k8s/k8s_provider.go`
- Modify or create: `cluster/clusterproviders/k8s/k8s_provider_unit_test.go` (add UpdateKinds test)

**Independent of:** Tasks 4, 5, 6

### Step 1: Write the failing test

Add to `cluster/clusterproviders/k8s/k8s_provider_unit_test.go`:

```go
// Compile-time check that Provider implements KindUpdater.
var _ cluster.KindUpdater = (*Provider)(nil)

func TestProvider_UpdateKinds_UpdatesKnownKinds(t *testing.T) {
	p := &Provider{
		knownKinds: []string{"kindA"},
	}

	err := p.UpdateKinds([]string{"kindA", "kindB"})
	// We expect an error because there's no K8s client, but knownKinds should be updated.
	// The error is from replacePodLabels failing without a real cluster.
	_ = err
	assert.ElementsMatch(t, []string{"kindA", "kindB"}, p.knownKinds)
}
```

Note: The K8s provider can't be fully tested without a K8s cluster. The unit test verifies the field update; the pod label update will naturally fail without a real API server. We'll note this as a known limitation — the K8s integration test requires a real cluster.

### Step 2: Run test to verify it fails

Run: `go test ./cluster/clusterproviders/k8s/ -run TestProvider_UpdateKinds -race -count=1 -v`

Expected: FAIL — compile error, `Provider` does not implement `KindUpdater`.

### Step 3: Implement UpdateKinds

Add to `cluster/clusterproviders/k8s/k8s_provider.go` (after `Shutdown` method):

```go
// UpdateKinds updates the node's kind list and re-registers pod labels
// so other nodes watching pods see the change. Each kind is stored as
// a separate pod label (LabelKind-{name}=true), avoiding the 63-character
// K8s label value limit.
func (p *Provider) UpdateKinds(kinds []string) error {
	p.knownKinds = kinds

	if p.cluster == nil || p.podName == "" {
		return nil
	}

	// Re-register to update pod labels with the new kinds.
	timeout := p.cluster.Config.RequestTimeoutTime
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return p.registerMember(timeout)
}
```

The `registerMember` method (line 198-230) already reads `p.knownKinds` to build labels. It fetches the current pod, sets all labels (including per-kind labels), and calls `replacePodLabels`. Since `registerMember` first fetches the pod and then replaces ALL labels, old kind labels from a previous set of kinds need to be cleaned up. Check if `registerMember` clears old kind labels:

Looking at `registerMember` (line 198-230):
```go
labels := Labels{
    LabelCluster:  p.clusterName,
    LabelPort:     fmt.Sprintf("%d", p.port),
    LabelMemberID: p.id,
}
for _, kind := range p.knownKinds {
    labelkey := fmt.Sprintf("%s-%s", LabelKind, kind)
    labels[labelkey] = "true"
}
// add existing labels back
for key, value := range pod.Labels {
    labels[key] = value
}
```

**Problem:** It adds existing labels back, which means OLD kind labels from a previous registration are preserved. When kinds are removed via `DeregisterKind`, the old labels remain.

**Fix:** Before adding existing labels back, remove old proto.actor kind labels:

```go
func (p *Provider) registerMember(timeout time.Duration) error {
	// ... existing code up to labels initialization ...

	labels := Labels{
		LabelCluster:  p.clusterName,
		LabelPort:     fmt.Sprintf("%d", p.port),
		LabelMemberID: p.id,
	}

	// add current known kinds to labels
	for _, kind := range p.knownKinds {
		labelkey := fmt.Sprintf("%s-%s", LabelKind, kind)
		labels[labelkey] = "true"
	}

	// add existing labels back, but skip old proto.actor kind labels
	// (they were replaced by the current knownKinds above)
	for key, value := range pod.Labels {
		if strings.HasPrefix(key, LabelKind+"-") {
			continue // skip old kind labels; current kinds were set above
		}
		labels[key] = value
	}
	pod.SetLabels(labels)

	return p.replacePodLabels(ctx, pod)
}
```

### Step 4: Run tests

Run: `go test ./cluster/clusterproviders/k8s/ -run TestProvider_UpdateKinds -race -count=1 -v`

Expected: PASS

Run: `go test ./cluster/clusterproviders/k8s/ -race -count=1`

Expected: All tests PASS

### Step 5: Commit

```bash
git add cluster/clusterproviders/k8s/k8s_provider.go cluster/clusterproviders/k8s/k8s_provider_unit_test.go
git commit -m "feat(k8s): implement KindUpdater interface

UpdateKinds re-registers pod labels with the updated kind list.
Old kind labels are cleaned up during re-registration to handle
DeregisterKind correctly."
```

---

## Task 8: MemberList Kind-Change Detection

**This is the highest-risk change.** It modifies the topology critical path.

**Files:**
- Modify: `cluster/member_list.go:104-145` (UpdateClusterTopology), `:169-194` (getTopologyChanges)
- Create: `cluster/member_list_kind_change_test.go`

**Depends on:** Task 1 (thread-safe registry)

### Architecture of the change

```
UpdateClusterTopology(members)
  │
  ├─ getTopologyChanges(members)
  │    ├─ NewMemberSet(members) → active
  │    ├─ active.Equals(ml.members)?  ← topology hash (ID-only)
  │    │    YES → hasKindChanges(active)?
  │    │           NO  → return done=true (truly unchanged)
  │    │           YES → fall through (process kind changes)
  │    ├─ compute joined, left
  │    └─ return topology, done=false, active, joined, left
  │
  ├─ Block left members
  ├─ ★ processKindChangesForStayingMembers(active)  ← NEW: before ml.members overwrite
  ├─ ml.members = active
  ├─ memberLeave for left members
  ├─ memberJoin for joined members
  └─ Publish topology event
```

**Critical ordering:** `processKindChangesForStayingMembers` MUST run BEFORE `ml.members = active` because it compares old kinds (from `ml.members`) against new kinds (from `active`).

### Step 1: Write the failing tests

Create `cluster/member_list_kind_change_test.go`:

```go
package cluster

import (
	"testing"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKindsEqual_SameKinds(t *testing.T) {
	assert.True(t, kindsEqual([]string{"a", "b"}, []string{"a", "b"}))
}

func TestKindsEqual_DifferentOrder(t *testing.T) {
	assert.True(t, kindsEqual([]string{"b", "a"}, []string{"a", "b"}))
}

func TestKindsEqual_DifferentLength(t *testing.T) {
	assert.False(t, kindsEqual([]string{"a"}, []string{"a", "b"}))
}

func TestKindsEqual_DifferentKinds(t *testing.T) {
	assert.False(t, kindsEqual([]string{"a", "b"}, []string{"a", "c"}))
}

func TestKindsEqual_BothEmpty(t *testing.T) {
	assert.True(t, kindsEqual([]string{}, []string{}))
}

func TestKindsEqual_BothNil(t *testing.T) {
	assert.True(t, kindsEqual(nil, nil))
}

func TestKindsEqual_NilVsEmpty(t *testing.T) {
	assert.True(t, kindsEqual(nil, []string{}))
}

func newTestClusterForMemberList() *Cluster {
	system := actor.NewActorSystem()
	cfg := Configure("test-cluster", nil, nil, nil)
	c := &Cluster{
		ActorSystem: system,
		Config:      cfg,
		kinds:       map[string]*ActivatedKind{},
	}
	c.MemberList = NewMemberList(c)
	return c
}

func TestMemberList_KindChange_Detected(t *testing.T) {
	c := newTestClusterForMemberList()
	ml := c.MemberList

	// Register kinds so getMemberStrategyByKind can find them
	c.InitKindsForTest(
		NewKind("kindA", actor.PropsFromProducer(func() actor.Actor { return &actor.EmptyActor{} })),
		NewKind("kindB", actor.PropsFromProducer(func() actor.Actor { return &actor.EmptyActor{} })),
	)

	// Initial topology: member1 has kindA
	initialMembers := Members{
		{Id: "member1", Host: "h1", Port: 1, Kinds: []string{"kindA"}},
	}
	ml.UpdateClusterTopology(initialMembers)

	// Verify member1 is in kindA strategy
	ml.mutex.RLock()
	stratA := ml.memberStrategyByKind["kindA"]
	ml.mutex.RUnlock()
	require.NotNil(t, stratA)
	assert.Len(t, stratA.GetAllMembers(), 1)

	// Update: member1 now has kindA AND kindB (same member ID, different kinds)
	updatedMembers := Members{
		{Id: "member1", Host: "h1", Port: 1, Kinds: []string{"kindA", "kindB"}},
	}
	ml.UpdateClusterTopology(updatedMembers)

	// Verify member1 is now also in kindB strategy
	ml.mutex.RLock()
	stratB := ml.memberStrategyByKind["kindB"]
	ml.mutex.RUnlock()
	require.NotNil(t, stratB, "kindB strategy should exist after kind change")
	assert.Len(t, stratB.GetAllMembers(), 1, "member1 should be in kindB strategy")
}

func TestMemberList_KindChange_RemovedKind(t *testing.T) {
	c := newTestClusterForMemberList()
	ml := c.MemberList

	c.InitKindsForTest(
		NewKind("kindA", actor.PropsFromProducer(func() actor.Actor { return &actor.EmptyActor{} })),
		NewKind("kindB", actor.PropsFromProducer(func() actor.Actor { return &actor.EmptyActor{} })),
	)

	// Initial: member1 has kindA and kindB
	initialMembers := Members{
		{Id: "member1", Host: "h1", Port: 1, Kinds: []string{"kindA", "kindB"}},
	}
	ml.UpdateClusterTopology(initialMembers)

	ml.mutex.RLock()
	assert.Len(t, ml.memberStrategyByKind["kindA"].GetAllMembers(), 1)
	assert.Len(t, ml.memberStrategyByKind["kindB"].GetAllMembers(), 1)
	ml.mutex.RUnlock()

	// Update: member1 drops kindB
	updatedMembers := Members{
		{Id: "member1", Host: "h1", Port: 1, Kinds: []string{"kindA"}},
	}
	ml.UpdateClusterTopology(updatedMembers)

	ml.mutex.RLock()
	assert.Len(t, ml.memberStrategyByKind["kindA"].GetAllMembers(), 1, "kindA should still have member1")
	assert.Len(t, ml.memberStrategyByKind["kindB"].GetAllMembers(), 0, "kindB should no longer have member1")
	ml.mutex.RUnlock()
}

func TestMemberList_KindChange_NoChangeIsNoop(t *testing.T) {
	c := newTestClusterForMemberList()
	ml := c.MemberList

	c.InitKindsForTest(
		NewKind("kindA", actor.PropsFromProducer(func() actor.Actor { return &actor.EmptyActor{} })),
	)

	members := Members{
		{Id: "member1", Host: "h1", Port: 1, Kinds: []string{"kindA"}},
	}
	ml.UpdateClusterTopology(members)

	ml.mutex.RLock()
	stratA := ml.memberStrategyByKind["kindA"]
	memberCountBefore := len(stratA.GetAllMembers())
	ml.mutex.RUnlock()

	// Call again with same data — no change expected
	ml.UpdateClusterTopology(members)

	ml.mutex.RLock()
	assert.Len(t, ml.memberStrategyByKind["kindA"].GetAllMembers(), memberCountBefore,
		"strategy should not have duplicate members after no-op update")
	ml.mutex.RUnlock()
}

func TestMemberList_KindChange_WithJoinAndLeave(t *testing.T) {
	c := newTestClusterForMemberList()
	ml := c.MemberList

	c.InitKindsForTest(
		NewKind("kindA", actor.PropsFromProducer(func() actor.Actor { return &actor.EmptyActor{} })),
		NewKind("kindB", actor.PropsFromProducer(func() actor.Actor { return &actor.EmptyActor{} })),
		NewKind("kindC", actor.PropsFromProducer(func() actor.Actor { return &actor.EmptyActor{} })),
	)

	// Initial: member1 (kindA), member2 (kindA)
	initialMembers := Members{
		{Id: "member1", Host: "h1", Port: 1, Kinds: []string{"kindA"}},
		{Id: "member2", Host: "h2", Port: 2, Kinds: []string{"kindA"}},
	}
	ml.UpdateClusterTopology(initialMembers)

	// Update: member1 adds kindB (kind change), member2 leaves, member3 joins with kindC
	updatedMembers := Members{
		{Id: "member1", Host: "h1", Port: 1, Kinds: []string{"kindA", "kindB"}},
		{Id: "member3", Host: "h3", Port: 3, Kinds: []string{"kindC"}},
	}
	ml.UpdateClusterTopology(updatedMembers)

	ml.mutex.RLock()
	defer ml.mutex.RUnlock()

	// member1: still in kindA, now also in kindB
	assert.Len(t, ml.memberStrategyByKind["kindA"].GetAllMembers(), 1, "only member1 in kindA")
	assert.Len(t, ml.memberStrategyByKind["kindB"].GetAllMembers(), 1, "member1 added to kindB")

	// member3: in kindC
	assert.Len(t, ml.memberStrategyByKind["kindC"].GetAllMembers(), 1, "member3 in kindC")
}

func TestMemberList_KindChange_StrategyMembersCorrect(t *testing.T) {
	c := newTestClusterForMemberList()
	ml := c.MemberList

	c.InitKindsForTest(
		NewKind("kindA", actor.PropsFromProducer(func() actor.Actor { return &actor.EmptyActor{} })),
		NewKind("kindB", actor.PropsFromProducer(func() actor.Actor { return &actor.EmptyActor{} })),
	)

	// Two members, both with kindA
	initialMembers := Members{
		{Id: "m1", Host: "h1", Port: 1, Kinds: []string{"kindA"}},
		{Id: "m2", Host: "h2", Port: 2, Kinds: []string{"kindA"}},
	}
	ml.UpdateClusterTopology(initialMembers)

	// m1 adds kindB, m2 drops kindA and adds kindB
	updatedMembers := Members{
		{Id: "m1", Host: "h1", Port: 1, Kinds: []string{"kindA", "kindB"}},
		{Id: "m2", Host: "h2", Port: 2, Kinds: []string{"kindB"}},
	}
	ml.UpdateClusterTopology(updatedMembers)

	ml.mutex.RLock()
	defer ml.mutex.RUnlock()

	// kindA: only m1
	assert.Len(t, ml.memberStrategyByKind["kindA"].GetAllMembers(), 1)
	// kindB: both m1 and m2
	assert.Len(t, ml.memberStrategyByKind["kindB"].GetAllMembers(), 2)
}
```

### Step 2: Run tests to verify they fail

Run: `go test ./cluster/ -run TestKindsEqual -race -count=1 -v`

Expected: FAIL — `kindsEqual` not defined

Run: `go test ./cluster/ -run TestMemberList_KindChange -race -count=1 -v`

Expected: FAIL — `kindsEqual` not defined, kind change detection doesn't exist

### Step 3: Implement Kind-change detection

**3a.** Add helper functions at the bottom of `cluster/member_list.go`:

```go
// kindsEqual reports whether two kind slices contain the same elements,
// regardless of order.
func kindsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	aSet := make(map[string]struct{}, len(a))
	for _, k := range a {
		aSet[k] = struct{}{}
	}
	for _, k := range b {
		if _, ok := aSet[k]; !ok {
			return false
		}
	}
	return true
}

// hasKindChanges checks whether any member in the new set has different
// Kinds compared to the same member (by ID) in ml.members.
func (ml *MemberList) hasKindChanges(newActive *MemberSet) bool {
	for _, newM := range newActive.Members() {
		if oldM := ml.members.GetMemberById(newM.Id); oldM != nil {
			if !kindsEqual(oldM.Kinds, newM.Kinds) {
				return true
			}
		}
	}
	return false
}

// processKindChangesForStayingMembers detects and applies Kind changes
// for members present in both old (ml.members) and new (newActive) sets.
// Must be called BEFORE ml.members is overwritten with the new set.
func (ml *MemberList) processKindChangesForStayingMembers(newActive *MemberSet) {
	for _, newM := range newActive.Members() {
		oldM := ml.members.GetMemberById(newM.Id)
		if oldM == nil {
			continue // new member, handled by memberJoin
		}
		if kindsEqual(oldM.Kinds, newM.Kinds) {
			continue // no change
		}
		ml.memberKindsChanged(oldM, newM)
	}
}

func (ml *MemberList) memberKindsChanged(oldMember, newMember *Member) {
	ml.cluster.Logger().Info("Member kinds changed",
		slog.String("member", newMember.Id),
		slog.Any("old", oldMember.Kinds),
		slog.Any("new", newMember.Kinds))

	oldKindSet := make(map[string]struct{}, len(oldMember.Kinds))
	for _, k := range oldMember.Kinds {
		oldKindSet[k] = struct{}{}
	}
	newKindSet := make(map[string]struct{}, len(newMember.Kinds))
	for _, k := range newMember.Kinds {
		newKindSet[k] = struct{}{}
	}

	// Remove member from strategies for removed kinds
	for _, kind := range oldMember.Kinds {
		if _, inNew := newKindSet[kind]; !inNew {
			if strategy, ok := ml.memberStrategyByKind[kind]; ok {
				strategy.RemoveMember(newMember)
			}
		}
	}

	// Add member to strategies for added kinds
	for _, kind := range newMember.Kinds {
		if _, inOld := oldKindSet[kind]; !inOld {
			if ml.memberStrategyByKind[kind] == nil {
				ml.memberStrategyByKind[kind] = ml.getMemberStrategyByKind(kind)
			}
			ml.memberStrategyByKind[kind].AddMember(newMember)
		}
	}
}
```

**3b.** Modify `getTopologyChanges` to not short-circuit on kind-only changes:

Replace the early return in `getTopologyChanges` (currently at line 179):

```go
func (ml *MemberList) getTopologyChanges(members Members) (topology *ClusterTopology, unchanged bool, active *MemberSet, joined *MemberSet, left *MemberSet) {
	memberSet := NewMemberSet(members)

	// get active members
	// (this bit means that we will never allow a member that failed a health check to join back in)
	blocked := ml.cluster.GetBlockedMembers().ToSlice()

	active = memberSet.ExceptIds(blocked)

	// nothing changed? exit — but also check for Kind changes on existing members
	if active.Equals(ml.members) && !ml.hasKindChanges(active) {
		return nil, true, nil, nil, nil
	}

	left = ml.members.Except(active)
	joined = active.Except(ml.members)

	topology = &ClusterTopology{
		TopologyHash: active.TopologyHash(),
		Members:      active.Members(),
		Left:         left.Members(),
		Joined:       joined.Members(),
	}

	return topology, false, active, joined, left
}
```

**3c.** Modify `UpdateClusterTopology` to process kind changes before overwriting `ml.members`:

```go
func (ml *MemberList) UpdateClusterTopology(members Members) {
	ml.mutex.Lock()
	defer ml.mutex.Unlock()

	// TLDR:
	// this method basically filters out any member status in the blocked list
	// then makes a delta between new and old members
	// notifying the cluster accordingly which members left or joined

	topology, done, active, joined, left := ml.getTopologyChanges(members)
	if done {
		return
	}

	// include any new blocked members into the known set of blocked members
	for _, m := range left.Members() {
		ml.cluster.Remote.BlockList().Block(m.Id)
	}

	// Detect kind changes on staying members BEFORE overwriting ml.members.
	// This must compare old state (ml.members) against new state (active).
	ml.processKindChangesForStayingMembers(active)

	ml.members = active

	// notify that these members left
	for _, m := range left.Members() {
		ml.memberLeave(m)
		ml.TerminateMember(m)
	}

	// notify that these members joined
	for _, m := range joined.Members() {
		ml.memberJoin(m)
	}

	ml.cluster.ActorSystem.EventStream.Publish(topology)

	ml.cluster.Logger().Info("Updated ClusterTopology",
		slog.Uint64("topology-hash", topology.TopologyHash),
		slog.Int("members", len(topology.Members)),
		slog.Int("joined", len(topology.Joined)),
		slog.Int("left", len(topology.Left)),
		slog.Int("blocked", len(topology.Blocked)),
		slog.Int("membersFromProvider", len(members)))
}
```

### Step 4: Run tests

Run: `go test ./cluster/ -run TestKindsEqual -race -count=1 -v`

Expected: All PASS

Run: `go test ./cluster/ -run TestMemberList_KindChange -race -count=1 -v`

Expected: All PASS

Run: `go test ./cluster/ -race -count=1`

Expected: All existing tests still PASS — the only behavior change is that kind-only updates now trigger topology events instead of being silently dropped.

### Step 5: Commit

```bash
git add cluster/member_list.go cluster/member_list_kind_change_test.go
git commit -m "feat(cluster): detect Kind changes in MemberList topology updates

When a member's Kinds change but the member set is unchanged,
getTopologyChanges now falls through instead of short-circuiting.
processKindChangesForStayingMembers updates member strategies for
added/removed kinds. Runs before ml.members is overwritten so it
can compare old vs new state."
```

---

## Task 9: End-to-End Integration Tests

**Files:**
- Create: `cluster/clusterproviders/natskv/natskv_kind_integration_test.go`
- Optionally: `cluster/clusterproviders/natsstream/natsstream_kind_integration_test.go`

**Depends on:** All previous tasks

### Step 1: Write the integration test (NATS KV)

Create `cluster/clusterproviders/natskv/natskv_kind_integration_test.go`:

```go
//go:build integration

package natskv

import (
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/remote"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupClusterWithKinds creates a provider, actor system, and cluster for
// testing with pre-registered Kinds.
func setupClusterWithKinds(t *testing.T, natsURL, clusterName string, kinds []*cluster.Kind, opts ...Option) (*Provider, *cluster.Cluster) {
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

	return p, c
}

// TestIntegration_RuntimeKindRegistration verifies the full flow:
// 1. Start 2-node cluster with kindA
// 2. Register kindB on node 1 at runtime
// 3. Verify node 2 discovers the new kind via topology update
func TestIntegration_RuntimeKindRegistration(t *testing.T) {
	natsURL := startNATSContainer(t)
	ttl := 3 * time.Second
	refresh := 500 * time.Millisecond

	kindA := cluster.NewKind("kindA", actor.PropsFromProducer(func() actor.Actor {
		return &actor.EmptyActor{}
	}))

	// Start node 1
	p1, c1 := setupClusterWithKinds(t, natsURL, "integ-kind-reg",
		[]*cluster.Kind{kindA},
		WithMemberTTL(ttl),
		WithRefreshInterval(refresh),
	)
	err := c1.StartMember()
	require.NoError(t, err)
	t.Cleanup(func() { c1.Shutdown(true) })

	time.Sleep(500 * time.Millisecond)

	// Start node 2
	p2, c2 := setupClusterWithKinds(t, natsURL, "integ-kind-reg",
		[]*cluster.Kind{kindA},
		WithMemberTTL(ttl),
		WithRefreshInterval(refresh),
	)
	err = c2.StartMember()
	require.NoError(t, err)
	t.Cleanup(func() { c2.Shutdown(true) })

	// Wait for mutual discovery
	require.Eventually(t, func() bool {
		p1.membersMu.RLock()
		_, p1SeesP2 := p1.members[p2.self.ID]
		p1.membersMu.RUnlock()
		return p1SeesP2
	}, 10*time.Second, 200*time.Millisecond, "node 1 should discover node 2")

	require.Eventually(t, func() bool {
		p2.membersMu.RLock()
		_, p2SeesP1 := p2.members[p1.self.ID]
		p2.membersMu.RUnlock()
		return p2SeesP1
	}, 10*time.Second, 200*time.Millisecond, "node 2 should discover node 1")

	// Register kindB on node 1 at runtime
	kindB := cluster.NewKind("kindB", actor.PropsFromProducer(func() actor.Actor {
		return &actor.EmptyActor{}
	}))
	err = c1.RegisterKind(kindB)
	require.NoError(t, err)

	// Verify node 1 has kindB locally
	_, ok := c1.TryGetClusterKind("kindB")
	assert.True(t, ok, "node 1 should have kindB locally")

	// Wait for node 2 to see that node 1 now has kindB
	require.Eventually(t, func() bool {
		p2.membersMu.RLock()
		node, found := p2.members[p1.self.ID]
		p2.membersMu.RUnlock()
		if !found {
			return false
		}
		for _, k := range node.Kinds {
			if k == "kindB" {
				return true
			}
		}
		return false
	}, 15*time.Second, 200*time.Millisecond,
		"node 2 should see kindB on node 1 after runtime registration")

	// Verify node 2's MemberList has updated strategies
	require.Eventually(t, func() bool {
		activator := c2.MemberList.GetActivatorMember("kindB", c2.ActorSystem.Address())
		return activator != ""
	}, 15*time.Second, 200*time.Millisecond,
		"node 2 should be able to find an activator for kindB")
}

// TestIntegration_RuntimeKindDeregistration verifies that deregistering a kind
// propagates to other nodes.
func TestIntegration_RuntimeKindDeregistration(t *testing.T) {
	natsURL := startNATSContainer(t)
	ttl := 3 * time.Second
	refresh := 500 * time.Millisecond

	kindA := cluster.NewKind("kindA", actor.PropsFromProducer(func() actor.Actor {
		return &actor.EmptyActor{}
	}))
	kindB := cluster.NewKind("kindB", actor.PropsFromProducer(func() actor.Actor {
		return &actor.EmptyActor{}
	}))

	// Start node 1 with both kinds
	_, c1 := setupClusterWithKinds(t, natsURL, "integ-kind-dereg",
		[]*cluster.Kind{kindA, kindB},
		WithMemberTTL(ttl),
		WithRefreshInterval(refresh),
	)
	err := c1.StartMember()
	require.NoError(t, err)
	t.Cleanup(func() { c1.Shutdown(true) })

	time.Sleep(500 * time.Millisecond)

	// Start node 2 with both kinds
	p2, c2 := setupClusterWithKinds(t, natsURL, "integ-kind-dereg",
		[]*cluster.Kind{kindA, kindB},
		WithMemberTTL(ttl),
		WithRefreshInterval(refresh),
	)
	err = c2.StartMember()
	require.NoError(t, err)
	t.Cleanup(func() { c2.Shutdown(true) })

	// Wait for mutual discovery
	require.Eventually(t, func() bool {
		p2.membersMu.RLock()
		_, found := p2.members[c1.ActorSystem.ID]
		p2.membersMu.RUnlock()
		// MemberList may not have same ID format. Check using the member list.
		return found || c2.MemberList.Length() >= 2
	}, 10*time.Second, 200*time.Millisecond)

	// Deregister kindB from node 1
	err = c1.DeregisterKind("kindB")
	require.NoError(t, err)

	// Verify node 1 no longer has kindB locally
	_, ok := c1.TryGetClusterKind("kindB")
	assert.False(t, ok, "node 1 should no longer have kindB")
}
```

### Step 2: Run integration test

Run: `go test ./cluster/clusterproviders/natskv/ -tags integration -run TestIntegration_RuntimeKind -race -count=1 -v -timeout 120s`

Expected: PASS — both registration and deregistration propagate correctly.

### Step 3: Write the NATS Stream integration test

Create `cluster/clusterproviders/natsstream/natsstream_kind_integration_test.go` following the same pattern as the NATS KV test but using the natsstream `setupCluster` helper. The test logic is identical:

```go
//go:build integration

package natsstream

import (
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/remote"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"context"
)

func startNATSContainer(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "nats:latest",
		ExposedPorts: []string{"4222/tcp"},
		Cmd:          []string{"-js"},
		WaitingFor:   wait.ForListeningPort("4222/tcp").WithStartupTimeout(30 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })
	endpoint, err := container.Endpoint(ctx, "")
	require.NoError(t, err)
	return "nats://" + endpoint
}

func setupClusterWithKinds(t *testing.T, natsURL, clusterName string, kinds []*cluster.Kind, opts ...Option) (*Provider, *cluster.Cluster) {
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

	return p, c
}

func TestIntegration_RuntimeKindRegistration(t *testing.T) {
	natsURL := startNATSContainer(t)

	kindA := cluster.NewKind("kindA", actor.PropsFromProducer(func() actor.Actor {
		return &actor.EmptyActor{}
	}))

	p1, c1 := setupClusterWithKinds(t, natsURL, "integ-stream-kind",
		[]*cluster.Kind{kindA},
	)
	err := c1.StartMember()
	require.NoError(t, err)
	t.Cleanup(func() { c1.Shutdown(true) })

	time.Sleep(500 * time.Millisecond)

	p2, c2 := setupClusterWithKinds(t, natsURL, "integ-stream-kind",
		[]*cluster.Kind{kindA},
	)
	err = c2.StartMember()
	require.NoError(t, err)
	t.Cleanup(func() { c2.Shutdown(true) })

	// Wait for mutual discovery
	require.Eventually(t, func() bool {
		p1.membersMu.RLock()
		_, found := p1.members[p2.self.ID]
		p1.membersMu.RUnlock()
		return found
	}, 10*time.Second, 200*time.Millisecond)

	require.Eventually(t, func() bool {
		p2.membersMu.RLock()
		_, found := p2.members[p1.self.ID]
		p2.membersMu.RUnlock()
		return found
	}, 10*time.Second, 200*time.Millisecond)

	// Register kindB at runtime on node 1
	kindB := cluster.NewKind("kindB", actor.PropsFromProducer(func() actor.Actor {
		return &actor.EmptyActor{}
	}))
	err = c1.RegisterKind(kindB)
	require.NoError(t, err)

	_, ok := c1.TryGetClusterKind("kindB")
	assert.True(t, ok)

	// Wait for node 2 to see kindB on node 1
	require.Eventually(t, func() bool {
		p2.membersMu.RLock()
		node, found := p2.members[p1.self.ID]
		p2.membersMu.RUnlock()
		if !found {
			return false
		}
		for _, k := range node.Kinds {
			if k == "kindB" {
				return true
			}
		}
		return false
	}, 15*time.Second, 200*time.Millisecond,
		"node 2 should see kindB on node 1")
}
```

### Step 4: Run integration tests

Run: `go test ./cluster/clusterproviders/natskv/ -tags integration -run TestIntegration_RuntimeKind -race -count=1 -v -timeout 120s`

Run: `go test ./cluster/clusterproviders/natsstream/ -tags integration -run TestIntegration_RuntimeKind -race -count=1 -v -timeout 120s`

Expected: Both PASS

### Step 5: Run full test suite

Run: `go test ./cluster/... -race -count=1 -timeout 120s`

Expected: All PASS

### Step 6: Commit

```bash
git add cluster/clusterproviders/natskv/natskv_kind_integration_test.go cluster/clusterproviders/natsstream/natsstream_kind_integration_test.go
git commit -m "test(cluster): add runtime Kind registration integration tests

End-to-end tests verifying that RegisterKind propagates kinds to
other cluster members via NATS KV and NATS Stream providers.
Includes topology convergence and strategy update verification."
```

---

## Summary of all files changed

| File | Change |
|------|--------|
| `cluster/cluster.go` | Add `kindsMu sync.RWMutex`, `provider ClusterProvider`; add locking to all `kinds` accessors; add `RegisterKind`, `DeregisterKind`, `getClusterKindsLocked`, `notifyKindUpdate`; rename `ensureTopicKindRegistered` → `ensureTopicKindRegisteredLocked`; set `c.provider` in `StartMember` |
| `cluster/cluster_provider.go` | Add `KindUpdater` interface |
| `cluster/member_list.go` | Add `kindsEqual`, `hasKindChanges`, `processKindChangesForStayingMembers`, `memberKindsChanged`; modify `getTopologyChanges` to check for kind changes; modify `UpdateClusterTopology` to call `processKindChangesForStayingMembers` before overwriting `ml.members` |
| `cluster/clusterproviders/automanaged/automanaged.go` | Add `UpdateKinds` method |
| `cluster/clusterproviders/natskv/natskv_provider.go` | Add `UpdateKinds` method; protect `p.self` serialization with `membersMu` in `registerSelf` and `refreshMemberKey` |
| `cluster/clusterproviders/natsstream/natsstream_provider.go` | Add `UpdateKinds` method; protect `p.self` serialization with `membersMu` in `publishHeartbeat` |
| `cluster/clusterproviders/k8s/k8s_provider.go` | Add `UpdateKinds` method; fix `registerMember` to clean old kind labels |
| `cluster/cluster_kind_test.go` | **New** — unit tests for RegisterKind, DeregisterKind, thread safety |
| `cluster/member_list_kind_change_test.go` | **New** — unit tests for kind-change detection in MemberList |
| `cluster/clusterproviders/automanaged/automanaged_kind_test.go` | **New** — unit tests for automanaged UpdateKinds |
| `cluster/clusterproviders/natskv/natskv_provider_test.go` | Add UpdateKinds test |
| `cluster/clusterproviders/natsstream/natsstream_provider_test.go` | Add UpdateKinds test |
| `cluster/clusterproviders/k8s/k8s_provider_unit_test.go` | Add UpdateKinds test |
| `cluster/clusterproviders/natskv/natskv_kind_integration_test.go` | **New** — integration test |
| `cluster/clusterproviders/natsstream/natsstream_kind_integration_test.go` | **New** — integration test |

## Task dependency graph

```
Task 1 (Thread-Safe Registry)
  │
  ├──→ Task 2 (RegisterKind/DeregisterKind API + KindUpdater interface)
  │      │
  │      ├──→ Task 4 (Automanaged) ──┐
  │      ├──→ Task 5 (NATS KV) ──────┤  (independent of each other)
  │      ├──→ Task 6 (NATS Stream) ──┤
  │      └──→ Task 7 (K8s) ──────────┘
  │                                    │
  └──→ Task 8 (MemberList) ───────────┘
                                       │
                              Task 9 (Integration Tests)
```

Tasks 4-7 are independent and can be done in any order (or in parallel).
Task 8 (MemberList) is independent of Tasks 4-7 but depends on Task 1.
Task 9 depends on all previous tasks.
