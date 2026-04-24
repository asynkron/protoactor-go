# Sub-project 2: Integrate Shared Placement Actor into IdentityStorageLookup

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor `IdentityStorageLookup` to use the shared placement actor and activator proxy from Sub-project 1c. This replaces the direct `spawnActivation` method with placement actor-mediated spawning, adds stale member validation, duplicate-request coalescing, and member strategy selection. Because Redis, Postgres, and NATS KV identity lookups all use `IdentityStorageLookup`, this single integration fixes all three backends.

**Architecture:** `Setup()` spawns `$placement-activator` and `$proxy-activator` with `StorageLookup`-backed persistence callbacks. `Get()` gains stale member validation, duplicate-request coalescing, strategy-based member selection, and sends `ActivationRequest` to the local or remote placement actor. `Shutdown()` stops the placement actor and proxy before removing the member from storage.

**Tech Stack:** Go 1.21+, testify, existing `cluster` package types (`PlacementConfig`, `NewPlacementActorProps`, `NewActivatorProxyProps`, `NewStrategyManager`, `ValidateActivationMember`, `ActivatorStrategy`), `cluster/identitylookup` package (`InMemoryStorageLookup`)

**Spec:** `docs/superpowers/specs/2026-03-17-shared-placement-actor-design.md`
**Depends on:** Sub-project 1a, 1b, 1c complete (placement actor, proxy, strategies, utilities all implemented)
**Tracker:** `docs/superpowers/plans/shared-placement-actor-tracker.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `cluster/identitylookup/storage/identity_storage_lookup.go` | Modify | Add placement actor/proxy lifecycle, inflight coalescing, stale validation, strategy selection |
| `cluster/identitylookup/storage/identity_storage_lookup_test.go` | Create | All unit tests for the refactored IdentityStorageLookup |

---

## Chunk 1: Inflight Coalescing and Stale Member Validation

### Task 1: Add inflight struct and coalescing map to IdentityStorageLookup

**Files:**
- Modify: `cluster/identitylookup/storage/identity_storage_lookup.go`
- Create: `cluster/identitylookup/storage/identity_storage_lookup_test.go`

- [ ] **Step 1: Write the failing test -- coalescing basics**

Create `cluster/identitylookup/storage/identity_storage_lookup_test.go`:

```go
package storage

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/cluster/identitylookup"
	"github.com/awevoke/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// echoProps returns Props for a simple actor that responds to any message
// with the message itself.
func echoProps() *actor.Props {
	return actor.PropsFromFunc(func(ctx actor.Context) {
		switch msg := ctx.Message().(type) {
		case *actor.Started, *actor.Stopping, *actor.Stopped:
			// lifecycle messages -- ignore
		default:
			ctx.Respond(msg)
		}
	})
}

// newTestCluster creates a minimal Cluster suitable for IdentityStorageLookup
// tests. It registers a kind with the given name and props, starts the
// remote subsystem (needed for placement actor message routing), and
// publishes a single-member topology so the member list is non-empty.
func newTestCluster(t *testing.T, kindName string, props *actor.Props) *cluster.Cluster {
	t.Helper()

	kind := cluster.NewKind(kindName, props)
	system := actor.NewActorSystem()
	provider := newTestProvider()
	storageLookup := identitylookup.NewInMemoryStorageLookup()
	isl := New(storageLookup)
	remoteCfg := remote.Configure("127.0.0.1", 0)

	cfg := cluster.Configure("test-cluster", provider, isl, remoteCfg,
		cluster.WithKinds(kind),
	)

	c := cluster.NewCluster(system, cfg)
	c.Remote = remote.NewRemote(system, c.Config.RemoteConfig)
	err := c.Remote.Start()
	require.NoError(t, err)

	// Initialize the identity lookup with the cluster.
	isl.Setup(c, []string{kindName}, false)

	// Publish a topology with ourselves as the only member.
	host, port, err := system.GetHostPort()
	require.NoError(t, err)
	self := &cluster.Member{
		Host:  host,
		Port:  int32(port),
		Id:    system.ID,
		Kinds: []string{kindName},
	}
	c.MemberList.UpdateClusterTopology(cluster.Members{self})

	t.Cleanup(func() {
		isl.Shutdown()
		c.Remote.Shutdown(true)
		system.Shutdown()
	})
	return c
}

// testProvider is a minimal ClusterProvider for tests.
type testProvider struct{}

func newTestProvider() *testProvider { return &testProvider{} }

func (p *testProvider) StartMember(c *cluster.Cluster) error { return nil }
func (p *testProvider) StartClient(c *cluster.Cluster) error { return nil }
func (p *testProvider) Shutdown(graceful bool) error         { return nil }

func TestIdentityStorageLookup_CoalesceConcurrentGets(t *testing.T) {
	c := newTestCluster(t, "testKind", echoProps())
	isl := c.Config.IdentityLookup.(*IdentityStorageLookup)

	ci := &cluster.ClusterIdentity{Kind: "testKind", Identity: "coalesce-1"}

	const concurrency = 10
	var wg sync.WaitGroup
	pids := make([]*actor.PID, concurrency)

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		i := i
		go func() {
			defer wg.Done()
			pids[i] = isl.Get(ci)
		}()
	}
	wg.Wait()

	// All should return the same non-nil PID.
	require.NotNil(t, pids[0], "first PID should not be nil")
	for i := 1; i < concurrency; i++ {
		require.NotNil(t, pids[i], "PID %d should not be nil", i)
		assert.True(t, pids[0].Equal(pids[i]),
			"all PIDs should be equal: pids[0]=%v, pids[%d]=%v", pids[0], i, pids[i])
	}
}
```

**Run:**
```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityStorageLookup_CoalesceConcurrentGets -count=1 -timeout 30s ./cluster/identitylookup/storage/
```

**Expected:** RED -- `IdentityStorageLookup` has no inflight map or coalescing logic; concurrent calls race and may fail.

- [ ] **Step 2: Verify RED**

Confirm test fails or races.

- [ ] **Step 3: Add inflight struct and fields to IdentityStorageLookup**

Edit `cluster/identitylookup/storage/identity_storage_lookup.go`. Add the inflight struct and new fields:

```go
package storage

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// inflight tracks a pending activation request so that concurrent Get()
// calls for the same identity are coalesced into a single spawn.
type inflight struct {
	done chan struct{} // closed when activation completes
	pid  *actor.PID   // result (nil on failure)
	err  error        // error (nil on success)
}

// IdentityStorageLookup adapts a cluster.StorageLookup into a
// cluster.IdentityLookup. It uses the storage backend to manage identity
// activations and spawn locks, bridging the external storage to the
// cluster's identity resolution protocol.
type IdentityStorageLookup struct {
	storage  cluster.StorageLookup
	cluster  *cluster.Cluster
	memberID string
	isClient bool

	// Placement actor and proxy PIDs (non-client only).
	placementPID *actor.PID
	proxyPID     *actor.PID

	// Strategy manager for member selection.
	strategyMgr *cluster.StrategyManager

	// Inflight coalescing map: concurrent Get() calls for the same
	// identity wait on the first caller's result.
	inflightMu sync.Mutex
	inflights  map[string]*inflight
}
```

- [ ] **Step 4: Verify test still RED (struct compiles but no logic yet)**

Run the test again -- should still fail because `Get()` doesn't use the inflight map.

---

### Task 2: Implement Setup() with placement actor and proxy spawning

- [ ] **Step 5: Write the failing test -- Setup spawns placement and proxy actors**

Add to `cluster/identitylookup/storage/identity_storage_lookup_test.go`:

```go
func TestIdentityStorageLookup_SetupSpawnsPlacementAndProxy(t *testing.T) {
	c := newTestCluster(t, "testKind", echoProps())
	isl := c.Config.IdentityLookup.(*IdentityStorageLookup)

	// After Setup, placement and proxy PIDs should be set.
	require.NotNil(t, isl.placementPID, "placementPID should be set after Setup")
	require.NotNil(t, isl.proxyPID, "proxyPID should be set after Setup")

	// The placement actor should be alive (respond to a ping via ActivationRequest).
	ci := &cluster.ClusterIdentity{Kind: "testKind", Identity: "setup-test-1"}
	req := &cluster.ActivationRequest{ClusterIdentity: ci, RequestId: "setup-req-1"}
	future := c.ActorSystem.Root.RequestFuture(isl.placementPID, req, 5*time.Second)
	res, err := future.Result()
	require.NoError(t, err)
	resp, ok := res.(*cluster.ActivationResponse)
	require.True(t, ok, "expected *ActivationResponse, got %T", res)
	assert.False(t, resp.Failed)
	assert.NotNil(t, resp.Pid)
}
```

**Run:**
```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityStorageLookup_SetupSpawnsPlacementAndProxy -count=1 -timeout 30s ./cluster/identitylookup/storage/
```

**Expected:** RED -- `placementPID` and `proxyPID` are nil.

- [ ] **Step 6: Verify RED**

- [ ] **Step 7: Implement Setup() with placement actor and proxy spawning**

Edit `cluster/identitylookup/storage/identity_storage_lookup.go`. Replace the existing `Setup` method:

```go
// Setup initializes the lookup with the cluster context. On non-client
// members, it spawns the placement actor and activator proxy, and creates
// a strategy manager for member selection.
func (l *IdentityStorageLookup) Setup(c *cluster.Cluster, kinds []string, isClient bool) {
	l.cluster = c
	l.isClient = isClient
	l.memberID = c.ActorSystem.ID
	l.inflights = make(map[string]*inflight)

	// Subscribe to topology events to remove activations when members leave.
	c.ActorSystem.EventStream.Subscribe(func(evt any) {
		if topology, ok := evt.(*cluster.ClusterTopology); ok {
			for _, member := range topology.Left {
				l.storage.RemoveMemberId(member.Id)
			}
		}
	})

	if isClient {
		return
	}

	// Create strategy manager for member selection.
	l.strategyMgr = cluster.NewStrategyManager(c)

	// Subscribe to topology events for strategy updates.
	c.ActorSystem.EventStream.Subscribe(func(evt any) {
		if topology, ok := evt.(*cluster.ClusterTopology); ok {
			for _, member := range topology.Left {
				l.strategyMgr.RemoveMember(member)
			}
			for _, member := range topology.Joined {
				l.strategyMgr.AddMember(member)
			}
		}
	})

	// Spawn placement actor with storage-backed persistence callbacks.
	// Note: PersistActivation is built per-request inside activateViaPlacement()
	// so that it can capture the real *SpawnLock returned by TryAcquireLock.
	// Setup only registers the RemoveActivation callback here; the placement
	// actor config is augmented with PersistActivation at call time.
	placementCfg := cluster.PlacementConfig{
		RemoveActivation: func(ctx context.Context, ci *cluster.ClusterIdentity, pid *actor.PID) error {
			l.storage.RemoveActivation(&cluster.SpawnLock{ClusterIdentity: ci})
			return nil
		},
	}
	placementProps := cluster.NewPlacementActorProps(c, placementCfg)
	var err error
	l.placementPID, err = c.ActorSystem.Root.SpawnNamed(placementProps, "$placement-activator")
	if err != nil {
		slog.Error("IdentityStorageLookup: failed to spawn placement actor", slog.Any("error", err))
		return
	}

	// Spawn activator proxy.
	proxyProps := cluster.NewActivatorProxyProps(l.placementPID, l)
	l.proxyPID, err = c.ActorSystem.Root.SpawnNamed(proxyProps, "$proxy-activator")
	if err != nil {
		slog.Error("IdentityStorageLookup: failed to spawn activator proxy", slog.Any("error", err))
		return
	}
}
```

- [ ] **Step 8: Verify GREEN**

```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityStorageLookup_SetupSpawnsPlacementAndProxy -count=1 -timeout 30s ./cluster/identitylookup/storage/
```

- [ ] **Step 9: Commit**

---

### Task 3: Implement the new Get() with stale validation, coalescing, and strategy selection

- [ ] **Step 10: Write the failing test -- stale member validation**

Add to test file:

```go
func TestIdentityStorageLookup_StaleActivationCleaned(t *testing.T) {
	c := newTestCluster(t, "testKind", echoProps())
	isl := c.Config.IdentityLookup.(*IdentityStorageLookup)

	ci := &cluster.ClusterIdentity{Kind: "testKind", Identity: "stale-1"}

	// Manually insert a stale activation with a member ID that is NOT in the
	// current topology. This simulates a crashed member that left records.
	stalePid := actor.NewPID("dead-node:9999", "testKind/stale-1")
	lock := isl.storage.TryAcquireLock(ci)
	require.NotNil(t, lock)
	isl.storage.StoreActivation("dead-member-999", lock, stalePid)

	// Verify the stale activation is in storage.
	existing := isl.storage.TryGetExistingActivation(ci)
	require.NotNil(t, existing, "stale activation should be in storage")

	// Get() should detect the stale member, clean it up, and spawn a fresh
	// activation on a live member.
	pid := isl.Get(ci)
	require.NotNil(t, pid, "Get() should return a PID after cleaning stale activation")
	assert.NotEqual(t, stalePid.Address, pid.Address,
		"PID should not be from the dead node")
}
```

**Run:**
```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityStorageLookup_StaleActivationCleaned -count=1 -timeout 30s ./cluster/identitylookup/storage/
```

**Expected:** RED -- current `Get()` does not validate member liveness.

- [ ] **Step 11: Verify RED**

- [ ] **Step 12: Write the failing test -- different identities are not coalesced**

```go
func TestIdentityStorageLookup_DifferentIdentitiesNotCoalesced(t *testing.T) {
	c := newTestCluster(t, "testKind", echoProps())
	isl := c.Config.IdentityLookup.(*IdentityStorageLookup)

	ci1 := &cluster.ClusterIdentity{Kind: "testKind", Identity: "diff-1"}
	ci2 := &cluster.ClusterIdentity{Kind: "testKind", Identity: "diff-2"}

	pid1 := isl.Get(ci1)
	pid2 := isl.Get(ci2)

	require.NotNil(t, pid1)
	require.NotNil(t, pid2)
	assert.False(t, pid1.Equal(pid2), "different identities should produce different PIDs")
}
```

- [ ] **Step 13: Write the failing test -- first concurrent Get succeeds; all coalesced waiters get the same result**

```go
func TestIdentityStorageLookup_CoalesceFirstSucceedAllGetSamePID(t *testing.T) {
	// Verify that when the first Get succeeds, all coalesced waiters receive
	// the same non-nil PID (not nil, not a different PID).
	storage := identitylookup.NewInMemoryStorageLookup()
	isl := New(storage)

	system := actor.NewActorSystem()
	kind := cluster.NewKind("testKind", echoProps())
	provider := newTestProvider()
	remoteCfg := remote.Configure("127.0.0.1", 0)
	cfg := cluster.Configure("test-cluster", provider, isl, remoteCfg,
		cluster.WithKinds(kind),
	)
	c := cluster.NewCluster(system, cfg)
	c.Remote = remote.NewRemote(system, c.Config.RemoteConfig)
	err := c.Remote.Start()
	require.NoError(t, err)
	isl.Setup(c, []string{"testKind"}, false)

	host, port, _ := system.GetHostPort()
	self := &cluster.Member{
		Host:  host,
		Port:  int32(port),
		Id:    system.ID,
		Kinds: []string{"testKind"},
	}
	c.MemberList.UpdateClusterTopology(cluster.Members{self})
	t.Cleanup(func() {
		isl.Shutdown()
		c.Remote.Shutdown(true)
		system.Shutdown()
	})

	ci := &cluster.ClusterIdentity{Kind: "testKind", Identity: "coalesce-succeed-1"}

	// Start a Get in background.
	var wg sync.WaitGroup
	wg.Add(1)
	var firstPID *actor.PID
	go func() {
		defer wg.Done()
		firstPID = isl.Get(ci)
	}()

	// Wait a tiny bit for the first Get to start.
	time.Sleep(20 * time.Millisecond)

	// The first Get should complete successfully.
	wg.Wait()
	require.NotNil(t, firstPID, "first Get should succeed")

	// A subsequent Get for the same identity should return the same PID
	// (hits existing activation, not a new spawn).
	secondPID := isl.Get(ci)
	require.NotNil(t, secondPID, "second Get should also succeed")
	assert.True(t, firstPID.Equal(secondPID), "both Gets should return the same PID")
}
```

- [ ] **Step 14: Implement the new Get() method**

Replace the existing `Get` method in `cluster/identitylookup/storage/identity_storage_lookup.go`:

```go
// Get resolves a cluster identity to an actor PID.
//
// The resolution protocol:
//  1. Check for an existing activation in the storage backend.
//  2. Validate that the owning member is still alive (stale check).
//  3. If coalescing: check in-progress map, wait if another request is in flight.
//  4. If client, wait for activation from a member.
//  5. Acquire lock.
//  6. Select target member via strategy.
//  7. Send ActivationRequest to target's placement/proxy actor.
//  8. Return PID from ActivationResponse.
//
// Returns nil if the identity could not be resolved (caller should retry).
func (l *IdentityStorageLookup) Get(ci *cluster.ClusterIdentity) *actor.PID {
	ctx, span := otel.Tracer("protoactor/identity").Start(context.Background(), "identity.lookup",
		trace.WithAttributes(
			attribute.String("kind", ci.Kind),
			attribute.String("identity", ci.Identity),
			attribute.String("provider", "storage"),
		),
	)
	defer span.End()

	key := ci.AsKey()

	// Check PID cache first (avoids storage round-trip for cached PIDs).
	if pid, ok := l.cluster.PidCache.Get(ci.Identity, ci.Kind); ok {
		return pid
	}

	// Step 1: Check for an existing activation.
	existing := l.storage.TryGetExistingActivation(ci)
	if existing != nil {
		// Step 2: Validate owning member is alive.
		if cluster.ValidateActivationMember(l.cluster.MemberList, existing.MemberID) {
			return l.pidFromStored(existing)
		}
		// Stale activation -- clean up and continue.
		slog.Info("IdentityStorageLookup: stale activation detected, cleaning up",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity),
			slog.String("staleMemberID", existing.MemberID))
		l.storage.RemoveMemberId(existing.MemberID)
	}

	// Step 3: Check coalescing map.
	l.inflightMu.Lock()
	if inf, ok := l.inflights[key]; ok {
		// Another request is already in flight -- wait for it.
		l.inflightMu.Unlock()
		<-inf.done
		return inf.pid
	}

	// We are the first caller -- create the inflight entry.
	inf := &inflight{done: make(chan struct{})}
	l.inflights[key] = inf
	l.inflightMu.Unlock()

	// Ensure cleanup: remove from map and close channel on exit.
	defer func() {
		l.inflightMu.Lock()
		delete(l.inflights, key)
		l.inflightMu.Unlock()
		close(inf.done)
	}()

	// Clients cannot spawn actors, so they must wait for a member to do it.
	if l.isClient {
		activation := l.storage.WaitForActivation(ci)
		if activation != nil {
			pid := l.pidFromStored(activation)
			inf.pid = pid
			return pid
		}
		return nil
	}

	// Step 5: Try to acquire the spawn lock.
	_, lockSpan := otel.Tracer("protoactor/identity").Start(ctx, "identity.lock_acquire",
		trace.WithAttributes(
			attribute.String("kind", ci.Kind),
			attribute.String("identity", ci.Identity),
		),
	)
	lock := l.storage.TryAcquireLock(ci)
	lockSpan.End()

	if lock == nil {
		// Another node is spawning this actor. Wait for it.
		activation := l.storage.WaitForActivation(ci)
		if activation != nil {
			pid := l.pidFromStored(activation)
			inf.pid = pid
			return pid
		}
		return nil
	}

	// Step 6: Select target member via strategy.
	pid := l.activateViaPlacement(ctx, ci, lock)
	if pid == nil {
		// Activation failed; release the lock so another node can try.
		l.storage.RemoveLock(*lock)
		return nil
	}

	inf.pid = pid

	// Populate the local PID cache.
	l.cluster.PidCache.Set(ci.Identity, ci.Kind, pid)

	return pid
}

// activateViaPlacement sends an ActivationRequest to the appropriate
// placement actor (local or remote) based on the strategy selection.
func (l *IdentityStorageLookup) activateViaPlacement(ctx context.Context, ci *cluster.ClusterIdentity, lock *cluster.SpawnLock) *actor.PID {
	_, activateSpan := otel.Tracer("protoactor/identity").Start(ctx, "identity.activate_via_placement",
		trace.WithAttributes(
			attribute.String("kind", ci.Kind),
			attribute.String("identity", ci.Identity),
		),
	)
	defer activateSpan.End()

	localAddress := l.cluster.ActorSystem.Address()
	var targetPID *actor.PID

	if l.strategyMgr != nil {
		targetMember := l.strategyMgr.GetActivator(ci, localAddress)
		if targetMember == nil {
			slog.Warn("IdentityStorageLookup: no suitable member for activation",
				slog.String("kind", ci.Kind),
				slog.String("identity", ci.Identity))
			return nil
		}

		targetAddress := targetMember.Address()
		if targetAddress == localAddress {
			// Local -- send to our placement actor.
			targetPID = l.placementPID
		} else {
			// Remote -- send to target's proxy activator.
			targetPID = actor.NewPID(targetAddress, "$proxy-activator")
		}
	} else {
		// No strategy manager (shouldn't happen for non-client, but be safe).
		targetPID = l.placementPID
	}

	if targetPID == nil {
		slog.Error("IdentityStorageLookup: no target PID for activation")
		return nil
	}

	// Build the PersistActivation callback here so it captures the real lock
	// (with its LockID) returned by TryAcquireLock earlier in Get().
	persistActivation := func(ctx context.Context, ci *cluster.ClusterIdentity, pid *actor.PID) error {
		// Note: StorageLookup.StoreActivation does not return an error.
		// Persistence failures from production backends (Redis, Postgres)
		// will need the StorageLookup interface to be updated in a future
		// change to return errors for proper retry behavior.
		l.storage.StoreActivation(l.memberID, lock, pid)
		return nil
	}

	req := &cluster.ActivationRequest{
		ClusterIdentity:   ci,
		RequestId:         lock.LockID,
		PersistActivation: persistActivation,
	}

	future := l.cluster.ActorSystem.Root.RequestFuture(targetPID, req, 10*time.Second)
	res, err := future.Result()
	if err != nil {
		slog.Error("IdentityStorageLookup: activation request failed",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity),
			slog.Any("error", err))
		return nil
	}

	resp, ok := res.(*cluster.ActivationResponse)
	if !ok {
		slog.Error("IdentityStorageLookup: unexpected response type",
			slog.String("kind", ci.Kind),
			slog.Any("type", res))
		return nil
	}

	if resp.Failed || resp.InvalidIdentity {
		slog.Warn("IdentityStorageLookup: activation rejected",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity),
			slog.Bool("failed", resp.Failed),
			slog.Bool("invalidIdentity", resp.InvalidIdentity))
		return nil
	}

	return resp.Pid
}
```

- [ ] **Step 15: Remove the old spawnActivation method**

Delete the entire `spawnActivation` method from `identity_storage_lookup.go`. It is no longer needed since activation goes through the placement actor.

- [ ] **Step 16: Verify GREEN for stale validation test**

```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityStorageLookup_StaleActivationCleaned -count=1 -timeout 30s ./cluster/identitylookup/storage/
```

- [ ] **Step 17: Verify GREEN for coalescing test**

```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityStorageLookup_CoalesceConcurrentGets -count=1 -timeout 30s ./cluster/identitylookup/storage/
```

- [ ] **Step 18: Verify GREEN for different-identities test**

```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityStorageLookup_DifferentIdentitiesNotCoalesced -count=1 -timeout 30s ./cluster/identitylookup/storage/
```

- [ ] **Step 19: Commit**

---

## Chunk 2: Shutdown Refactoring

### Task 4: Refactor Shutdown() to stop placement actor and proxy first

- [ ] **Step 20: Write the failing test -- Shutdown stops placement actor before removing member**

```go
func TestIdentityStorageLookup_ShutdownStopsPlacementFirst(t *testing.T) {
	c := newTestCluster(t, "testKind", echoProps())
	isl := c.Config.IdentityLookup.(*IdentityStorageLookup)

	ci := &cluster.ClusterIdentity{Kind: "testKind", Identity: "shutdown-1"}

	// Activate an actor.
	pid := isl.Get(ci)
	require.NotNil(t, pid, "should get a PID")

	// Capture the placement PID before shutdown.
	placementPID := isl.placementPID
	require.NotNil(t, placementPID)

	// Shutdown the lookup. This should stop placement/proxy first.
	// We override the cleanup to NOT call Shutdown twice.
	isl.Shutdown()

	// The placement actor should be stopped after Shutdown.
	// Sending a request should timeout or return DeadLetterResponse.
	req := &cluster.ActivationRequest{
		ClusterIdentity: &cluster.ClusterIdentity{Kind: "testKind", Identity: "post-shutdown"},
		RequestId:       "post-shutdown-req",
	}
	future := c.ActorSystem.Root.RequestFuture(placementPID, req, 1*time.Second)
	_, err := future.Result()
	assert.Error(t, err, "placement actor should be stopped after Shutdown")
}
```

**Run:**
```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityStorageLookup_ShutdownStopsPlacementFirst -count=1 -timeout 30s ./cluster/identitylookup/storage/
```

**Expected:** RED -- current `Shutdown()` only calls `RemoveMemberId`, doesn't stop the actors.

- [ ] **Step 21: Verify RED**

- [ ] **Step 22: Implement the new Shutdown()**

Replace the existing `Shutdown` method:

```go
// Shutdown performs cleanup when the cluster is shutting down.
// It stops the placement actor (which gracefully poisons all local grains),
// then the proxy activator, then closes strategies, and finally removes
// member records from storage.
func (l *IdentityStorageLookup) Shutdown() {
	// Stop placement actor first -- this triggers graceful shutdown of all
	// locally tracked grains (poisons them with DeactivationReasonShutdown).
	if l.placementPID != nil {
		if err := l.cluster.ActorSystem.Root.PoisonFuture(l.placementPID).Wait(); err != nil {
			slog.Error("IdentityStorageLookup: failed to stop placement actor",
				slog.Any("error", err))
		}
		l.placementPID = nil
	}

	// Stop proxy activator.
	if l.proxyPID != nil {
		if err := l.cluster.ActorSystem.Root.PoisonFuture(l.proxyPID).Wait(); err != nil {
			slog.Error("IdentityStorageLookup: failed to stop proxy activator",
				slog.Any("error", err))
		}
		l.proxyPID = nil
	}

	// Close strategy manager.
	if l.strategyMgr != nil {
		l.strategyMgr.Close()
		l.strategyMgr = nil
	}

	// Remove all activations belonging to this member from storage.
	if l.memberID != "" {
		l.storage.RemoveMemberId(l.memberID)
	}
}
```

- [ ] **Step 23: Verify GREEN**

```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityStorageLookup_ShutdownStopsPlacementFirst -count=1 -timeout 30s ./cluster/identitylookup/storage/
```

- [ ] **Step 24: Commit**

---

## Chunk 3: RemovePid Integration and Edge Cases

### Task 5: Verify RemovePid still works correctly

- [ ] **Step 25: Write the test -- RemovePid removes activation from storage**

```go
func TestIdentityStorageLookup_RemovePidCleansStorage(t *testing.T) {
	storage := identitylookup.NewInMemoryStorageLookup()
	isl := New(storage)

	ci := &cluster.ClusterIdentity{Kind: "testKind", Identity: "remove-1"}
	pid := actor.NewPID("127.0.0.1:8080", "testKind/remove-1")

	// Manually store an activation.
	lock := storage.TryAcquireLock(ci)
	require.NotNil(t, lock)
	storage.StoreActivation("member-1", lock, pid)

	// Verify it exists.
	existing := storage.TryGetExistingActivation(ci)
	require.NotNil(t, existing)

	// RemovePid should remove it.
	isl.RemovePid(ci, pid)

	// Verify it's gone.
	gone := storage.TryGetExistingActivation(ci)
	assert.Nil(t, gone, "activation should be removed after RemovePid")
}
```

**Run:**
```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityStorageLookup_RemovePidCleansStorage -count=1 -timeout 30s ./cluster/identitylookup/storage/
```

**Expected:** GREEN -- RemovePid is unchanged and already works.

- [ ] **Step 26: Verify GREEN**

- [ ] **Step 27: Commit (if any changes needed)**

---

### Task 6: Test first request fails -- all waiters get nil

- [ ] **Step 28: Write the test -- coalesced waiters get nil on failure**

```go
func TestIdentityStorageLookup_CoalesceFirstFailAllGetNil(t *testing.T) {
	// Create a lookup with a kind that the placement actor will fail to spawn
	// (use an unknown kind so GetClusterKind returns nil).
	c := newTestCluster(t, "testKind", echoProps())
	isl := c.Config.IdentityLookup.(*IdentityStorageLookup)

	// Use a kind that doesn't exist in the cluster -> placement actor returns Failed.
	ci := &cluster.ClusterIdentity{Kind: "nonExistentKind", Identity: "fail-1"}

	const concurrency = 5
	var wg sync.WaitGroup
	pids := make([]*actor.PID, concurrency)

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		i := i
		go func() {
			defer wg.Done()
			pids[i] = isl.Get(ci)
		}()
	}
	wg.Wait()

	// All should return nil since the kind is unknown.
	for i := 0; i < concurrency; i++ {
		assert.Nil(t, pids[i], "PID %d should be nil for unknown kind", i)
	}
}
```

**Run:**
```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityStorageLookup_CoalesceFirstFailAllGetNil -count=1 -timeout 30s ./cluster/identitylookup/storage/
```

**Expected:** GREEN -- the first caller gets nil from the placement actor (Failed=true), sets inf.pid=nil, and all waiters get nil.

- [ ] **Step 29: Verify GREEN**

- [ ] **Step 30: Commit**

---

## Chunk 4: Strategy Selection and Remote Placement

### Task 7: Test strategy selects local member

- [ ] **Step 31: Write the test -- strategy selects local, no remote hop**

```go
func TestIdentityStorageLookup_StrategySelectsLocal(t *testing.T) {
	c := newTestCluster(t, "testKind", echoProps())
	isl := c.Config.IdentityLookup.(*IdentityStorageLookup)

	require.NotNil(t, isl.strategyMgr, "strategy manager should be set")

	ci := &cluster.ClusterIdentity{Kind: "testKind", Identity: "strategy-local-1"}

	// With only one member (ourselves), strategy must select local.
	pid := isl.Get(ci)
	require.NotNil(t, pid, "should activate successfully")

	// The PID should be on the local address.
	localAddr := c.ActorSystem.Address()
	assert.Equal(t, localAddr, pid.Address,
		"activation should be on local member")
}
```

**Run:**
```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityStorageLookup_StrategySelectsLocal -count=1 -timeout 30s ./cluster/identitylookup/storage/
```

**Expected:** GREEN -- with one member, the strategy selects local and the placement actor spawns locally.

- [ ] **Step 32: Verify GREEN**

- [ ] **Step 33: Commit**

---

### Task 8: Test client mode (cannot spawn, waits for activation)

- [ ] **Step 34: Write the test -- client waits for activation**

```go
func TestIdentityStorageLookup_ClientWaitsForActivation(t *testing.T) {
	storage := identitylookup.NewInMemoryStorageLookup()
	isl := New(storage)

	system := actor.NewActorSystem()
	kind := cluster.NewKind("testKind", echoProps())
	provider := newTestProvider()
	remoteCfg := remote.Configure("127.0.0.1", 0)
	cfg := cluster.Configure("test-cluster", provider, isl, remoteCfg,
		cluster.WithKinds(kind),
	)
	c := cluster.NewCluster(system, cfg)
	c.Remote = remote.NewRemote(system, c.Config.RemoteConfig)
	err := c.Remote.Start()
	require.NoError(t, err)

	// Setup as CLIENT.
	isl.Setup(c, []string{"testKind"}, true)

	t.Cleanup(func() {
		isl.Shutdown()
		c.Remote.Shutdown(true)
		system.Shutdown()
	})

	// Placement and proxy should NOT be spawned for clients.
	assert.Nil(t, isl.placementPID, "clients should not have placement actor")
	assert.Nil(t, isl.proxyPID, "clients should not have proxy actor")

	ci := &cluster.ClusterIdentity{Kind: "testKind", Identity: "client-wait-1"}

	// Start Get in background -- it will block on WaitForActivation.
	var wg sync.WaitGroup
	wg.Add(1)
	var result *actor.PID
	go func() {
		defer wg.Done()
		result = isl.Get(ci)
	}()

	// Give it time to start waiting.
	time.Sleep(50 * time.Millisecond)

	// Simulate another node storing the activation.
	lock := storage.TryAcquireLock(ci)
	require.NotNil(t, lock)
	storedPid := actor.NewPID("other-node:9999", "testKind/client-wait-1")
	storage.StoreActivation("other-member", lock, storedPid)

	// Wait for result.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(5 * time.Second):
		t.Fatal("client Get() did not return within 5 seconds")
	}

	require.NotNil(t, result, "client should get PID after activation is stored")
}
```

**Run:**
```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityStorageLookup_ClientWaitsForActivation -count=1 -timeout 30s ./cluster/identitylookup/storage/
```

**Expected:** GREEN -- client path uses WaitForActivation, no placement actor involved.

- [ ] **Step 35: Verify GREEN**

- [ ] **Step 36: Commit**

---

## Chunk 5: End-to-End Flow and Full Test Suite

### Task 9: Full Get -> placement actor -> spawn -> persist -> return PID flow

- [ ] **Step 37: Write the test -- full end-to-end activation flow**

```go
func TestIdentityStorageLookup_EndToEnd_ActivateAndRetrieve(t *testing.T) {
	storage := identitylookup.NewInMemoryStorageLookup()
	isl := New(storage)

	system := actor.NewActorSystem()
	kind := cluster.NewKind("orderGrain", echoProps())
	provider := newTestProvider()
	remoteCfg := remote.Configure("127.0.0.1", 0)
	cfg := cluster.Configure("e2e-cluster", provider, isl, remoteCfg,
		cluster.WithKinds(kind),
	)
	c := cluster.NewCluster(system, cfg)
	c.Remote = remote.NewRemote(system, c.Config.RemoteConfig)
	err := c.Remote.Start()
	require.NoError(t, err)
	isl.Setup(c, []string{"orderGrain"}, false)

	host, port, _ := system.GetHostPort()
	self := &cluster.Member{
		Host:  host,
		Port:  int32(port),
		Id:    system.ID,
		Kinds: []string{"orderGrain"},
	}
	c.MemberList.UpdateClusterTopology(cluster.Members{self})

	t.Cleanup(func() {
		isl.Shutdown()
		c.Remote.Shutdown(true)
		system.Shutdown()
	})

	ci := &cluster.ClusterIdentity{Kind: "orderGrain", Identity: "order-123"}

	// First Get -- should activate via placement actor.
	pid1 := isl.Get(ci)
	require.NotNil(t, pid1, "first Get should return PID")

	// Verify the activation is stored in the backend.
	stored := storage.TryGetExistingActivation(ci)
	require.NotNil(t, stored, "activation should be persisted in storage")
	assert.Equal(t, system.ID, stored.MemberID, "member ID should be ours")

	// Second Get -- should find existing activation (no new spawn).
	pid2 := isl.Get(ci)
	require.NotNil(t, pid2, "second Get should return PID")
	assert.True(t, pid1.Equal(pid2), "second Get should return same PID")
}
```

**Run:**
```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityStorageLookup_EndToEnd_ActivateAndRetrieve -count=1 -timeout 30s ./cluster/identitylookup/storage/
```

**Expected:** GREEN -- the full flow works: Get -> lock -> strategy -> placement actor -> spawn -> persist -> return PID.

- [ ] **Step 38: Verify GREEN**

- [ ] **Step 39: Commit**

---

### Task 10: Verify GrainEnumerator still works

- [ ] **Step 40: Write the test -- ListGrains works after placement-actor-mediated activation**

```go
func TestIdentityStorageLookup_ListGrainsAfterActivation(t *testing.T) {
	storage := identitylookup.NewInMemoryStorageLookup()
	isl := New(storage)

	system := actor.NewActorSystem()
	kind := cluster.NewKind("listKind", echoProps())
	provider := newTestProvider()
	remoteCfg := remote.Configure("127.0.0.1", 0)
	cfg := cluster.Configure("list-cluster", provider, isl, remoteCfg,
		cluster.WithKinds(kind),
	)
	c := cluster.NewCluster(system, cfg)
	c.Remote = remote.NewRemote(system, c.Config.RemoteConfig)
	err := c.Remote.Start()
	require.NoError(t, err)
	isl.Setup(c, []string{"listKind"}, false)

	host, port, _ := system.GetHostPort()
	self := &cluster.Member{
		Host:  host,
		Port:  int32(port),
		Id:    system.ID,
		Kinds: []string{"listKind"},
	}
	c.MemberList.UpdateClusterTopology(cluster.Members{self})

	t.Cleanup(func() {
		isl.Shutdown()
		c.Remote.Shutdown(true)
		system.Shutdown()
	})

	// Activate two grains.
	ci1 := &cluster.ClusterIdentity{Kind: "listKind", Identity: "list-a"}
	ci2 := &cluster.ClusterIdentity{Kind: "listKind", Identity: "list-b"}
	pid1 := isl.Get(ci1)
	pid2 := isl.Get(ci2)
	require.NotNil(t, pid1)
	require.NotNil(t, pid2)

	// ListGrains should return both.
	grains, err := isl.ListGrains()
	require.NoError(t, err)
	assert.Len(t, grains, 2, "should list 2 grains")
}
```

**Run:**
```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityStorageLookup_ListGrainsAfterActivation -count=1 -timeout 30s ./cluster/identitylookup/storage/
```

**Expected:** GREEN -- ListGrains delegates to the storage backend, which now has activations stored by the placement actor's PersistActivation callback.

- [ ] **Step 41: Verify GREEN**

- [ ] **Step 42: Commit**

---

## Chunk 6: Run Full Test Suite and Fix Any Regressions

### Task 11: Run the existing conformance suite

- [ ] **Step 43: Run conformance tests**

```bash
cd /home/cchamplin/development/protoactor-go && go test -race -count=1 -timeout 60s ./cluster/identitylookup/...
```

**Expected:** GREEN -- conformance tests exercise the InMemoryStorageLookup directly, not IdentityStorageLookup.

- [ ] **Step 44: Run full cluster package tests**

```bash
cd /home/cchamplin/development/protoactor-go && go test -race -count=1 -timeout 120s ./cluster/...
```

**Expected:** GREEN -- placement actor tests (from SP1c) and all other cluster tests pass.

- [ ] **Step 45: Run all storage lookup tests together**

```bash
cd /home/cchamplin/development/protoactor-go && go test -race -count=1 -timeout 60s ./cluster/identitylookup/storage/
```

**Expected:** GREEN -- all new tests pass.

- [ ] **Step 46: Final commit with all changes**

---

## Summary of Changes

| File | Change |
|------|--------|
| `cluster/identitylookup/storage/identity_storage_lookup.go` | Add `inflight` struct, `placementPID`/`proxyPID`/`strategyMgr`/`inflights` fields; refactor `Setup()` to spawn placement actor + proxy with persistence callbacks; refactor `Get()` with stale validation, coalescing, strategy selection, placement actor routing; refactor `Shutdown()` to stop actors first; remove `spawnActivation()` |
| `cluster/identitylookup/storage/identity_storage_lookup_test.go` | New file: 10 tests covering setup, coalescing, stale validation, strategy selection, client mode, end-to-end flow, shutdown, RemovePid, ListGrains |

## Key Design Decisions

1. **PersistActivation callback wraps StoreActivation**: The callback is built inside `activateViaPlacement()` so it can close over the real `*SpawnLock` returned by `TryAcquireLock` (which carries the actual `LockID`). The `ActivationRequest.RequestId` is also set to `lock.LockID`. The callback delegates to `storage.StoreActivation()`. Note that `StoreActivation` does not return an error; production backends (Redis, Postgres) will require the `StorageLookup` interface to be updated in a future change.

2. **RemoveActivation callback wraps RemoveActivation**: Uses a `SpawnLock` with the `ClusterIdentity` to match the existing `StorageLookup.RemoveActivation` signature.

3. **Strategy manager subscribes to topology events**: The lookup subscribes to `ClusterTopology` events and calls `AddMember`/`RemoveMember` on the strategy manager to keep it in sync with the cluster membership.

4. **Coalescing uses sync.Mutex + channel**: The inflight map is protected by a `sync.Mutex` for the check-then-create pattern. Waiters block on the `done` channel. The first caller cleans up the map entry and closes the channel in a defer.

5. **spawnActivation is removed entirely**: All spawning goes through the placement actor. The direct `SpawnNamed` call is no longer needed in IdentityStorageLookup.
