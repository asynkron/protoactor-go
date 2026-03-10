# Grain Introspection & Registry Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add grain enumeration, lifecycle events, per-grain metrics, and member stats to protoactor-go across all identity lookup backends.

**Architecture:** New opt-in interfaces (`GrainEnumerator`, `StorageGrainEnumerator`) let identity lookups expose grain listing. A `GrainRegistry` on `Cluster` delegates to these. Lifecycle events (`GrainActivated`, `GrainDeactivated`) are published from the existing cluster middleware. Opt-in per-grain metrics track `LastMessageAt` and `MessageCount` via a receiver middleware gated by `WithGrainMetrics()`.

**Tech Stack:** Go, protobuf (existing), testify, testcontainers (for NATS integration tests), Redis go-redis, PostgreSQL pgx, NATS JetStream.

**Spec:** `docs/plans/2026-03-10-grain-introspection-design.md`

---

## Chunk 1: Core Types, Interfaces, and Lifecycle Events

### Task 1: Add GrainEnumerator and StorageGrainEnumerator interfaces

**Files:**
- Modify: `cluster/identity_lookup.go`

These interfaces are opt-in — implementations that support grain listing implement them in addition to `IdentityLookup` or `StorageLookup`.

- [ ] **Step 1: Add the new types and interfaces to identity_lookup.go**

Add after the existing `StorageLookup` interface (after line 33) in `cluster/identity_lookup.go`:

```go
// StoredActivationInfo describes a stored activation with parsed identity fields.
type StoredActivationInfo struct {
	Identity string
	Kind     string
	Pid      string // "address/id" format
	MemberID string
}

// GrainEnumerator is an optional interface that IdentityLookup implementations
// may implement to support listing active grain activations.
type GrainEnumerator interface {
	// ListGrains returns all known grain activations.
	ListGrains() ([]*GrainInfo, error)
	// ListGrainsByKind returns grains filtered by kind.
	ListGrainsByKind(kind string) ([]*GrainInfo, error)
	// ListGrainsByMember returns grains owned by a specific member.
	ListGrainsByMember(memberID string) ([]*GrainInfo, error)
}

// StorageGrainEnumerator is an optional interface that StorageLookup backends
// may implement to support listing stored activations.
type StorageGrainEnumerator interface {
	// ListActivations returns all stored activations.
	ListActivations() ([]*StoredActivationInfo, error)
	// ListActivationsByMember returns activations belonging to a specific member.
	ListActivationsByMember(memberID string) ([]*StoredActivationInfo, error)
}
```

- [ ] **Step 2: Verify the project compiles**

Run: `cd /home/cchamplin/development/protoactor-go && go build ./cluster/...`
Expected: Success (no code depends on these interfaces yet)

- [ ] **Step 3: Commit**

```bash
git add cluster/identity_lookup.go
git commit -m "feat(cluster): add GrainEnumerator and StorageGrainEnumerator interfaces"
```

---

### Task 2: Add GrainInfo type and GrainActivated / GrainDeactivated events

**Files:**
- Create: `cluster/grain_events.go`
- Create: `cluster/grain_events_test.go`

- [ ] **Step 1: Create grain_events.go with event types**

Create `cluster/grain_events.go`:

```go
package cluster

import (
	"sync"
	"time"

	"github.com/asynkron/protoactor-go/actor"
)

// GrainInfo describes an active grain activation.
type GrainInfo struct {
	Identity      string
	Kind          string
	PID           *actor.PID
	MemberID      string
	ActivatedAt   time.Time // zero if unknown (e.g. remote grains from storage)
	LastMessageAt time.Time // zero unless WithGrainMetrics() enabled
	MessageCount  int64     // zero unless WithGrainMetrics() enabled
}

// GrainActivated is published to the EventStream when a grain actor starts.
type GrainActivated struct {
	ClusterIdentity *ClusterIdentity
	PID             *actor.PID
}

// DeactivationReason indicates why a grain was deactivated.
type DeactivationReason int

const (
	// DeactivationReasonUnknown is the default when the reason cannot be determined.
	DeactivationReasonUnknown DeactivationReason = iota
	// DeactivationReasonPassivation means the grain was idle and passivated.
	DeactivationReasonPassivation
	// DeactivationReasonShutdown means the cluster or actor was explicitly shut down.
	DeactivationReasonShutdown
	// DeactivationReasonTopologyChange means the grain was moved due to member join/leave.
	DeactivationReasonTopologyChange
)

// String returns a human-readable name for the deactivation reason.
func (r DeactivationReason) String() string {
	switch r {
	case DeactivationReasonPassivation:
		return "passivation"
	case DeactivationReasonShutdown:
		return "shutdown"
	case DeactivationReasonTopologyChange:
		return "topology-change"
	default:
		return "unknown"
	}
}

// GrainDeactivated is published to the EventStream when a grain actor stops.
// The existing ActivationTerminating event is still published for backward compatibility.
type GrainDeactivated struct {
	ClusterIdentity *ClusterIdentity
	PID             *actor.PID
	Reason          DeactivationReason
}

// deactivationReasons tracks the reason a grain is being deactivated.
// The reason is set before stopping the actor (from placement actor topology
// changes, passivation timer, or cluster shutdown), and read from the
// handleStopped middleware. Uses a sync.Map because setters and readers
// are on different goroutines.
type deactivationReasons struct {
	m sync.Map // map[pidKey]DeactivationReason
}

func newDeactivationReasons() *deactivationReasons {
	return &deactivationReasons{}
}

// pidKey returns a string key for a PID suitable for map lookups.
func pidKey(pid *actor.PID) string {
	return pid.Address + "/" + pid.Id
}

// Set records the deactivation reason for the given PID.
func (d *deactivationReasons) Set(pid *actor.PID, reason DeactivationReason) {
	d.m.Store(pidKey(pid), reason)
}

// Pop retrieves and removes the deactivation reason for the given PID.
// Returns DeactivationReasonUnknown if no reason was set.
func (d *deactivationReasons) Pop(pid *actor.PID) DeactivationReason {
	key := pidKey(pid)
	v, ok := d.m.LoadAndDelete(key)
	if !ok {
		return DeactivationReasonUnknown
	}
	return v.(DeactivationReason)
}
```

- [ ] **Step 2: Write tests for DeactivationReason and deactivationReasons**

Create `cluster/grain_events_test.go`:

```go
package cluster

import (
	"testing"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

func TestDeactivationReason_String(t *testing.T) {
	tests := []struct {
		reason DeactivationReason
		want   string
	}{
		{DeactivationReasonUnknown, "unknown"},
		{DeactivationReasonPassivation, "passivation"},
		{DeactivationReasonShutdown, "shutdown"},
		{DeactivationReasonTopologyChange, "topology-change"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.reason.String())
	}
}

func TestDeactivationReasons_SetAndPop(t *testing.T) {
	dr := newDeactivationReasons()
	pid := actor.NewPID("127.0.0.1:8080", "test-actor")

	// Pop without Set returns Unknown.
	assert.Equal(t, DeactivationReasonUnknown, dr.Pop(pid))

	// Set then Pop returns the reason.
	dr.Set(pid, DeactivationReasonPassivation)
	assert.Equal(t, DeactivationReasonPassivation, dr.Pop(pid))

	// Second Pop returns Unknown (entry was deleted).
	assert.Equal(t, DeactivationReasonUnknown, dr.Pop(pid))
}

func TestDeactivationReasons_MultiplePIDs(t *testing.T) {
	dr := newDeactivationReasons()
	pid1 := actor.NewPID("127.0.0.1:8080", "actor-1")
	pid2 := actor.NewPID("127.0.0.1:8080", "actor-2")

	dr.Set(pid1, DeactivationReasonShutdown)
	dr.Set(pid2, DeactivationReasonTopologyChange)

	assert.Equal(t, DeactivationReasonShutdown, dr.Pop(pid1))
	assert.Equal(t, DeactivationReasonTopologyChange, dr.Pop(pid2))
}
```

- [ ] **Step 3: Run tests to verify they pass**

Run: `cd /home/cchamplin/development/protoactor-go && go test -race ./cluster/ -run TestDeactivation -v`
Expected: All 3 tests PASS

- [ ] **Step 4: Commit**

```bash
git add cluster/grain_events.go cluster/grain_events_test.go
git commit -m "feat(cluster): add GrainInfo, GrainActivated, GrainDeactivated event types"
```

---

### Task 3: Wire lifecycle events into cluster middleware

**Files:**
- Modify: `cluster/cluster.go` (lines 22-38, add deactivationReasons field)
- Modify: `cluster/config.go` (lines 145-175, handleStarted/handleStopped)

- [ ] **Step 1: Add deactivationReasons field to Cluster struct**

In `cluster/cluster.go`, add a new field to the `Cluster` struct (after line 37, `metricsEnabled bool`):

```go
	deactivationReasons *deactivationReasons
```

In `NewCluster` function (after line 57, `c.PidCache = NewPidCache()`), add:

```go
	c.deactivationReasons = newDeactivationReasons()
```

Add a public method for external code (placement actors, passivation) to set deactivation reasons. Add after the `Logger()` method (after line 367):

```go
// SetDeactivationReason records why a grain is being deactivated.
// This must be called before stopping the grain actor. The reason is
// consumed by the handleStopped middleware and published in GrainDeactivated.
func (c *Cluster) SetDeactivationReason(pid *actor.PID, reason DeactivationReason) {
	c.deactivationReasons.Set(pid, reason)
}
```

- [ ] **Step 2: Modify handleStarted to publish GrainActivated**

In `cluster/config.go`, modify `handleStarted` (lines 163-175). After the existing code that sends `ClusterInit`, add the `GrainActivated` publish. The function should become:

```go
func handleStarted(c actor.ReceiverContext, next actor.ReceiverFunc, envelope *actor.MessageEnvelope) {
	next(c, envelope)
	cl := GetCluster(c.ActorSystem())
	identity := GetClusterIdentity(c)

	grainInit := &ClusterInit{
		Identity: identity,
		Cluster:  cl,
	}

	ge := actor.WrapEnvelope(grainInit)
	next(c, ge)

	if identity != nil {
		cl.ActorSystem.EventStream.Publish(&GrainActivated{
			ClusterIdentity: identity,
			PID:             c.Self(),
		})
	}
}
```

- [ ] **Step 3: Modify handleStopped to publish GrainDeactivated**

In `cluster/config.go`, modify `handleStopped` (lines 145-161). After the existing `ActivationTerminating` publish (which stays for backward compatibility), add the `GrainDeactivated` publish:

```go
func handleStopped(c actor.ReceiverContext, next actor.ReceiverFunc, envelope *actor.MessageEnvelope) {
	cl := GetCluster(c.ActorSystem())
	identity := GetClusterIdentity(c)

	if identity != nil {
		// Existing event — kept for backward compatibility.
		cl.ActorSystem.EventStream.Publish(&ActivationTerminating{
			Pid:             c.Self(),
			ClusterIdentity: identity,
		})
		cl.PidCache.RemoveByValue(identity.Identity, identity.Kind, c.Self())

		// New enriched event with deactivation reason.
		reason := cl.deactivationReasons.Pop(c.Self())
		cl.ActorSystem.EventStream.Publish(&GrainDeactivated{
			ClusterIdentity: identity,
			PID:             c.Self(),
			Reason:          reason,
		})
	}

	next(c, envelope)
}
```

- [ ] **Step 4: Write test for lifecycle events**

Add to `cluster/grain_events_test.go`:

```go
func TestGrainActivated_EventPublished(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("test-cluster",
		newInmemoryProvider(),
		disthash.New(),
		remote.Configure("127.0.0.1", 0),
		WithKinds(NewKind("test-kind", actor.PropsFromFunc(func(ctx actor.Context) {}))),
	)
	cl := NewCluster(system, config)

	var activated *GrainActivated
	var deactivated *GrainDeactivated
	var wgActivated, wgDeactivated sync.WaitGroup
	wgActivated.Add(1)
	wgDeactivated.Add(1)

	system.EventStream.Subscribe(func(evt any) {
		switch e := evt.(type) {
		case *GrainActivated:
			activated = e
			wgActivated.Done()
		case *GrainDeactivated:
			deactivated = e
			wgDeactivated.Done()
		}
	})

	err := cl.StartMember()
	require.NoError(t, err)
	defer cl.Shutdown(true)

	// Trigger a grain activation.
	pid := cl.Get("test-identity", "test-kind")
	require.NotNil(t, pid)

	// Wait for activation event.
	waitCh := make(chan struct{})
	go func() { wgActivated.Wait(); close(waitCh) }()
	select {
	case <-waitCh:
	case <-time.After(5 * time.Second):
		t.Fatal("GrainActivated event not received within 5s")
	}

	assert.Equal(t, "test-identity", activated.ClusterIdentity.Identity)
	assert.Equal(t, "test-kind", activated.ClusterIdentity.Kind)
	assert.NotNil(t, activated.PID)

	// Stop the grain to trigger deactivation.
	system.Root.Poison(pid)

	waitCh2 := make(chan struct{})
	go func() { wgDeactivated.Wait(); close(waitCh2) }()
	select {
	case <-waitCh2:
	case <-time.After(5 * time.Second):
		t.Fatal("GrainDeactivated event not received within 5s")
	}

	assert.Equal(t, "test-identity", deactivated.ClusterIdentity.Identity)
	assert.Equal(t, "test-kind", deactivated.ClusterIdentity.Kind)
	assert.Equal(t, DeactivationReasonUnknown, deactivated.Reason)
}
```

Note: This test requires importing `sync`, `time`, `disthash`, and `remote` packages and having `newInmemoryProvider()` available (check the test helpers in the cluster package — look at existing test files for how clusters are created in tests). Adapt imports and helpers as needed.

- [ ] **Step 5: Run tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test -race ./cluster/ -run TestGrainActivated -v -timeout 30s`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add cluster/cluster.go cluster/config.go cluster/grain_events.go cluster/grain_events_test.go
git commit -m "feat(cluster): publish GrainActivated and GrainDeactivated events from middleware"
```

---

### Task 4: Wire deactivation reasons from placement actor and cluster shutdown

**Files:**
- Modify: `cluster/identitylookup/disthash/placement_actor.go` (lines 179-195, onClusterTopology; lines 81-94, onStopping)
- Modify: `cluster/cluster.go` (line 197-216, Shutdown method)

- [ ] **Step 1: Set topology-change reason in placement actor onClusterTopology**

In `cluster/identitylookup/disthash/placement_actor.go`, modify `onClusterTopology` (line 193). Before `ctx.Poison(meta.PID)`, set the deactivation reason:

```go
func (p *placementActor) onClusterTopology(msg *clustering.ClusterTopology, ctx actor.Context) {
	rdv := clustering.NewRendezvous()
	rdv.UpdateMembers(msg.Members)
	myAddress := p.cluster.ActorSystem.Address()
	for identity, meta := range p.actors {
		ownerAddress := rdv.GetByIdentity(identity)
		if ownerAddress == myAddress {
			ctx.Logger().Debug("Actor stays", slog.String("identity", identity), slog.String("owner", ownerAddress), slog.String("me", myAddress))
			continue
		}

		ctx.Logger().Debug("Actor moved", slog.String("identity", identity), slog.String("owner", ownerAddress), slog.String("me", myAddress))

		p.cluster.SetDeactivationReason(meta.PID, clustering.DeactivationReasonTopologyChange)
		ctx.Poison(meta.PID)
	}
}
```

- [ ] **Step 2: Set shutdown reason in placement actor onStopping**

In `cluster/identitylookup/disthash/placement_actor.go`, modify `onStopping` (lines 81-94). Before poisoning actors, set the shutdown reason:

```go
func (p *placementActor) onStopping(ctx actor.Context) {
	futures := make(map[string]actor.Future, len(p.actors))

	for key, meta := range p.actors {
		p.cluster.SetDeactivationReason(meta.PID, clustering.DeactivationReasonShutdown)
		futures[key] = ctx.PoisonFuture(meta.PID)
	}

	for key, future := range futures {
		err := future.Wait()
		if err != nil {
			ctx.Logger().Error("Failed to poison actor", slog.String("identity", key), slog.Any("error", err))
		}
	}
}
```

- [ ] **Step 3: Set shutdown reason in Cluster.Shutdown**

In `cluster/cluster.go`, at the beginning of the `Shutdown` method (after line 198), add a flag that the `handleStopped` middleware can use as a fallback. Actually, the placement actor's `onStopping` already handles this for disthash. For storage-based lookups where grains are stopped via `IdentityLookup.Shutdown()`, the storage lookup doesn't directly stop actors — it just removes records. The actors are stopped when the ActorSystem shuts down. We need to mark all remaining grains before `c.ActorSystem.Shutdown()`:

No additional change needed here — the placement actor's `onStopping` handles disthash, and for storage-backed lookups, the `ActorSystem.Shutdown()` stops actors generically (without going through the placement actor). Those will get `DeactivationReasonUnknown` which is acceptable.

- [ ] **Step 4: Verify compilation**

Run: `cd /home/cchamplin/development/protoactor-go && go build ./...`
Expected: Success

- [ ] **Step 5: Commit**

```bash
git add cluster/identitylookup/disthash/placement_actor.go
git commit -m "feat(cluster): set deactivation reasons in placement actor for topology and shutdown"
```

---

### Task 5: Wire passivation deactivation reason

**Files:**
- Modify: `plugin/passivation.go` (lines 36-44, Init method goroutine)

The passivation plugin calls `actorSystem.Root.Stop(pid)` from a goroutine when the idle timer fires. We need to set the deactivation reason before stopping. The passivation plugin doesn't have a direct reference to the `Cluster`, but it has the `ActorSystem`. We can get the cluster via `cluster.GetCluster(actorSystem)`.

- [ ] **Step 1: Modify PassivationHolder.Init to set deactivation reason**

In `plugin/passivation.go`, modify the `Init` method. Add the import for the cluster package and set the reason before stopping:

```go
func (state *PassivationHolder) Init(actorSystem *actor.ActorSystem, pid *actor.PID, duration time.Duration) {
	state.timer = time.NewTimer(duration)
	state.done = 0
	state.doneCh = make(chan struct{})
	go func() {
		select {
		case <-state.timer.C:
			// Set passivation reason if this is a cluster grain.
			if cl := cluster.GetCluster(actorSystem); cl != nil {
				cl.SetDeactivationReason(pid, cluster.DeactivationReasonPassivation)
			}
			actorSystem.Root.Stop(pid)
			atomic.StoreInt32(&state.done, 1)
		case <-state.doneCh:
			return
		}
	}()
}
```

Add import: `clustering "github.com/asynkron/protoactor-go/cluster"`

**Important:** Check for import cycles. The `plugin` package importing `cluster` should be fine since `plugin` is a separate package. Verify:

Run: `cd /home/cchamplin/development/protoactor-go && go build ./plugin/...`

If there's a circular dependency, an alternative approach is to accept a `func(*actor.PID)` callback in `PassivationHolder.Init` that the cluster middleware sets. But check first — it should be fine since `cluster` doesn't import `plugin`.

- [ ] **Step 2: Run existing passivation tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test -race ./plugin/... -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add plugin/passivation.go
git commit -m "feat(cluster): set DeactivationReasonPassivation in passivation plugin"
```

---

## Chunk 2: Opt-In Per-Grain Metrics, GrainRegistry, and MemberStats

### Task 6: Implement grain metrics store (must be before GrainRegistry)

**Files:**
- Create: `cluster/grain_metrics.go`
- Create: `cluster/grain_metrics_test.go`
- Modify: `cluster/config.go:13` (add GrainMetricsEnabled to Config struct)
- Modify: `cluster/config_opts.go` (add WithGrainMetrics option)
- Modify: `cluster/cluster.go` (add grainMetrics field)

The grain metrics store type must exist before GrainRegistry (Task 7) since the registry references it. The middleware wiring is done in Task 8 after both are in place.

- [ ] **Step 1: Create grain_metrics.go**

Create `cluster/grain_metrics.go`:

```go
package cluster

import (
	"sync"
	"sync/atomic"
	"time"
)

// grainMetricsEntry stores per-grain message statistics.
type grainMetricsEntry struct {
	lastMessageAt atomic.Int64 // unix nanoseconds
	messageCount  atomic.Int64
}

// grainMetricsStore is a concurrent map of identity key → metrics entry.
type grainMetricsStore struct {
	entries sync.Map // map[string]*grainMetricsEntry
}

func newGrainMetricsStore() *grainMetricsStore {
	return &grainMetricsStore{}
}

// Record updates the metrics for a grain identified by its key.
func (s *grainMetricsStore) Record(key string) {
	v, _ := s.entries.LoadOrStore(key, &grainMetricsEntry{})
	entry := v.(*grainMetricsEntry)
	entry.lastMessageAt.Store(time.Now().UnixNano())
	entry.messageCount.Add(1)
}

// Remove deletes the metrics entry for a grain.
func (s *grainMetricsStore) Remove(key string) {
	s.entries.Delete(key)
}

// applyTo merges the stored metrics into a GrainInfo.
func (s *grainMetricsStore) applyTo(key string, info *GrainInfo) {
	v, ok := s.entries.Load(key)
	if !ok {
		return
	}
	entry := v.(*grainMetricsEntry)
	if nanos := entry.lastMessageAt.Load(); nanos > 0 {
		info.LastMessageAt = time.Unix(0, nanos)
	}
	info.MessageCount = entry.messageCount.Load()
}
```

- [ ] **Step 2: Add GrainMetricsEnabled to Config and WithGrainMetrics option**

In `cluster/config.go`, add `GrainMetricsEnabled` field to the `Config` struct (after `PubSubConfig`):

```go
	GrainMetricsEnabled bool
```

In `cluster/config_opts.go`, add at the end:

```go
// WithGrainMetrics enables per-grain metrics tracking (LastMessageAt, MessageCount).
// This adds a small overhead to each message processed by a grain.
func WithGrainMetrics() ConfigOption {
	return func(c *Config) {
		c.GrainMetricsEnabled = true
	}
}
```

- [ ] **Step 3: Add grainMetrics field to Cluster and initialize**

In `cluster/cluster.go`, add to the `Cluster` struct (after `deactivationReasons`):

```go
	grainMetrics *grainMetricsStore // nil unless WithGrainMetrics() is set
```

In `NewCluster`, after `c.deactivationReasons = newDeactivationReasons()`:

```go
	if config.GrainMetricsEnabled {
		c.grainMetrics = newGrainMetricsStore()
	}
```

- [ ] **Step 4: Write tests for grain metrics store**

Create `cluster/grain_metrics_test.go`:

```go
package cluster

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGrainMetricsStore_Record(t *testing.T) {
	store := newGrainMetricsStore()

	before := time.Now()
	store.Record("kind/identity")
	store.Record("kind/identity")
	store.Record("kind/identity")
	after := time.Now()

	info := &GrainInfo{}
	store.applyTo("kind/identity", info)

	assert.Equal(t, int64(3), info.MessageCount)
	assert.False(t, info.LastMessageAt.IsZero())
	assert.True(t, info.LastMessageAt.After(before) || info.LastMessageAt.Equal(before))
	assert.True(t, info.LastMessageAt.Before(after) || info.LastMessageAt.Equal(after))
}

func TestGrainMetricsStore_Remove(t *testing.T) {
	store := newGrainMetricsStore()

	store.Record("kind/identity")
	store.Remove("kind/identity")

	info := &GrainInfo{}
	store.applyTo("kind/identity", info)

	assert.Equal(t, int64(0), info.MessageCount)
	assert.True(t, info.LastMessageAt.IsZero())
}

func TestGrainMetricsStore_ApplyToUnknownKey(t *testing.T) {
	store := newGrainMetricsStore()

	info := &GrainInfo{}
	store.applyTo("unknown/key", info)

	assert.Equal(t, int64(0), info.MessageCount)
	assert.True(t, info.LastMessageAt.IsZero())
}

func TestGrainMetricsStore_ConcurrentAccess(t *testing.T) {
	store := newGrainMetricsStore()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.Record("kind/concurrent")
		}()
	}
	wg.Wait()

	info := &GrainInfo{}
	store.applyTo("kind/concurrent", info)
	assert.Equal(t, int64(100), info.MessageCount)
}
```

- [ ] **Step 5: Run tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test -race ./cluster/ -run TestGrainMetrics -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add cluster/grain_metrics.go cluster/grain_metrics_test.go cluster/config.go cluster/config_opts.go cluster/cluster.go
git commit -m "feat(cluster): add grain metrics store and WithGrainMetrics config option"
```

---

### Task 7: Implement GrainRegistry

**Files:**
- Create: `cluster/grain_registry.go`
- Create: `cluster/grain_registry_test.go`
- Modify: `cluster/cluster.go` (add grainRegistry field and accessor)

- [ ] **Step 1: Create grain_registry.go**

Create `cluster/grain_registry.go`:

```go
package cluster

import (
	"errors"
	"strings"

	"github.com/asynkron/protoactor-go/actor"
)

// ErrEnumerationNotSupported is returned by GrainRegistry methods that require
// the IdentityLookup to implement GrainEnumerator.
var ErrEnumerationNotSupported = errors.New("identity lookup does not support grain enumeration")

// GrainRegistry provides read-only access to grain activation information.
// Count and CountByKind always work. Enumeration methods (All, ByKind,
// ByMember, Get) require the IdentityLookup to implement GrainEnumerator.
type GrainRegistry struct {
	cluster *Cluster
}

// Count returns the total number of active grains on the local node.
func (r *GrainRegistry) Count() int {
	return int(r.cluster.VirtualActorCount())
}

// CountByKind returns a map of kind name to active grain count on the local node.
func (r *GrainRegistry) CountByKind() map[string]int {
	r.cluster.kindsMu.RLock()
	defer r.cluster.kindsMu.RUnlock()

	result := make(map[string]int, len(r.cluster.kinds))
	for name, ak := range r.cluster.kinds {
		result[name] = int(ak.Count())
	}
	return result
}

// enumerator returns the GrainEnumerator if the identity lookup supports it.
func (r *GrainRegistry) enumerator() (GrainEnumerator, error) {
	if enum, ok := r.cluster.IdentityLookup.(GrainEnumerator); ok {
		return enum, nil
	}
	return nil, ErrEnumerationNotSupported
}

// All returns all known grain activations. Returns ErrEnumerationNotSupported
// if the identity lookup does not implement GrainEnumerator.
func (r *GrainRegistry) All() ([]*GrainInfo, error) {
	enum, err := r.enumerator()
	if err != nil {
		return nil, err
	}
	grains, err := enum.ListGrains()
	if err != nil {
		return nil, err
	}
	r.enrichWithMetrics(grains)
	return grains, nil
}

// ByKind returns grain activations filtered by kind. Returns
// ErrEnumerationNotSupported if the identity lookup does not implement
// GrainEnumerator.
func (r *GrainRegistry) ByKind(kind string) ([]*GrainInfo, error) {
	enum, err := r.enumerator()
	if err != nil {
		return nil, err
	}
	grains, err := enum.ListGrainsByKind(kind)
	if err != nil {
		return nil, err
	}
	r.enrichWithMetrics(grains)
	return grains, nil
}

// ByMember returns grain activations owned by the specified member. Returns
// ErrEnumerationNotSupported if the identity lookup does not implement
// GrainEnumerator.
func (r *GrainRegistry) ByMember(memberID string) ([]*GrainInfo, error) {
	enum, err := r.enumerator()
	if err != nil {
		return nil, err
	}
	grains, err := enum.ListGrainsByMember(memberID)
	if err != nil {
		return nil, err
	}
	r.enrichWithMetrics(grains)
	return grains, nil
}

// Get returns information about a specific grain. Returns
// ErrEnumerationNotSupported if the identity lookup does not implement
// GrainEnumerator.
func (r *GrainRegistry) Get(identity, kind string) (*GrainInfo, bool, error) {
	enum, err := r.enumerator()
	if err != nil {
		return nil, false, err
	}
	grains, err := enum.ListGrainsByKind(kind)
	if err != nil {
		return nil, false, err
	}
	for _, g := range grains {
		if g.Identity == identity {
			r.enrichWithMetrics([]*GrainInfo{g})
			return g, true, nil
		}
	}
	return nil, false, nil
}

// enrichWithMetrics merges per-grain metrics (LastMessageAt, MessageCount)
// into the GrainInfo slice if grain metrics are enabled.
func (r *GrainRegistry) enrichWithMetrics(grains []*GrainInfo) {
	if r.cluster.grainMetrics == nil {
		return
	}
	for _, g := range grains {
		key := g.Kind + "/" + g.Identity
		r.cluster.grainMetrics.applyTo(key, g)
	}
}

// ParseStoredActivationInfoKey parses a key in "kind/identity" format.
// Returns kind, identity. If the key doesn't contain '/', returns the
// whole key as kind and empty identity.
func ParseStoredActivationInfoKey(key string) (kind, identity string) {
	if idx := strings.Index(key, "/"); idx >= 0 {
		return key[:idx], key[idx+1:]
	}
	return key, ""
}

// ParseDotSeparatedKey parses a key in "kind.identity" format (used by NATS).
// Returns kind, identity. If the key doesn't contain '.', returns the
// whole key as kind and empty identity.
func ParseDotSeparatedKey(key string) (kind, identity string) {
	if idx := strings.Index(key, "."); idx >= 0 {
		return key[:idx], key[idx+1:]
	}
	return key, ""
}

// StoredActivationInfoToGrainInfo converts a StoredActivationInfo to a GrainInfo.
func StoredActivationInfoToGrainInfo(info *StoredActivationInfo) *GrainInfo {
	var pid *actor.PID
	if info.Pid != "" {
		if idx := strings.Index(info.Pid, "/"); idx >= 0 {
			pid = actor.NewPID(info.Pid[:idx], info.Pid[idx+1:])
		}
	}
	return &GrainInfo{
		Identity: info.Identity,
		Kind:     info.Kind,
		PID:      pid,
		MemberID: info.MemberID,
	}
}
```

- [ ] **Step 2: Add grainRegistry field and accessor to Cluster**

In `cluster/cluster.go`, add to the `Cluster` struct (after the `grainMetrics` field):

```go
	grainReg *GrainRegistry
```

In `NewCluster`, after the `grainMetrics` initialization block, add:

```go
	c.grainReg = &GrainRegistry{cluster: c}
```

Add the accessor method after `SetDeactivationReason`:

```go
// GrainRegistry returns the cluster's grain registry for introspection.
func (c *Cluster) GrainRegistry() *GrainRegistry {
	return c.grainReg
}
```

- [ ] **Step 3: Write tests for GrainRegistry.Count and CountByKind**

Create `cluster/grain_registry_test.go`:

```go
package cluster

import (
	"testing"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

func TestGrainRegistry_Count_ReturnsVirtualActorCount(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("test-cluster",
		newInmemoryProvider(),
		&mockIdentityLookup{},
		remote.Configure("127.0.0.1", 0),
	)
	cl := NewCluster(system, config)

	// No kinds registered yet, count should be 0.
	assert.Equal(t, 0, cl.GrainRegistry().Count())
}

func TestGrainRegistry_CountByKind_ReturnsPerKindCounts(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("test-cluster",
		newInmemoryProvider(),
		&mockIdentityLookup{},
		remote.Configure("127.0.0.1", 0),
		WithKinds(NewKind("kind-a", actor.PropsFromFunc(func(ctx actor.Context) {}))),
		WithKinds(NewKind("kind-b", actor.PropsFromFunc(func(ctx actor.Context) {}))),
	)
	cl := NewCluster(system, config)
	cl.initKinds()

	result := cl.GrainRegistry().CountByKind()
	assert.Contains(t, result, "kind-a")
	assert.Contains(t, result, "kind-b")
}

func TestGrainRegistry_All_ReturnsErrorWhenNotSupported(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("test-cluster",
		newInmemoryProvider(),
		&mockIdentityLookup{},
		remote.Configure("127.0.0.1", 0),
	)
	cl := NewCluster(system, config)

	_, err := cl.GrainRegistry().All()
	assert.ErrorIs(t, err, ErrEnumerationNotSupported)
}

// mockIdentityLookup is a minimal IdentityLookup that does NOT implement GrainEnumerator.
type mockIdentityLookup struct{}

func (m *mockIdentityLookup) Get(_ *ClusterIdentity) *actor.PID { return nil }
func (m *mockIdentityLookup) RemovePid(_ *ClusterIdentity, _ *actor.PID) {}
func (m *mockIdentityLookup) Setup(_ *Cluster, _ []string, _ bool) {}
func (m *mockIdentityLookup) Shutdown() {}
```

Note: You'll need to adapt imports. The test uses `newInmemoryProvider()` — check existing cluster tests for how this helper is defined (it may be in a test helper file). If it doesn't exist, create a minimal `ClusterProvider` for testing. Also check if `remote` needs to be imported from `github.com/asynkron/protoactor-go/remote`.

- [ ] **Step 4: Run tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test -race ./cluster/ -run TestGrainRegistry -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cluster/grain_registry.go cluster/grain_registry_test.go cluster/cluster.go
git commit -m "feat(cluster): add GrainRegistry with Count, CountByKind, and enumeration delegation"
```

---

### Task 7: Implement MemberStats

**Files:**
- Create: `cluster/member_stats.go`
- Create: `cluster/member_stats_test.go`

- [ ] **Step 1: Create member_stats.go**

Create `cluster/member_stats.go`:

```go
package cluster

// MemberStats contains grain statistics for a cluster member.
type MemberStats struct {
	MemberID   string
	Address    string
	GrainCount int64
	ByKind     map[string]int64
}

// MemberStats returns grain statistics for all cluster members by reading
// the existing gossiped heartbeat data. No new gossip state is introduced.
func (c *Cluster) MemberStats() ([]MemberStats, error) {
	state, err := c.Gossip.GetState(HeartbeatKey)
	if err != nil {
		return nil, err
	}

	members := c.MemberList.Members()
	memberMap := make(map[string]*Member)
	if members != nil {
		for _, m := range members.Members() {
			memberMap[m.Id] = m
		}
	}

	var result []MemberStats
	for memberID, gkv := range state {
		if gkv.Value == nil {
			continue
		}

		var hb MemberHeartbeat
		if err := gkv.Value.UnmarshalTo(&hb); err != nil {
			continue
		}

		stats := MemberStats{
			MemberID: memberID,
			ByKind:   make(map[string]int64),
		}

		// Resolve the member address from the member list.
		if m, ok := memberMap[memberID]; ok {
			stats.Address = m.Address()
		}

		if hb.ActorStatistics != nil && hb.ActorStatistics.ActorCount != nil {
			for kind, count := range hb.ActorStatistics.ActorCount {
				stats.ByKind[kind] = count
				stats.GrainCount += count
			}
		}

		result = append(result, stats)
	}

	return result, nil
}
```

- [ ] **Step 2: Write tests for MemberStats**

Create `cluster/member_stats_test.go`:

```go
package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestMemberStats_ParsesHeartbeatData(t *testing.T) {
	// This test verifies the parsing logic. A full integration test
	// would require a running cluster with gossip.

	hb := &MemberHeartbeat{
		ActorStatistics: &ActorStatistics{
			ActorCount: map[string]int64{
				"kind-a": 5,
				"kind-b": 3,
			},
		},
	}

	anyVal, err := anypb.New(hb)
	require.NoError(t, err)

	gkv := &GossipKeyValue{
		Value: anyVal,
	}

	// Verify we can unmarshal correctly.
	var parsed MemberHeartbeat
	err = gkv.Value.UnmarshalTo(&parsed)
	require.NoError(t, err)

	assert.Equal(t, int64(5), parsed.ActorStatistics.ActorCount["kind-a"])
	assert.Equal(t, int64(3), parsed.ActorStatistics.ActorCount["kind-b"])
}
```

- [ ] **Step 3: Run tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test -race ./cluster/ -run TestMemberStats -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add cluster/member_stats.go cluster/member_stats_test.go
git commit -m "feat(cluster): add MemberStats convenience API over gossip heartbeat data"
```

---

## Chunk 3: Grain Metrics Middleware Wiring

### Task 8: Wire grain metrics into cluster middleware

**Files:**
- Modify: `cluster/config.go` (add metrics tracking to middleware)

Now that `grainMetricsStore` exists (Task 6) and `GrainRegistry` can merge metrics (Task 7), wire the middleware to actually record metrics on each message.

- [ ] **Step 1: Add grain metrics middleware to withClusterReceiveMiddleware**

In `cluster/config.go`, modify `withClusterReceiveMiddleware` (lines 128-143) to also track per-grain metrics on non-system messages:

```go
func withClusterReceiveMiddleware() actor.PropsOption {
	return actor.WithReceiverMiddleware(func(next actor.ReceiverFunc) actor.ReceiverFunc {
		return func(c actor.ReceiverContext, envelope *actor.MessageEnvelope) {
			switch envelope.Message.(type) {
			case *actor.Started:
				handleStarted(c, next, envelope)
			case *actor.Stopped:
				handleStopped(c, next, envelope)
			default:
				handleGrainMetrics(c)
				next(c, envelope)
			}
		}
	})
}

func handleGrainMetrics(c actor.ReceiverContext) {
	cl := GetCluster(c.ActorSystem())
	if cl == nil || cl.grainMetrics == nil {
		return
	}
	identity := GetClusterIdentity(c)
	if identity == nil {
		return
	}
	cl.grainMetrics.Record(identity.AsKey())
}
```

- [ ] **Step 5: Clean up metrics on grain stop**

In `cluster/config.go`, modify `handleStopped` to remove the grain metrics entry. Add after the `GrainDeactivated` publish:

```go
		// Clean up grain metrics entry.
		if cl.grainMetrics != nil {
			cl.grainMetrics.Remove(identity.AsKey())
		}
```

- [ ] **Step 6: Verify compilation and run tests**

Run: `cd /home/cchamplin/development/protoactor-go && go build ./cluster/... && go test -race ./cluster/ -run TestGrainMetrics -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add cluster/config.go
git commit -m "feat(cluster): wire grain metrics middleware into cluster receiver"
```

---

## Chunk 4: Disthash GrainEnumerator Implementation

### Task 9: Add ActivatedAt to GrainMeta and ListGrainsRequest handling

**Files:**
- Modify: `cluster/identitylookup/disthash/placement_actor.go`

- [ ] **Step 1: Enrich GrainMeta with ActivatedAt**

In `cluster/identitylookup/disthash/placement_actor.go`, modify `GrainMeta` (lines 14-18):

```go
// GrainMeta tracks the PID and metadata associated with a cluster identity.
type GrainMeta struct {
	ID          *clustering.ClusterIdentity
	PID         *actor.PID
	ActivatedAt time.Time
}
```

- [ ] **Step 2: Set ActivatedAt in onActivationRequest**

In `onActivationRequest` (line 151), update the GrainMeta creation:

```go
	p.actors[key] = GrainMeta{
		ID:          msg.ClusterIdentity,
		PID:         pid,
		ActivatedAt: time.Now(),
	}
```

- [ ] **Step 3: Add ListGrains message types and handler**

Add new message types at the top of the file (after the `GrainMeta` struct):

```go
// ListGrainsRequest asks the placement actor to return all active grains.
type ListGrainsRequest struct{}

// ListGrainsResponse contains all active grains on this placement actor.
type ListGrainsResponse struct {
	Grains []*clustering.GrainInfo
}
```

Add a new case to `Receive` (in the switch statement, before `default:`):

```go
	case *ListGrainsRequest:
		p.onListGrains(ctx)
```

Add the handler method:

```go
func (p *placementActor) onListGrains(ctx actor.Context) {
	grains := make([]*clustering.GrainInfo, 0, len(p.actors))
	memberID := p.cluster.ActorSystem.ID
	for _, meta := range p.actors {
		grains = append(grains, &clustering.GrainInfo{
			Identity:    meta.ID.Identity,
			Kind:        meta.ID.Kind,
			PID:         meta.PID,
			MemberID:    memberID,
			ActivatedAt: meta.ActivatedAt,
		})
	}
	ctx.Respond(&ListGrainsResponse{Grains: grains})
}
```

- [ ] **Step 4: Verify compilation**

Run: `cd /home/cchamplin/development/protoactor-go && go build ./cluster/identitylookup/disthash/...`
Expected: Success

- [ ] **Step 5: Commit**

```bash
git add cluster/identitylookup/disthash/placement_actor.go
git commit -m "feat(disthash): add ActivatedAt to GrainMeta and ListGrainsRequest handler"
```

---

### Task 10: Implement GrainEnumerator on disthash IdentityLookup

**Files:**
- Modify: `cluster/identitylookup/disthash/identity_lookup.go`
- Modify: `cluster/identitylookup/disthash/manager.go`
- Create: `cluster/identitylookup/disthash/identity_lookup_test.go`

- [ ] **Step 1: Expose placement actor PID from Manager**

In `cluster/identitylookup/disthash/manager.go`, add a method to access the placement actor PID (after line 68):

```go
// PlacementActorPID returns the local placement actor PID.
func (pm *Manager) PlacementActorPID() *actor.PID {
	return pm.placementActor
}
```

- [ ] **Step 2: Implement GrainEnumerator on IdentityLookup**

In `cluster/identitylookup/disthash/identity_lookup.go`, add the `GrainEnumerator` implementation. Add the compile-time check and import `time`:

```go
// Compile-time check that IdentityLookup implements cluster.GrainEnumerator.
var _ cluster.GrainEnumerator = (*IdentityLookup)(nil)

// ListGrains returns all grain activations across the cluster by fanning out
// a ListGrainsRequest to every member's placement actor. For disthash, each
// member's placement actor only tracks its own local grains, so we must
// query all members to get a cluster-wide view.
func (p *IdentityLookup) ListGrains() ([]*cluster.GrainInfo, error) {
	return p.fanOutListGrains("")
}

// ListGrainsByKind returns grain activations filtered by kind (cluster-wide).
func (p *IdentityLookup) ListGrainsByKind(kind string) ([]*cluster.GrainInfo, error) {
	all, err := p.fanOutListGrains("")
	if err != nil {
		return nil, err
	}
	var result []*cluster.GrainInfo
	for _, g := range all {
		if g.Kind == kind {
			result = append(result, g)
		}
	}
	return result, nil
}

// ListGrainsByMember returns grain activations for a specific member.
// Sends ListGrainsRequest only to that member's placement actor.
func (p *IdentityLookup) ListGrainsByMember(memberID string) ([]*cluster.GrainInfo, error) {
	return p.fanOutListGrains(memberID)
}

// fanOutListGrains queries placement actors across the cluster.
// If memberID is empty, queries all members. If memberID is set, queries only that member.
func (p *IdentityLookup) fanOutListGrains(memberID string) ([]*cluster.GrainInfo, error) {
	c := p.partitionManager.cluster
	memberSet := c.MemberList.Members()
	if memberSet == nil {
		return nil, nil
	}

	members := memberSet.Members()
	if len(members) == 0 {
		return nil, nil
	}

	type futureEntry struct {
		future actor.Future
	}

	var futures []futureEntry
	for _, m := range members {
		if memberID != "" && m.Id != memberID {
			continue
		}
		placementPID := p.partitionManager.PidOfActivatorActor(m.Address())
		future := c.ActorSystem.Root.RequestFuture(placementPID, &ListGrainsRequest{}, 5*time.Second)
		futures = append(futures, futureEntry{future: future})
	}

	var result []*cluster.GrainInfo
	for _, fe := range futures {
		res, err := fe.future.Result()
		if err != nil {
			// Log and skip unreachable members — don't fail the whole query.
			c.Logger().Warn("ListGrains: failed to query member placement actor", slog.Any("error", err))
			continue
		}
		typed, ok := res.(*ListGrainsResponse)
		if !ok {
			continue
		}
		result = append(result, typed.Grains...)
	}
	return result, nil
}
```

Add `time`, `log/slog`, and `"github.com/asynkron/protoactor-go/actor"` to the imports (some may already be present).

- [ ] **Step 3: Write tests**

Create `cluster/identitylookup/disthash/identity_lookup_test.go`:

```go
package disthash

import (
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	clustering "github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdentityLookup_ListGrains(t *testing.T) {
	system := actor.NewActorSystem()

	cfg := clustering.Configure("test-cluster",
		newTestProvider(),
		New(),
		remote.Configure("127.0.0.1", 0),
		clustering.WithKinds(clustering.NewKind("test-kind", actor.PropsFromFunc(func(ctx actor.Context) {}))),
	)
	cl := clustering.NewCluster(system, cfg)

	err := cl.StartMember()
	require.NoError(t, err)
	defer cl.Shutdown(true)

	// Activate a grain.
	pid := cl.Get("my-grain", "test-kind")
	require.NotNil(t, pid)

	// Give the activation a moment to register.
	time.Sleep(100 * time.Millisecond)

	// List grains via the GrainEnumerator interface.
	enum, ok := cl.IdentityLookup.(clustering.GrainEnumerator)
	require.True(t, ok, "disthash IdentityLookup should implement GrainEnumerator")

	grains, err := enum.ListGrains()
	require.NoError(t, err)
	require.NotEmpty(t, grains)

	// Find our grain.
	var found *clustering.GrainInfo
	for _, g := range grains {
		if g.Identity == "my-grain" && g.Kind == "test-kind" {
			found = g
			break
		}
	}
	require.NotNil(t, found, "should find the activated grain")
	assert.NotNil(t, found.PID)
	assert.False(t, found.ActivatedAt.IsZero())
	assert.Equal(t, system.ID, found.MemberID)
}

func TestIdentityLookup_ListGrainsByKind_Filters(t *testing.T) {
	system := actor.NewActorSystem()

	cfg := clustering.Configure("test-cluster",
		newTestProvider(),
		New(),
		remote.Configure("127.0.0.1", 0),
		clustering.WithKinds(
			clustering.NewKind("kind-a", actor.PropsFromFunc(func(ctx actor.Context) {})),
			clustering.NewKind("kind-b", actor.PropsFromFunc(func(ctx actor.Context) {})),
		),
	)
	cl := clustering.NewCluster(system, cfg)

	err := cl.StartMember()
	require.NoError(t, err)
	defer cl.Shutdown(true)

	cl.Get("grain-1", "kind-a")
	cl.Get("grain-2", "kind-b")
	time.Sleep(100 * time.Millisecond)

	enum := cl.IdentityLookup.(clustering.GrainEnumerator)
	grains, err := enum.ListGrainsByKind("kind-a")
	require.NoError(t, err)

	for _, g := range grains {
		assert.Equal(t, "kind-a", g.Kind)
	}
}

// newTestProvider creates a minimal cluster provider for testing.
// Check existing test helpers in the cluster package — this may already exist.
// If automanaged or inmemory provider exists, use that instead.
func newTestProvider() clustering.ClusterProvider {
	// Use automanaged provider for single-node test.
	// Import the appropriate provider package.
	return nil // PLACEHOLDER — replace with actual provider
}
```

**Important:** You need to find the actual test provider used in the cluster tests. Look at `cluster/cluster_test_tool.go` or similar files for `newInmemoryProvider()` or automanaged provider creation. Replace the `newTestProvider()` placeholder with the real implementation.

- [ ] **Step 4: Run tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test -race ./cluster/identitylookup/disthash/ -run TestIdentityLookup -v -timeout 30s`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cluster/identitylookup/disthash/identity_lookup.go cluster/identitylookup/disthash/manager.go cluster/identitylookup/disthash/identity_lookup_test.go
git commit -m "feat(disthash): implement GrainEnumerator on IdentityLookup"
```

---

## Chunk 5: Storage Backend Enumerators

### Task 11: Implement StorageGrainEnumerator on InMemory storage

**Files:**
- Modify: `cluster/identitylookup/inmemory_storage.go`
- Modify: `cluster/identitylookup/conformance.go` (add enumerator conformance tests)

- [ ] **Step 1: Add ListActivations and ListActivationsByMember to InMemoryStorageLookup**

In `cluster/identitylookup/inmemory_storage.go`, add (after `RemoveMemberId`):

```go
// Compile-time check that InMemoryStorageLookup implements cluster.StorageGrainEnumerator.
var _ cluster.StorageGrainEnumerator = (*InMemoryStorageLookup)(nil)

// ListActivations returns all stored activations.
func (s *InMemoryStorageLookup) ListActivations() ([]*cluster.StoredActivationInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]*cluster.StoredActivationInfo, 0, len(s.activations))
	for key, act := range s.activations {
		kind, identity := parseIdentityKey(key)
		result = append(result, &cluster.StoredActivationInfo{
			Identity: identity,
			Kind:     kind,
			Pid:      act.Pid,
			MemberID: act.MemberID,
		})
	}
	return result, nil
}

// ListActivationsByMember returns activations belonging to a specific member.
func (s *InMemoryStorageLookup) ListActivationsByMember(memberID string) ([]*cluster.StoredActivationInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []*cluster.StoredActivationInfo
	for key, act := range s.activations {
		if act.MemberID == memberID {
			kind, identity := parseIdentityKey(key)
			result = append(result, &cluster.StoredActivationInfo{
				Identity: identity,
				Kind:     kind,
				Pid:      act.Pid,
				MemberID: act.MemberID,
			})
		}
	}
	return result, nil
}

// parseIdentityKey parses "kind/identity" format. Returns kind, identity.
func parseIdentityKey(key string) (string, string) {
	for i := 0; i < len(key); i++ {
		if key[i] == '/' {
			return key[:i], key[i+1:]
		}
	}
	return key, ""
}
```

- [ ] **Step 2: Add enumerator conformance tests**

In `cluster/identitylookup/conformance.go`, add `"github.com/stretchr/testify/assert"` to the imports (existing imports already have `require`). Then add a new test suite for `StorageGrainEnumerator` after the `StorageConformanceSuite` (after line 231):

```go
// EnumeratorConformanceSuite validates that a StorageGrainEnumerator implementation
// correctly lists activations. Backend implementations should run this suite:
//
//	func TestEnumeratorConformance(t *testing.T) {
//	    suite := &identitylookup.EnumeratorConformanceSuite{
//	        NewStorage: func() identitylookup.EnumerableStorage { return myimpl.New() },
//	        Cleanup:    func() { /* optional teardown */ },
//	    }
//	    suite.RunAll(t)
//	}
type EnumerableStorage interface {
	cluster.StorageLookup
	cluster.StorageGrainEnumerator
}

type EnumeratorConformanceSuite struct {
	NewStorage func() EnumerableStorage
	Cleanup    func()
}

func (s *EnumeratorConformanceSuite) RunAll(t *testing.T) {
	t.Helper()

	tests := []struct {
		name string
		fn   func(t *testing.T, storage EnumerableStorage)
	}{
		{"ListActivations_ReturnsStoredActivations", s.testListActivations},
		{"ListActivationsByMember_Filters", s.testListActivationsByMember},
		{"ListActivations_ExcludesLockedEntries", s.testExcludesLocked},
		{"ListActivations_ReflectsRemovals", s.testReflectsRemovals},
		{"ListActivationsByMember_AfterRemoveMember", s.testAfterRemoveMember},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			storage := s.NewStorage()
			defer func() {
				if s.Cleanup != nil {
					s.Cleanup()
				}
			}()
			tc.fn(t, storage)
		})
	}
}

func (s *EnumeratorConformanceSuite) testListActivations(t *testing.T, storage EnumerableStorage) {
	ci1 := &cluster.ClusterIdentity{Identity: "actor-1", Kind: "kind-a"}
	ci2 := &cluster.ClusterIdentity{Identity: "actor-2", Kind: "kind-b"}

	lock1 := storage.TryAcquireLock(ci1)
	require.NotNil(t, lock1)
	storage.StoreActivation("member-1", lock1, actor.NewPID("127.0.0.1:8080", "actor-1"))

	lock2 := storage.TryAcquireLock(ci2)
	require.NotNil(t, lock2)
	storage.StoreActivation("member-1", lock2, actor.NewPID("127.0.0.1:8080", "actor-2"))

	activations, err := storage.ListActivations()
	require.NoError(t, err)
	require.Len(t, activations, 2)

	// Verify both activations are present.
	identities := map[string]bool{}
	for _, a := range activations {
		identities[a.Kind+"/"+a.Identity] = true
		require.NotEmpty(t, a.Pid)
		require.Equal(t, "member-1", a.MemberID)
	}
	assert.True(t, identities["kind-a/actor-1"])
	assert.True(t, identities["kind-b/actor-2"])
}

func (s *EnumeratorConformanceSuite) testListActivationsByMember(t *testing.T, storage EnumerableStorage) {
	ci1 := &cluster.ClusterIdentity{Identity: "actor-1", Kind: "kind-a"}
	ci2 := &cluster.ClusterIdentity{Identity: "actor-2", Kind: "kind-a"}

	lock1 := storage.TryAcquireLock(ci1)
	require.NotNil(t, lock1)
	storage.StoreActivation("member-1", lock1, actor.NewPID("127.0.0.1:8080", "actor-1"))

	lock2 := storage.TryAcquireLock(ci2)
	require.NotNil(t, lock2)
	storage.StoreActivation("member-2", lock2, actor.NewPID("127.0.0.1:8081", "actor-2"))

	result, err := storage.ListActivationsByMember("member-1")
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "actor-1", result[0].Identity)
	assert.Equal(t, "member-1", result[0].MemberID)
}

func (s *EnumeratorConformanceSuite) testExcludesLocked(t *testing.T, storage EnumerableStorage) {
	ci := &cluster.ClusterIdentity{Identity: "locked-actor", Kind: "kind-a"}

	// Acquire a lock but don't store an activation.
	lock := storage.TryAcquireLock(ci)
	require.NotNil(t, lock)

	activations, err := storage.ListActivations()
	require.NoError(t, err)

	// The locked-but-not-activated entry should NOT appear.
	for _, a := range activations {
		assert.NotEqual(t, "locked-actor", a.Identity, "locked entries should be excluded")
	}
}

func (s *EnumeratorConformanceSuite) testReflectsRemovals(t *testing.T, storage EnumerableStorage) {
	ci := &cluster.ClusterIdentity{Identity: "removable", Kind: "kind-a"}

	lock := storage.TryAcquireLock(ci)
	require.NotNil(t, lock)
	storage.StoreActivation("member-1", lock, actor.NewPID("127.0.0.1:8080", "removable"))

	// Verify it's listed.
	before, err := storage.ListActivations()
	require.NoError(t, err)
	found := false
	for _, a := range before {
		if a.Identity == "removable" {
			found = true
		}
	}
	require.True(t, found)

	// Remove it.
	storage.RemoveActivation(&cluster.SpawnLock{
		LockID:          lock.LockID,
		ClusterIdentity: ci,
	})

	// Verify it's gone.
	after, err := storage.ListActivations()
	require.NoError(t, err)
	for _, a := range after {
		assert.NotEqual(t, "removable", a.Identity)
	}
}

func (s *EnumeratorConformanceSuite) testAfterRemoveMember(t *testing.T, storage EnumerableStorage) {
	ci := &cluster.ClusterIdentity{Identity: "member-actor", Kind: "kind-a"}

	lock := storage.TryAcquireLock(ci)
	require.NotNil(t, lock)
	storage.StoreActivation("member-cleanup", lock, actor.NewPID("127.0.0.1:8080", "member-actor"))

	storage.RemoveMemberId("member-cleanup")

	result, err := storage.ListActivationsByMember("member-cleanup")
	require.NoError(t, err)
	assert.Empty(t, result)
}
```

- [ ] **Step 3: Run InMemory enumerator conformance tests**

Add to `cluster/identitylookup/conformance_test.go` (or the existing test file):

```go
func TestInMemoryEnumeratorConformance(t *testing.T) {
	suite := &EnumeratorConformanceSuite{
		NewStorage: func() EnumerableStorage {
			return NewInMemoryStorageLookup()
		},
	}
	suite.RunAll(t)
}
```

Run: `cd /home/cchamplin/development/protoactor-go && go test -race ./cluster/identitylookup/ -run TestInMemoryEnumerator -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add cluster/identitylookup/inmemory_storage.go cluster/identitylookup/conformance.go cluster/identitylookup/conformance_test.go
git commit -m "feat(cluster): add StorageGrainEnumerator to InMemory storage and conformance suite"
```

---

### Task 12: Implement StorageGrainEnumerator on Redis

**Files:**
- Modify: `cluster/identitylookup/redis/redis_identity.go`
- Modify: `cluster/identitylookup/redis/redis_identity_test.go`

- [ ] **Step 1: Add ListActivations and ListActivationsByMember to RedisIdentityStorage**

In `cluster/identitylookup/redis/redis_identity.go`, add after `RemoveMemberId`:

```go
// Compile-time check that RedisIdentityStorage implements cluster.StorageGrainEnumerator.
var _ cluster.StorageGrainEnumerator = (*RedisIdentityStorage)(nil)

// ListActivations returns all stored activations by scanning identity keys.
func (s *RedisIdentityStorage) ListActivations() ([]*cluster.StoredActivationInfo, error) {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	var result []*cluster.StoredActivationInfo

	// Scan for all identity keys.
	pattern := s.ciPrefix + "*"
	iter := s.client.Scan(ctx, 0, pattern, 100).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("redis ListActivations scan: %w", err)
	}

	if len(keys) == 0 {
		return result, nil
	}

	// Pipeline HGETALL for all keys.
	pipe := s.client.Pipeline()
	cmds := make([]*goredis.MapStringStringCmd, len(keys))
	for i, key := range keys {
		cmds[i] = pipe.HGetAll(ctx, key)
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("redis ListActivations pipeline: %w", err)
	}

	for i, cmd := range cmds {
		fields, err := cmd.Result()
		if err != nil || len(fields) == 0 {
			continue
		}
		pidID := fields["pid"]
		pidAddr := fields["adr"]
		memberID := fields["mid"]
		if pidID == "" || pidAddr == "" || memberID == "" {
			continue // Lock-only entry, skip.
		}

		// Parse kind and identity from key: "{cluster}:ci:{kind}/{identity}"
		identityPart := keys[i][len(s.ciPrefix):]
		kind, identity := cluster.ParseStoredActivationInfoKey(identityPart)

		result = append(result, &cluster.StoredActivationInfo{
			Identity: identity,
			Kind:     kind,
			Pid:      fmt.Sprintf("%s/%s", pidAddr, pidID),
			MemberID: memberID,
		})
	}

	return result, nil
}

// ListActivationsByMember returns activations belonging to a specific member
// by reading the member's tracking set.
func (s *RedisIdentityStorage) ListActivationsByMember(memberID string) ([]*cluster.StoredActivationInfo, error) {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	mbKey := s.memberKey(memberID)

	// Get all identity keys for this member.
	keys, err := s.client.SMembers(ctx, mbKey).Result()
	if err != nil {
		return nil, fmt.Errorf("redis ListActivationsByMember smembers: %w", err)
	}

	if len(keys) == 0 {
		return nil, nil
	}

	// Pipeline HGETALL for all keys.
	pipe := s.client.Pipeline()
	cmds := make([]*goredis.MapStringStringCmd, len(keys))
	for i, key := range keys {
		cmds[i] = pipe.HGetAll(ctx, key)
	}
	_, err = pipe.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("redis ListActivationsByMember pipeline: %w", err)
	}

	var result []*cluster.StoredActivationInfo
	for i, cmd := range cmds {
		fields, err := cmd.Result()
		if err != nil || len(fields) == 0 {
			continue
		}
		pidID := fields["pid"]
		pidAddr := fields["adr"]
		mid := fields["mid"]
		if pidID == "" || pidAddr == "" || mid == "" {
			continue
		}

		// Parse kind and identity from key: "{cluster}:ci:{kind}/{identity}"
		identityPart := keys[i][len(s.ciPrefix):]
		kind, identity := cluster.ParseStoredActivationInfoKey(identityPart)

		result = append(result, &cluster.StoredActivationInfo{
			Identity: identity,
			Kind:     kind,
			Pid:      fmt.Sprintf("%s/%s", pidAddr, pidID),
			MemberID: mid,
		})
	}

	return result, nil
}
```

- [ ] **Step 2: Add enumerator tests to redis_identity_test.go**

Add the enumerator conformance test to the existing Redis test file. This will require a running Redis instance (via testcontainers or a local Redis). Follow the pattern already used in the existing `redis_identity_test.go` for setup/teardown:

```go
func TestRedisEnumeratorConformance(t *testing.T) {
	// Follow the existing test setup pattern in this file for Redis connection.
	// Create the suite and run it.
	suite := &identitylookup.EnumeratorConformanceSuite{
		NewStorage: func() identitylookup.EnumerableStorage {
			// Create a fresh Redis storage using the test client.
			return New("test-enum-"+uuid.New().String(), redisClient)
		},
		Cleanup: func() {
			// Optional: flush test keys.
		},
	}
	suite.RunAll(t)
}
```

- [ ] **Step 3: Verify compilation and tests**

Run: `cd /home/cchamplin/development/protoactor-go && go build ./cluster/identitylookup/redis/...`
Expected: Success

Run Redis tests if a Redis instance is available, otherwise verify compilation only.

- [ ] **Step 4: Commit**

```bash
git add cluster/identitylookup/redis/redis_identity.go cluster/identitylookup/redis/redis_identity_test.go
git commit -m "feat(redis): implement StorageGrainEnumerator for Redis identity storage"
```

---

### Task 13: Implement StorageGrainEnumerator on Postgres

**Files:**
- Modify: `cluster/identitylookup/postgres/postgres_identity.go`
- Modify: `cluster/identitylookup/postgres/postgres_identity_test.go`

- [ ] **Step 1: Add ListActivations and ListActivationsByMember to PostgresIdentityStorage**

In `cluster/identitylookup/postgres/postgres_identity.go`, add after `RemoveMemberId`:

```go
// Compile-time check that PostgresIdentityStorage implements cluster.StorageGrainEnumerator.
var _ cluster.StorageGrainEnumerator = (*PostgresIdentityStorage)(nil)

// ListActivations returns all stored activations.
func (s *PostgresIdentityStorage) ListActivations() ([]*cluster.StoredActivationInfo, error) {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	query := fmt.Sprintf(
		`SELECT key, pid_id, pid_address, member_id FROM %s WHERE pid_id != ''`,
		s.tableName,
	)

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("postgres ListActivations: %w", err)
	}
	defer rows.Close()

	var result []*cluster.StoredActivationInfo
	for rows.Next() {
		var key, pidID, pidAddr, memberID string
		if err := rows.Scan(&key, &pidID, &pidAddr, &memberID); err != nil {
			return nil, fmt.Errorf("postgres ListActivations scan: %w", err)
		}

		kind, identity := cluster.ParseStoredActivationInfoKey(key)
		result = append(result, &cluster.StoredActivationInfo{
			Identity: identity,
			Kind:     kind,
			Pid:      fmt.Sprintf("%s/%s", pidAddr, pidID),
			MemberID: memberID,
		})
	}

	return result, rows.Err()
}

// ListActivationsByMember returns activations belonging to a specific member.
func (s *PostgresIdentityStorage) ListActivationsByMember(memberID string) ([]*cluster.StoredActivationInfo, error) {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	query := fmt.Sprintf(
		`SELECT key, pid_id, pid_address, member_id FROM %s WHERE pid_id != '' AND member_id = $1`,
		s.tableName,
	)

	rows, err := s.db.QueryContext(ctx, query, memberID)
	if err != nil {
		return nil, fmt.Errorf("postgres ListActivationsByMember: %w", err)
	}
	defer rows.Close()

	var result []*cluster.StoredActivationInfo
	for rows.Next() {
		var key, pidID, pidAddr, mid string
		if err := rows.Scan(&key, &pidID, &pidAddr, &mid); err != nil {
			return nil, fmt.Errorf("postgres ListActivationsByMember scan: %w", err)
		}

		kind, identity := cluster.ParseStoredActivationInfoKey(key)
		result = append(result, &cluster.StoredActivationInfo{
			Identity: identity,
			Kind:     kind,
			Pid:      fmt.Sprintf("%s/%s", pidAddr, pidID),
			MemberID: mid,
		})
	}

	return result, rows.Err()
}
```

- [ ] **Step 2: Add enumerator tests (follow existing Postgres test pattern)**

- [ ] **Step 3: Verify compilation**

Run: `cd /home/cchamplin/development/protoactor-go && go build ./cluster/identitylookup/postgres/...`
Expected: Success

- [ ] **Step 4: Commit**

```bash
git add cluster/identitylookup/postgres/postgres_identity.go cluster/identitylookup/postgres/postgres_identity_test.go
git commit -m "feat(postgres): implement StorageGrainEnumerator for Postgres identity storage"
```

---

### Task 14: Implement StorageGrainEnumerator on NATS KV (identitylookup/nats)

**Files:**
- Modify: `cluster/identitylookup/nats/nats_identity.go`
- Modify: `cluster/identitylookup/nats/nats_identity_test.go`

- [ ] **Step 1: Add ListActivations and ListActivationsByMember to NatsIdentityStorage**

In `cluster/identitylookup/nats/nats_identity.go`, add after `removeKeyFromMember`:

```go
// Compile-time check that NatsIdentityStorage implements cluster.StorageGrainEnumerator.
var _ cluster.StorageGrainEnumerator = (*NatsIdentityStorage)(nil)

// ListActivations returns all stored activations by iterating all member
// tracking records and looking up each identity key.
func (s *NatsIdentityStorage) ListActivations() ([]*cluster.StoredActivationInfo, error) {
	s.acquire()
	defer s.release()

	ctx := context.Background()

	// Get all member IDs from the members bucket.
	keys, err := s.members.Keys(ctx)
	if err != nil {
		if errors.Is(err, jetstream.ErrNoKeysFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("nats ListActivations keys: %w", err)
	}

	var result []*cluster.StoredActivationInfo
	for _, memberID := range keys {
		memberActivations, err := s.listMemberActivations(ctx, memberID)
		if err != nil {
			continue // Skip members with errors.
		}
		result = append(result, memberActivations...)
	}

	return result, nil
}

// ListActivationsByMember returns activations belonging to a specific member.
func (s *NatsIdentityStorage) ListActivationsByMember(memberID string) ([]*cluster.StoredActivationInfo, error) {
	s.acquire()
	defer s.release()

	ctx := context.Background()
	return s.listMemberActivations(ctx, memberID)
}

// listMemberActivations reads a member's tracking record and looks up each identity.
func (s *NatsIdentityStorage) listMemberActivations(ctx context.Context, memberID string) ([]*cluster.StoredActivationInfo, error) {
	entry, err := s.members.Get(ctx, memberID)
	if err != nil {
		return nil, err
	}

	var mrec memberRecord
	if err := json.Unmarshal(entry.Value(), &mrec); err != nil {
		return nil, err
	}

	var result []*cluster.StoredActivationInfo
	for _, key := range mrec.Keys {
		idEntry, err := s.identities.Get(ctx, key)
		if err != nil {
			continue
		}

		var rec activationRecord
		if err := json.Unmarshal(idEntry.Value(), &rec); err != nil {
			continue
		}

		// Skip lock-only entries.
		if rec.PidID == "" || rec.PidAddress == "" {
			continue
		}

		// Parse kind.identity from the KV key (dot-separated).
		kind, identity := cluster.ParseDotSeparatedKey(key)

		result = append(result, &cluster.StoredActivationInfo{
			Identity: identity,
			Kind:     kind,
			Pid:      fmt.Sprintf("%s/%s", rec.PidAddress, rec.PidID),
			MemberID: rec.MemberID,
		})
	}

	return result, nil
}
```

- [ ] **Step 2: Add enumerator tests (follow existing NATS test pattern using testcontainers)**

- [ ] **Step 3: Verify compilation**

Run: `cd /home/cchamplin/development/protoactor-go && go build ./cluster/identitylookup/nats/...`
Expected: Success

- [ ] **Step 4: Commit**

```bash
git add cluster/identitylookup/nats/nats_identity.go cluster/identitylookup/nats/nats_identity_test.go
git commit -m "feat(nats): implement StorageGrainEnumerator for NATS KV identity storage"
```

---

## Chunk 6: Direct IdentityLookup Enumerators and Storage Adapter

### Task 15: Implement GrainEnumerator on IdentityStorageLookup adapter

**Files:**
- Modify: `cluster/identitylookup/storage/identity_storage_lookup.go`

- [ ] **Step 1: Add GrainEnumerator delegation to IdentityStorageLookup**

In `cluster/identitylookup/storage/identity_storage_lookup.go`, add after `pidFromStored`:

```go
// ListGrains returns all grain activations if the underlying storage
// implements StorageGrainEnumerator. Returns ErrEnumerationNotSupported otherwise.
func (l *IdentityStorageLookup) ListGrains() ([]*cluster.GrainInfo, error) {
	enum, ok := l.storage.(cluster.StorageGrainEnumerator)
	if !ok {
		return nil, cluster.ErrEnumerationNotSupported
	}
	infos, err := enum.ListActivations()
	if err != nil {
		return nil, err
	}
	return convertStoredToGrainInfos(infos), nil
}

// ListGrainsByKind returns grain activations filtered by kind.
func (l *IdentityStorageLookup) ListGrainsByKind(kind string) ([]*cluster.GrainInfo, error) {
	all, err := l.ListGrains()
	if err != nil {
		return nil, err
	}
	var result []*cluster.GrainInfo
	for _, g := range all {
		if g.Kind == kind {
			result = append(result, g)
		}
	}
	return result, nil
}

// ListGrainsByMember returns grain activations for a specific member.
func (l *IdentityStorageLookup) ListGrainsByMember(memberID string) ([]*cluster.GrainInfo, error) {
	enum, ok := l.storage.(cluster.StorageGrainEnumerator)
	if !ok {
		return nil, cluster.ErrEnumerationNotSupported
	}
	infos, err := enum.ListActivationsByMember(memberID)
	if err != nil {
		return nil, err
	}
	return convertStoredToGrainInfos(infos), nil
}

func convertStoredToGrainInfos(infos []*cluster.StoredActivationInfo) []*cluster.GrainInfo {
	result := make([]*cluster.GrainInfo, len(infos))
	for i, info := range infos {
		result[i] = cluster.StoredActivationInfoToGrainInfo(info)
	}
	return result
}
```

Add a conditional compile-time check. Since `IdentityStorageLookup` only implements `GrainEnumerator` when the storage backend supports it, we can add the interface assertion:

```go
// Compile-time check that IdentityStorageLookup implements cluster.GrainEnumerator.
// The actual enumeration depends on the StorageLookup backend also implementing
// cluster.StorageGrainEnumerator.
var _ cluster.GrainEnumerator = (*IdentityStorageLookup)(nil)
```

- [ ] **Step 2: Verify compilation**

Run: `cd /home/cchamplin/development/protoactor-go && go build ./cluster/identitylookup/storage/...`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add cluster/identitylookup/storage/identity_storage_lookup.go
git commit -m "feat(storage): implement GrainEnumerator delegation in IdentityStorageLookup adapter"
```

---

### Task 16: Implement GrainEnumerator on natskv provider

**Files:**
- Modify: `cluster/clusterproviders/natskv/natskv_identity.go`

- [ ] **Step 1: Add GrainEnumerator methods to natskv IdentityLookup**

In `cluster/clusterproviders/natskv/natskv_identity.go`, add after `pidFromRecord`:

```go
// Compile-time check that IdentityLookup implements cluster.GrainEnumerator.
var _ cluster.GrainEnumerator = (*IdentityLookup)(nil)

// ListGrains returns all known grain activations from the NATS KV store.
func (il *IdentityLookup) ListGrains() ([]*cluster.GrainInfo, error) {
	if il.setupErr != nil {
		return nil, il.setupErr
	}

	ctx := context.Background()
	keys, err := il.memberTracker.Keys(ctx)
	if err != nil {
		if errors.Is(err, jetstream.ErrNoKeysFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("natskv ListGrains keys: %w", err)
	}

	var result []*cluster.GrainInfo
	for _, memberID := range keys {
		memberGrains, err := il.listMemberGrains(ctx, memberID)
		if err != nil {
			continue
		}
		result = append(result, memberGrains...)
	}

	return result, nil
}

// ListGrainsByKind returns grain activations filtered by kind.
func (il *IdentityLookup) ListGrainsByKind(kind string) ([]*cluster.GrainInfo, error) {
	all, err := il.ListGrains()
	if err != nil {
		return nil, err
	}
	var result []*cluster.GrainInfo
	for _, g := range all {
		if g.Kind == kind {
			result = append(result, g)
		}
	}
	return result, nil
}

// ListGrainsByMember returns grain activations for a specific member.
func (il *IdentityLookup) ListGrainsByMember(memberID string) ([]*cluster.GrainInfo, error) {
	if il.setupErr != nil {
		return nil, il.setupErr
	}
	ctx := context.Background()
	return il.listMemberGrains(ctx, memberID)
}

func (il *IdentityLookup) listMemberGrains(ctx context.Context, memberID string) ([]*cluster.GrainInfo, error) {
	entry, err := il.memberTracker.Get(ctx, memberID)
	if err != nil {
		return nil, err
	}

	var mrec memberRecord
	if err := json.Unmarshal(entry.Value(), &mrec); err != nil {
		return nil, err
	}

	var result []*cluster.GrainInfo
	for _, key := range mrec.Keys {
		idEntry, err := il.identities.Get(ctx, key)
		if err != nil {
			continue
		}

		var rec activationRecord
		if err := json.Unmarshal(idEntry.Value(), &rec); err != nil {
			continue
		}

		if rec.PidID == "" || rec.PidAddress == "" {
			continue
		}

		kind, identity := cluster.ParseDotSeparatedKey(key)
		result = append(result, &cluster.GrainInfo{
			Identity: identity,
			Kind:     kind,
			PID:      pidFromRecord(&rec),
			MemberID: rec.MemberID,
		})
	}

	return result, nil
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd /home/cchamplin/development/protoactor-go && go build ./cluster/clusterproviders/natskv/...`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add cluster/clusterproviders/natskv/natskv_identity.go
git commit -m "feat(natskv): implement GrainEnumerator for natskv identity lookup"
```

---

### Task 17: Implement GrainEnumerator on natsstream provider

**Files:**
- Modify: `cluster/clusterproviders/natsstream/natsstream_identity.go`

- [ ] **Step 1: Add GrainEnumerator methods to natsstream IdentityLookup**

In `cluster/clusterproviders/natsstream/natsstream_identity.go`, add after `pidFromRecord`:

```go
// Compile-time check that IdentityLookup implements cluster.GrainEnumerator.
var _ cluster.GrainEnumerator = (*IdentityLookup)(nil)

// ListGrains returns all known grain activations from the in-memory member tracking.
// Note: This reflects only this node's knowledge of activations.
func (il *IdentityLookup) ListGrains() ([]*cluster.GrainInfo, error) {
	il.memberKeysMu.Lock()
	// Copy the map to avoid holding the lock during I/O.
	allKeys := make(map[string][]string, len(il.memberKeys))
	for memberID, keys := range il.memberKeys {
		keysCopy := make([]string, len(keys))
		copy(keysCopy, keys)
		allKeys[memberID] = keysCopy
	}
	il.memberKeysMu.Unlock()

	ctx := context.Background()
	var result []*cluster.GrainInfo
	for memberID, subjects := range allKeys {
		for _, subject := range subjects {
			g := il.lookupSubject(ctx, subject, memberID)
			if g != nil {
				result = append(result, g)
			}
		}
	}

	return result, nil
}

// ListGrainsByKind returns grain activations filtered by kind.
func (il *IdentityLookup) ListGrainsByKind(kind string) ([]*cluster.GrainInfo, error) {
	all, err := il.ListGrains()
	if err != nil {
		return nil, err
	}
	var result []*cluster.GrainInfo
	for _, g := range all {
		if g.Kind == kind {
			result = append(result, g)
		}
	}
	return result, nil
}

// ListGrainsByMember returns grain activations for a specific member.
func (il *IdentityLookup) ListGrainsByMember(memberID string) ([]*cluster.GrainInfo, error) {
	il.memberKeysMu.Lock()
	keys := make([]string, len(il.memberKeys[memberID]))
	copy(keys, il.memberKeys[memberID])
	il.memberKeysMu.Unlock()

	ctx := context.Background()
	var result []*cluster.GrainInfo
	for _, subject := range keys {
		g := il.lookupSubject(ctx, subject, memberID)
		if g != nil {
			result = append(result, g)
		}
	}

	return result, nil
}

// lookupSubject reads an activation record from the stream for the given subject.
func (il *IdentityLookup) lookupSubject(ctx context.Context, subject string, memberID string) *cluster.GrainInfo {
	if il.identityStream == nil {
		return nil
	}

	msg, err := il.identityStream.GetLastMsgForSubject(ctx, subject)
	if err != nil {
		return nil
	}

	var rec activationRecord
	if err := json.Unmarshal(msg.Data, &rec); err != nil {
		return nil
	}

	if rec.PidID == "" || rec.PidAddress == "" {
		return nil
	}

	// Strip the subject prefix to get "kind.identity".
	kvPart := subject
	if idx := len(il.identitySubjectPrefix) + 1; idx < len(subject) {
		kvPart = subject[idx:]
	}
	kind, identity := cluster.ParseDotSeparatedKey(kvPart)

	return &cluster.GrainInfo{
		Identity: identity,
		Kind:     kind,
		PID:      pidFromRecord(&rec),
		MemberID: rec.MemberID,
	}
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd /home/cchamplin/development/protoactor-go && go build ./cluster/clusterproviders/natsstream/...`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add cluster/clusterproviders/natsstream/natsstream_identity.go
git commit -m "feat(natsstream): implement GrainEnumerator for natsstream identity lookup"
```

---

## Chunk 7: Integration Tests and Final Verification

### Task 18: Full compilation and test pass

- [ ] **Step 1: Verify full project compiles**

Run: `cd /home/cchamplin/development/protoactor-go && go build ./...`
Expected: Success with no errors

- [ ] **Step 2: Run all cluster tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test -race ./cluster/... -v -timeout 120s`
Expected: All tests PASS

- [ ] **Step 3: Run identitylookup tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test -race ./cluster/identitylookup/... -v -timeout 120s`
Expected: All tests PASS

- [ ] **Step 4: Run disthash tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test -race ./cluster/identitylookup/disthash/... -v -timeout 120s`
Expected: All tests PASS

- [ ] **Step 5: Run plugin tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test -race ./plugin/... -v -timeout 120s`
Expected: All tests PASS

- [ ] **Step 6: Run provider tests (if infrastructure available)**

Run: `cd /home/cchamplin/development/protoactor-go && go test -race ./cluster/clusterproviders/... -v -timeout 120s`
Expected: PASS (may skip tests that require external infrastructure)

- [ ] **Step 7: Commit any remaining fixes**

```bash
git add -A
git commit -m "fix: resolve compilation and test issues from grain introspection implementation"
```

---

### Task 19: End-to-end integration test for GrainRegistry

**Files:**
- Create: `cluster/grain_registry_integration_test.go`

- [ ] **Step 1: Write integration test**

Create `cluster/grain_registry_integration_test.go`:

```go
package cluster

import (
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGrainRegistry_Integration_DisthashEnumeration(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("test-registry",
		newInmemoryProvider(),
		disthash.New(),
		remote.Configure("127.0.0.1", 0),
		WithKinds(
			NewKind("kind-a", actor.PropsFromFunc(func(ctx actor.Context) {})),
			NewKind("kind-b", actor.PropsFromFunc(func(ctx actor.Context) {})),
		),
	)
	cl := NewCluster(system, config)
	err := cl.StartMember()
	require.NoError(t, err)
	defer cl.Shutdown(true)

	// Activate grains.
	pid1 := cl.Get("grain-1", "kind-a")
	pid2 := cl.Get("grain-2", "kind-a")
	pid3 := cl.Get("grain-3", "kind-b")
	require.NotNil(t, pid1)
	require.NotNil(t, pid2)
	require.NotNil(t, pid3)

	time.Sleep(200 * time.Millisecond)

	reg := cl.GrainRegistry()

	// Count.
	assert.Equal(t, 3, reg.Count())

	// CountByKind.
	byKind := reg.CountByKind()
	assert.Equal(t, 2, byKind["kind-a"])
	assert.Equal(t, 1, byKind["kind-b"])

	// All.
	all, err := reg.All()
	require.NoError(t, err)
	assert.Len(t, all, 3)

	// ByKind.
	kindA, err := reg.ByKind("kind-a")
	require.NoError(t, err)
	assert.Len(t, kindA, 2)
	for _, g := range kindA {
		assert.Equal(t, "kind-a", g.Kind)
	}

	// Get.
	info, found, err := reg.Get("grain-3", "kind-b")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "grain-3", info.Identity)
	assert.Equal(t, "kind-b", info.Kind)

	// Get non-existent.
	_, found, err = reg.Get("nonexistent", "kind-a")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestGrainRegistry_Integration_WithGrainMetrics(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("test-metrics",
		newInmemoryProvider(),
		disthash.New(),
		remote.Configure("127.0.0.1", 0),
		WithKinds(NewKind("echo", actor.PropsFromFunc(func(ctx actor.Context) {
			switch ctx.Message().(type) {
			case *actor.Started:
			case *actor.Stopping:
			case *actor.Stopped:
			default:
				ctx.Respond(ctx.Message())
			}
		}))),
		WithGrainMetrics(),
	)
	cl := NewCluster(system, config)
	err := cl.StartMember()
	require.NoError(t, err)
	defer cl.Shutdown(true)

	// Activate and send messages.
	_, err = cl.Request("my-echo", "echo", "hello")
	require.NoError(t, err)
	_, err = cl.Request("my-echo", "echo", "world")
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	reg := cl.GrainRegistry()
	info, found, err := reg.Get("my-echo", "echo")
	require.NoError(t, err)
	require.True(t, found)

	// Should have metrics since WithGrainMetrics() is enabled.
	assert.Greater(t, info.MessageCount, int64(0))
	assert.False(t, info.LastMessageAt.IsZero())
}
```

Note: Adapt `newInmemoryProvider()` to whatever the cluster test helpers provide. Check existing test files in the `cluster/` package for the right function name.

- [ ] **Step 2: Run the integration test**

Run: `cd /home/cchamplin/development/protoactor-go && go test -race ./cluster/ -run TestGrainRegistry_Integration -v -timeout 60s`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add cluster/grain_registry_integration_test.go
git commit -m "test(cluster): add end-to-end integration tests for GrainRegistry"
```

---

### Task 20: Final review and cleanup

- [ ] **Step 1: Run full test suite**

Run: `cd /home/cchamplin/development/protoactor-go && go test -race ./... -timeout 300s 2>&1 | tail -50`
Expected: All tests PASS (some may skip due to missing infrastructure like Redis, Postgres, NATS)

- [ ] **Step 2: Run go vet**

Run: `cd /home/cchamplin/development/protoactor-go && go vet ./...`
Expected: No issues

- [ ] **Step 3: Verify no unused imports**

Run: `cd /home/cchamplin/development/protoactor-go && go build ./...`
Expected: Clean build

- [ ] **Step 4: Final commit if any fixes**

```bash
git add -A
git commit -m "chore: final cleanup for grain introspection feature"
```
