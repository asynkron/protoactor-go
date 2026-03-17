# Sub-project 4: Integrate Shared Placement Actor into natsstream

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor `natsstream.IdentityLookup` to use the shared placement actor and activator proxy from Sub-project 1c. This replaces the direct `spawnActivation` method with placement actor-mediated spawning, adds stale member validation, duplicate-request coalescing, and member strategy selection.

**Architecture:** `Setup()` spawns `$placement-activator` and `$proxy-activator` with natsstream-specific persistence callbacks. `Get()` gains stale member validation, duplicate-request coalescing, strategy-based member selection, and sends `ActivationRequest` to the local or remote placement actor. `Shutdown()` stops the placement actor and proxy before removing the member from storage.

**Key differences from natskv:**
- natsstream uses NATS JetStream Streams (not KV) for identity storage
- Member tracking uses an in-memory `memberKeys` map (not a persistent KV bucket)
- Lock acquisition uses `jetstream.WithExpectLastSequencePerSubject(0)` CAS publish
- Activation storage uses `jetstream.WithExpectLastSequencePerSubject(lockSeq)` CAS publish
- Removal uses `identityStream.Purge(ctx, jetstream.WithPurgeSubject(subject))`
- No client activation support (natsstream doesn't have `handleActivationRequest` like natskv)

**Tech Stack:** Go 1.21+, testify, embedded NATS server, existing `cluster` package types (`PlacementConfig`, `NewPlacementActorProps`, `NewActivatorProxyProps`, `NewStrategyManager`, `ValidateActivationMember`, `ActivatorStrategy`, `ErrLockNotHeld`), existing natsstream test helpers (`startEmbeddedNATS`, `setupCluster`, `setupClusterWithKindsEmbedded`, `connectNATS`)

**Spec:** `docs/superpowers/specs/2026-03-17-shared-placement-actor-design.md`
**Depends on:** Sub-project 1a, 1b, 1c complete (placement actor, proxy, strategies, utilities all implemented)
**Tracker:** `docs/superpowers/plans/shared-placement-actor-tracker.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `cluster/clusterproviders/natsstream/natsstream_identity.go` | Modify | Add placement actor/proxy lifecycle, inflight coalescing, stale validation, strategy selection; remove `spawnActivation` |
| `cluster/clusterproviders/natsstream/natsstream_identity_test.go` | Modify | Update existing tests, add new tests for coalescing, stale validation, strategy selection, shutdown ordering |
| `cluster/clusterproviders/natsstream/testhelpers_test.go` | Modify | Add `setupClusterWithTopology` helper for tests that need a live member list |

---

## Chunk 1: Add Inflight Coalescing and New Fields

### Task 1: Add inflight struct and new fields to IdentityLookup

**Files:**
- Modify: `cluster/clusterproviders/natsstream/natsstream_identity.go`
- Modify: `cluster/clusterproviders/natsstream/natsstream_identity_test.go`

- [ ] **Step 1: Write the failing test -- coalescing concurrent Gets**

Add to `cluster/clusterproviders/natsstream/natsstream_identity_test.go`:

```go
func TestIdentityLookup_CoalesceConcurrentGets(t *testing.T) {
	srv := startEmbeddedNATS(t)
	kindProps := actor.PropsFromFunc(func(ctx actor.Context) {})
	p, c := setupClusterWithKindsEmbedded(t, srv, "test-coalesce",
		[]*cluster.Kind{cluster.NewKind("TestKind", kindProps)})
	c.InitKindsForTest(cluster.NewKind("TestKind", kindProps))

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	il := p.IdentityLookup()
	il.Setup(c, []string{"TestKind"}, false)
	time.Sleep(500 * time.Millisecond)

	// Publish a single-member topology so the strategy manager has a member.
	self := &cluster.Member{
		Host:  "127.0.0.1",
		Port:  0,
		Id:    c.ActorSystem.ID,
		Kinds: []string{"TestKind"},
	}
	c.MemberList.UpdateClusterTopology(cluster.Members{self})

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "coalesce-1"}

	const concurrency = 10
	var wg sync.WaitGroup
	pids := make([]*actor.PID, concurrency)

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		i := i
		go func() {
			defer wg.Done()
			pids[i] = il.Get(ci)
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

Add required import `"sync"` to the test file imports.

**Run:**
```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityLookup_CoalesceConcurrentGets -count=1 -timeout 30s ./cluster/clusterproviders/natsstream/
```

**Expected:** RED -- `IdentityLookup` has no inflight map or coalescing logic; concurrent calls race and may produce different PIDs or fail.

- [ ] **Step 2: Verify RED**

Confirm test fails or races.

- [ ] **Step 3: Add inflight struct and new fields to IdentityLookup**

Edit `cluster/clusterproviders/natsstream/natsstream_identity.go`. Add the inflight struct and new fields to the `IdentityLookup` struct:

```go
// inflight tracks a pending activation request so that concurrent Get()
// calls for the same identity are coalesced into a single spawn.
type inflight struct {
	done chan struct{} // closed when activation completes
	pid  *actor.PID   // result (nil on failure)
	err  error        // error (nil on success)
}
```

Add these fields to the `IdentityLookup` struct:

```go
type IdentityLookup struct {
	provider              *Provider
	cluster               *cluster.Cluster
	memberID              string
	isClient              bool
	identityStream        jetstream.Stream
	identitySubjectPrefix string // e.g. "identities.<clusterName>"
	config                *config
	semaphore             chan struct{}
	memberKeys            map[string][]string // memberID -> list of identity subject keys
	memberKeysMu          sync.Mutex

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

## Chunk 2: Refactor Setup() to Spawn Placement Actor and Proxy

### Task 2: Implement Setup() with placement actor and proxy spawning

**Files:**
- Modify: `cluster/clusterproviders/natsstream/natsstream_identity.go`
- Modify: `cluster/clusterproviders/natsstream/natsstream_identity_test.go`
- Modify: `cluster/clusterproviders/natsstream/testhelpers_test.go`

- [ ] **Step 5: Write the failing test -- Setup spawns placement and proxy actors**

Add to `cluster/clusterproviders/natsstream/natsstream_identity_test.go`:

```go
func TestIdentityLookup_SetupSpawnsPlacementAndProxy(t *testing.T) {
	srv := startEmbeddedNATS(t)
	kindProps := actor.PropsFromFunc(func(ctx actor.Context) {})
	p, c := setupClusterWithKindsEmbedded(t, srv, "test-setup-actors",
		[]*cluster.Kind{cluster.NewKind("TestKind", kindProps)})
	c.InitKindsForTest(cluster.NewKind("TestKind", kindProps))

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	il := p.IdentityLookup()
	il.Setup(c, []string{"TestKind"}, false)
	time.Sleep(500 * time.Millisecond)

	// After Setup (non-client), placement and proxy PIDs should be set.
	require.NotNil(t, il.placementPID, "placementPID should be set after Setup")
	require.NotNil(t, il.proxyPID, "proxyPID should be set after Setup")

	// Publish topology so placement actor can find the kind.
	self := &cluster.Member{
		Host:  "127.0.0.1",
		Port:  0,
		Id:    c.ActorSystem.ID,
		Kinds: []string{"TestKind"},
	}
	c.MemberList.UpdateClusterTopology(cluster.Members{self})

	// The placement actor should be alive and respond to an ActivationRequest.
	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "setup-test-1"}
	req := &cluster.ActivationRequest{ClusterIdentity: ci, RequestId: "setup-req-1"}
	future := c.ActorSystem.Root.RequestFuture(il.placementPID, req, 5*time.Second)
	res, err := future.Result()
	require.NoError(t, err)
	resp, ok := res.(*cluster.ActivationResponse)
	require.True(t, ok, "expected *ActivationResponse, got %T", res)
	assert.False(t, resp.Failed)
	assert.NotNil(t, resp.Pid)
}

func TestIdentityLookup_ClientSetupSkipsPlacementActor(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-client-setup")

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	il := p.IdentityLookup()
	il.Setup(c, []string{"TestKind"}, true)
	time.Sleep(500 * time.Millisecond)

	// Clients should NOT have placement actor or proxy.
	assert.Nil(t, il.placementPID, "clients should not have placement actor")
	assert.Nil(t, il.proxyPID, "clients should not have proxy actor")
}
```

**Run:**
```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityLookup_SetupSpawnsPlacementAndProxy -count=1 -timeout 30s ./cluster/clusterproviders/natsstream/
```

**Expected:** RED -- `placementPID` and `proxyPID` are nil.

- [ ] **Step 6: Verify RED**

- [ ] **Step 7: Add a test helper that creates a cluster with topology for placement-mediated tests**

Add to `cluster/clusterproviders/natsstream/testhelpers_test.go`:

```go
// setupClusterWithTopology creates a provider, cluster, and identity lookup
// with a single-member topology already published. This is needed for tests
// that use the placement actor (which requires a live member list for strategy
// selection).
func setupClusterWithTopology(t *testing.T, srv *server.Server, clusterName string, kinds []*cluster.Kind, opts ...Option) (*Provider, *cluster.Cluster, *IdentityLookup) {
	t.Helper()

	p, c := setupClusterWithKindsEmbedded(t, srv, clusterName, kinds, opts...)
	for _, k := range kinds {
		c.InitKindsForTest(k)
	}

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	il := p.IdentityLookup()
	il.Setup(c, kindNames(kinds), false)
	time.Sleep(500 * time.Millisecond)

	// Publish a single-member topology.
	self := &cluster.Member{
		Host:  "127.0.0.1",
		Port:  0,
		Id:    c.ActorSystem.ID,
		Kinds: kindNames(kinds),
	}
	c.MemberList.UpdateClusterTopology(cluster.Members{self})

	return p, c, il
}

// kindNames extracts kind name strings from a slice of cluster.Kind.
func kindNames(kinds []*cluster.Kind) []string {
	names := make([]string, len(kinds))
	for i, k := range kinds {
		names[i] = k.Kind
	}
	return names
}
```

- [ ] **Step 8: Implement Setup() with placement actor and proxy spawning**

Edit `cluster/clusterproviders/natsstream/natsstream_identity.go`. Replace the existing `Setup` method. The key changes are:
1. Initialize the `inflights` map.
2. On non-client nodes: create a `StrategyManager`, subscribe to topology events for strategy updates, spawn the placement actor with natsstream persistence callbacks, spawn the activator proxy.

The `PersistActivation` callback captures the identity lookup and calls `il.storeActivation()`. Since `storeActivation` requires `lockID` and `lockSeq` which the placement actor doesn't have, we need to adapt: the placement actor's `PersistActivation` callback signature is `func(ctx context.Context, ci *ClusterIdentity, pid *actor.PID) error`. The lock is already held at this point (acquired by the Get() path), and `storeActivation` uses the lock's sequence for CAS. However, since the placement actor doesn't know the lock details, the persistence callback must capture them from the Get() caller.

**Design decision:** The placement actor's `PersistActivation` callback does NOT have access to lock IDs or sequences. Instead, the natsstream callback will publish the activation record directly (bypassing the CAS check), since the placement actor already prevents duplicate spawns via its in-flight tracking set. The lock's CAS check is no longer needed because the placement actor serializes all ActivationRequests for the same identity.

The callback:
```go
PersistActivation: func(ctx context.Context, ci *cluster.ClusterIdentity, pid *actor.PID) error {
    subject := il.identitySubject(ci)
    rec := activationRecord{
        PidID:      pid.Id,
        PidAddress: pid.Address,
        MemberID:   il.memberID,
    }
    data, err := json.Marshal(&rec)
    if err != nil {
        return fmt.Errorf("natsstream identity: PersistActivation marshal: %w", err)
    }
    _, err = il.provider.js.Publish(ctx, subject, data)
    if err != nil {
        return fmt.Errorf("natsstream identity: PersistActivation publish: %w", err)
    }
    il.addKeyToMember(il.memberID, subject)
    return nil
},
```

The `RemoveActivation` callback:
```go
RemoveActivation: func(ctx context.Context, ci *cluster.ClusterIdentity, pid *actor.PID) error {
    subject := il.identitySubject(ci)
    if err := il.identityStream.Purge(ctx, jetstream.WithPurgeSubject(subject)); err != nil {
        return fmt.Errorf("natsstream identity: RemoveActivation purge: %w", err)
    }
    il.removeKeyFromMember(il.memberID, subject)
    return nil
},
```

Full replacement for `Setup`:

```go
// Setup initializes the identity lookup with the cluster context, creates the
// identity stream, subscribes to topology events for member cleanup, and
// spawns the placement actor and proxy on non-client members.
func (il *IdentityLookup) Setup(c *cluster.Cluster, kinds []string, isClient bool) {
	il.cluster = c
	il.memberID = c.ActorSystem.ID
	il.isClient = isClient
	il.inflights = make(map[string]*inflight)

	ctx := context.Background()
	js := il.provider.js
	clusterName := c.Config.Name

	// Use a separate subject namespace to avoid overlapping with the cluster
	// membership stream which uses "<prefix>.>".
	il.identitySubjectPrefix = "identities." + clusterName

	streamName := il.config.identityStreamName(clusterName)
	s, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:              streamName,
		Subjects:          []string{il.identitySubjectPrefix + ".>"},
		MaxMsgsPerSubject: 1,
		Retention:         jetstream.LimitsPolicy,
		Storage:           il.config.Storage,
		Replicas:          il.config.Replicas,
	})
	if err != nil {
		slog.Error("natsstream identity: failed to create identity stream",
			slog.Any("error", err))
		return
	}
	il.identityStream = s

	// Subscribe to ClusterTopology events to clean up when members leave.
	c.ActorSystem.EventStream.Subscribe(func(evt any) {
		if topology, ok := evt.(*cluster.ClusterTopology); ok {
			for _, member := range topology.Left {
				il.removeMemberID(context.Background(), member.Id)
			}
		}
	})

	if isClient {
		return
	}

	// Create strategy manager for member selection.
	il.strategyMgr = cluster.NewStrategyManager(c)

	// Subscribe to topology events for strategy updates.
	c.ActorSystem.EventStream.Subscribe(func(evt any) {
		if topology, ok := evt.(*cluster.ClusterTopology); ok {
			for _, member := range topology.Left {
				il.strategyMgr.RemoveMember(member)
			}
			for _, member := range topology.Joined {
				il.strategyMgr.AddMember(member)
			}
		}
	})

	// Spawn placement actor with natsstream persistence callbacks.
	placementCfg := cluster.PlacementConfig{
		PersistActivation: func(ctx context.Context, ci *cluster.ClusterIdentity, pid *actor.PID) error {
			subject := il.identitySubject(ci)
			rec := activationRecord{
				PidID:      pid.Id,
				PidAddress: pid.Address,
				MemberID:   il.memberID,
			}
			data, mErr := json.Marshal(&rec)
			if mErr != nil {
				return fmt.Errorf("natsstream identity: PersistActivation marshal: %w", mErr)
			}
			_, pErr := il.provider.js.Publish(ctx, subject, data)
			if pErr != nil {
				return fmt.Errorf("natsstream identity: PersistActivation publish: %w", pErr)
			}
			il.addKeyToMember(il.memberID, subject)
			return nil
		},
		RemoveActivation: func(ctx context.Context, ci *cluster.ClusterIdentity, pid *actor.PID) error {
			subject := il.identitySubject(ci)
			if pErr := il.identityStream.Purge(ctx, jetstream.WithPurgeSubject(subject)); pErr != nil {
				return fmt.Errorf("natsstream identity: RemoveActivation purge: %w", pErr)
			}
			il.removeKeyFromMember(il.memberID, subject)
			return nil
		},
	}
	placementProps := cluster.NewPlacementActorProps(c, placementCfg)
	il.placementPID, err = c.ActorSystem.Root.SpawnNamed(placementProps, "$placement-activator")
	if err != nil {
		slog.Error("natsstream identity: failed to spawn placement actor", slog.Any("error", err))
		return
	}

	// Spawn activator proxy.
	proxyProps := cluster.NewActivatorProxyProps(il.placementPID, il)
	il.proxyPID, err = c.ActorSystem.Root.SpawnNamed(proxyProps, "$proxy-activator")
	if err != nil {
		slog.Error("natsstream identity: failed to spawn activator proxy", slog.Any("error", err))
		return
	}
}
```

- [ ] **Step 9: Verify GREEN for Setup tests**

```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run "TestIdentityLookup_SetupSpawnsPlacementAndProxy|TestIdentityLookup_ClientSetupSkipsPlacementActor" -count=1 -timeout 30s ./cluster/clusterproviders/natsstream/
```

- [ ] **Step 10: Commit**

---

## Chunk 3: Refactor Get() with Stale Validation, Coalescing, and Strategy Selection

### Task 3: Implement the new Get() method

**Files:**
- Modify: `cluster/clusterproviders/natsstream/natsstream_identity.go`
- Modify: `cluster/clusterproviders/natsstream/natsstream_identity_test.go`

- [ ] **Step 11: Write the failing test -- stale member validation**

Add to `cluster/clusterproviders/natsstream/natsstream_identity_test.go`:

```go
func TestIdentityLookup_StaleActivationCleaned(t *testing.T) {
	srv := startEmbeddedNATS(t)
	kindProps := actor.PropsFromFunc(func(ctx actor.Context) {})
	_, c, il := setupClusterWithTopology(t, srv, "test-stale-clean",
		[]*cluster.Kind{cluster.NewKind("TestKind", kindProps)})

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "stale-1"}
	ctx := context.Background()

	// Manually plant a stale activation with a dead member ID.
	subject := il.identitySubject(ci)
	staleRec := activationRecord{
		PidID:      "TestKind/stale-1",
		PidAddress: "dead-node:9999",
		MemberID:   "dead-member-999",
	}
	data, err := json.Marshal(&staleRec)
	require.NoError(t, err)
	_, err = il.provider.js.Publish(ctx, subject, data)
	require.NoError(t, err)

	// Verify the stale activation is in the stream.
	rec := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec, "stale activation should be in stream")
	assert.Equal(t, "dead-node:9999", rec.PidAddress)

	// Get() should detect the stale member, clean it up, and spawn a fresh
	// activation on the live member.
	pid := il.Get(ci)
	require.NotNil(t, pid, "Get() should return a PID after cleaning stale activation")
	assert.NotEqual(t, "dead-node:9999", pid.Address,
		"PID should not be from the dead node")
	assert.Equal(t, c.ActorSystem.Address(), pid.Address,
		"PID should be on the local (live) member")
}
```

**Run:**
```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityLookup_StaleActivationCleaned -count=1 -timeout 30s ./cluster/clusterproviders/natsstream/
```

**Expected:** RED -- current `Get()` does not validate member liveness; it returns the stale PID directly.

- [ ] **Step 12: Verify RED**

- [ ] **Step 13: Write the failing test -- different identities are not coalesced**

```go
func TestIdentityLookup_DifferentIdentitiesNotCoalesced(t *testing.T) {
	srv := startEmbeddedNATS(t)
	kindProps := actor.PropsFromFunc(func(ctx actor.Context) {})
	_, _, il := setupClusterWithTopology(t, srv, "test-diff-identity",
		[]*cluster.Kind{cluster.NewKind("TestKind", kindProps)})

	ci1 := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "diff-1"}
	ci2 := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "diff-2"}

	pid1 := il.Get(ci1)
	pid2 := il.Get(ci2)

	require.NotNil(t, pid1)
	require.NotNil(t, pid2)
	assert.False(t, pid1.Equal(pid2), "different identities should produce different PIDs")
}
```

- [ ] **Step 14: Write the failing test -- coalesced waiters get nil on failure**

```go
func TestIdentityLookup_CoalesceFirstFailAllGetNil(t *testing.T) {
	srv := startEmbeddedNATS(t)
	kindProps := actor.PropsFromFunc(func(ctx actor.Context) {})
	_, _, il := setupClusterWithTopology(t, srv, "test-coalesce-fail",
		[]*cluster.Kind{cluster.NewKind("TestKind", kindProps)})

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
			pids[i] = il.Get(ci)
		}()
	}
	wg.Wait()

	// All should return nil since the kind is unknown.
	for i := 0; i < concurrency; i++ {
		assert.Nil(t, pids[i], "PID %d should be nil for unknown kind", i)
	}
}
```

- [ ] **Step 15: Implement the new Get() method**

Replace the existing `Get` method in `cluster/clusterproviders/natsstream/natsstream_identity.go`:

```go
// Get resolves a cluster identity to an actor PID.
//
// The resolution protocol:
//  1. Check for an existing activation in the stream.
//  2. Validate that the owning member is still alive (stale check).
//  3. If coalescing: check in-progress map, wait if another request is in flight.
//  4. If client, wait for activation (cannot spawn).
//  5. Acquire lock.
//  6. Select target member via strategy.
//  7. Send ActivationRequest to target's placement/proxy actor.
//  8. Return PID from ActivationResponse.
//
// Returns nil if the identity could not be resolved (caller should retry).
func (il *IdentityLookup) Get(ci *cluster.ClusterIdentity) *actor.PID {
	il.acquire()
	defer il.release()

	ctx := context.Background()
	key := ci.AsKey()

	// Step 1: Check for existing activation.
	rec := il.getExistingActivation(ctx, ci)
	if rec != nil {
		// Step 2: Validate owning member is alive.
		if cluster.ValidateActivationMember(il.cluster.MemberList, rec.MemberID) {
			return pidFromRecord(rec)
		}
		// Stale activation -- clean up and continue.
		slog.Info("natsstream identity: stale activation detected, cleaning up",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity),
			slog.String("staleMemberID", rec.MemberID))
		il.removeMemberID(ctx, rec.MemberID)
	}

	// Step 3: Check coalescing map.
	il.inflightMu.Lock()
	if inf, ok := il.inflights[key]; ok {
		// Another request is already in flight -- wait for it.
		il.inflightMu.Unlock()
		<-inf.done
		return inf.pid
	}

	// We are the first caller -- create the inflight entry.
	inf := &inflight{done: make(chan struct{})}
	il.inflights[key] = inf
	il.inflightMu.Unlock()

	// Ensure cleanup: remove from map and close channel on exit.
	defer func() {
		if r := recover(); r != nil {
			inf.err = fmt.Errorf("panic in Get: %v", r)
		}
		il.inflightMu.Lock()
		delete(il.inflights, key)
		il.inflightMu.Unlock()
		close(inf.done)
	}()

	// Step 4: If client, wait for activation (cannot spawn).
	if il.isClient {
		rec = il.waitForActivation(ctx, ci)
		if rec != nil {
			pid := pidFromRecord(rec)
			inf.pid = pid
			return pid
		}
		return nil
	}

	// Step 5: Try to acquire the spawn lock.
	_, _, ok := il.tryAcquireLock(ctx, ci)
	if !ok {
		// Another node is spawning. Wait for it.
		rec = il.waitForActivation(ctx, ci)
		if rec != nil {
			pid := pidFromRecord(rec)
			inf.pid = pid
			return pid
		}
		return nil
	}

	// Step 6+7: Select target member via strategy and send ActivationRequest.
	pid := il.activateViaPlacement(ctx, ci)
	if pid == nil {
		// Activation failed; purge the lock subject so another node can try.
		subject := il.identitySubject(ci)
		_ = il.identityStream.Purge(ctx, jetstream.WithPurgeSubject(subject))
		return nil
	}

	inf.pid = pid

	// Populate the local PID cache.
	il.cluster.PidCache.Set(ci.Identity, ci.Kind, pid)

	return pid
}

// activateViaPlacement sends an ActivationRequest to the appropriate
// placement actor (local or remote) based on the strategy selection.
func (il *IdentityLookup) activateViaPlacement(ctx context.Context, ci *cluster.ClusterIdentity) *actor.PID {
	localAddress := il.cluster.ActorSystem.Address()
	var targetPID *actor.PID

	if il.strategyMgr != nil {
		targetMember := il.strategyMgr.GetActivator(ci, localAddress)
		if targetMember == nil {
			slog.Warn("natsstream identity: no suitable member for activation",
				slog.String("kind", ci.Kind),
				slog.String("identity", ci.Identity))
			return nil
		}

		targetAddress := targetMember.Address()
		if targetAddress == localAddress {
			// Local -- send to our placement actor.
			targetPID = il.placementPID
		} else {
			// Remote -- send to target's proxy activator.
			targetPID = actor.NewPID(targetAddress, "$proxy-activator")
		}
	} else {
		// No strategy manager (shouldn't happen for non-client, but be safe).
		targetPID = il.placementPID
	}

	if targetPID == nil {
		slog.Error("natsstream identity: no target PID for activation")
		return nil
	}

	req := &cluster.ActivationRequest{
		ClusterIdentity: ci,
		RequestId:       il.memberID, // used for logging; lock is already held
	}

	future := il.cluster.ActorSystem.Root.RequestFuture(targetPID, req, 10*time.Second)
	res, err := future.Result()
	if err != nil {
		slog.Error("natsstream identity: activation request failed",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity),
			slog.Any("error", err))
		return nil
	}

	resp, ok := res.(*cluster.ActivationResponse)
	if !ok {
		slog.Error("natsstream identity: unexpected response type",
			slog.String("kind", ci.Kind),
			slog.Any("type", res))
		return nil
	}

	if resp.Failed || resp.InvalidIdentity {
		slog.Warn("natsstream identity: activation rejected",
			slog.String("kind", ci.Kind),
			slog.String("identity", ci.Identity),
			slog.Bool("failed", resp.Failed),
			slog.Bool("invalidIdentity", resp.InvalidIdentity))
		return nil
	}

	return resp.Pid
}
```

- [ ] **Step 16: Remove the `spawnActivation` method**

Delete the entire `spawnActivation` method from `natsstream_identity.go`. It is no longer needed since activation goes through the placement actor. Lines 446-493 of the current file.

- [ ] **Step 17: Verify GREEN for stale validation test**

```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityLookup_StaleActivationCleaned -count=1 -timeout 30s ./cluster/clusterproviders/natsstream/
```

- [ ] **Step 18: Verify GREEN for coalescing test**

```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityLookup_CoalesceConcurrentGets -count=1 -timeout 30s ./cluster/clusterproviders/natsstream/
```

- [ ] **Step 19: Verify GREEN for different identities test**

```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityLookup_DifferentIdentitiesNotCoalesced -count=1 -timeout 30s ./cluster/clusterproviders/natsstream/
```

- [ ] **Step 20: Verify GREEN for coalesce failure test**

```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityLookup_CoalesceFirstFailAllGetNil -count=1 -timeout 30s ./cluster/clusterproviders/natsstream/
```

- [ ] **Step 21: Commit**

---

## Chunk 4: Refactor Shutdown() to Stop Placement Actor and Proxy First

### Task 4: Implement the new Shutdown()

**Files:**
- Modify: `cluster/clusterproviders/natsstream/natsstream_identity.go`
- Modify: `cluster/clusterproviders/natsstream/natsstream_identity_test.go`

- [ ] **Step 22: Write the failing test -- Shutdown stops placement actor before removing member**

```go
func TestIdentityLookup_ShutdownStopsPlacementFirst(t *testing.T) {
	srv := startEmbeddedNATS(t)
	kindProps := actor.PropsFromFunc(func(ctx actor.Context) {})
	_, c, il := setupClusterWithTopology(t, srv, "test-shutdown-order",
		[]*cluster.Kind{cluster.NewKind("TestKind", kindProps)})

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "shutdown-1"}

	// Activate an actor.
	pid := il.Get(ci)
	require.NotNil(t, pid, "should get a PID")

	// Capture the placement PID before shutdown.
	placementPID := il.placementPID
	require.NotNil(t, placementPID)

	// Shutdown the lookup. This should stop placement/proxy first.
	il.Shutdown()

	// The placement actor should be stopped after Shutdown.
	// Sending a request should timeout or return DeadLetterResponse.
	req := &cluster.ActivationRequest{
		ClusterIdentity: &cluster.ClusterIdentity{Kind: "TestKind", Identity: "post-shutdown"},
		RequestId:       "post-shutdown-req",
	}
	future := c.ActorSystem.Root.RequestFuture(placementPID, req, 1*time.Second)
	_, err := future.Result()
	assert.Error(t, err, "placement actor should be stopped after Shutdown")
}
```

**Run:**
```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityLookup_ShutdownStopsPlacementFirst -count=1 -timeout 30s ./cluster/clusterproviders/natsstream/
```

**Expected:** RED -- current `Shutdown()` only calls `removeMemberID`, doesn't stop the actors.

- [ ] **Step 23: Verify RED**

- [ ] **Step 24: Write the test -- Shutdown cleans up strategy manager**

```go
func TestIdentityLookup_ShutdownCleansStrategy(t *testing.T) {
	srv := startEmbeddedNATS(t)
	kindProps := actor.PropsFromFunc(func(ctx actor.Context) {})
	_, _, il := setupClusterWithTopology(t, srv, "test-shutdown-strategy",
		[]*cluster.Kind{cluster.NewKind("TestKind", kindProps)})

	require.NotNil(t, il.strategyMgr, "strategy manager should be set before shutdown")

	il.Shutdown()

	assert.Nil(t, il.strategyMgr, "strategy manager should be nil after shutdown")
	assert.Nil(t, il.placementPID, "placement PID should be nil after shutdown")
	assert.Nil(t, il.proxyPID, "proxy PID should be nil after shutdown")
}
```

- [ ] **Step 25: Implement the new Shutdown()**

Replace the existing `Shutdown` method:

```go
// Shutdown performs cleanup when the cluster is shutting down.
// It stops the placement actor (which gracefully poisons all local grains),
// then the proxy activator, then closes strategies, and finally removes
// member records from storage.
func (il *IdentityLookup) Shutdown() {
	// Stop placement actor first -- this triggers graceful shutdown of all
	// locally tracked grains (poisons them with DeactivationReasonShutdown).
	if il.placementPID != nil {
		if err := il.cluster.ActorSystem.Root.PoisonFuture(il.placementPID).Wait(); err != nil {
			slog.Error("natsstream identity: failed to stop placement actor",
				slog.Any("error", err))
		}
		il.placementPID = nil
	}

	// Stop proxy activator.
	if il.proxyPID != nil {
		if err := il.cluster.ActorSystem.Root.PoisonFuture(il.proxyPID).Wait(); err != nil {
			slog.Error("natsstream identity: failed to stop proxy activator",
				slog.Any("error", err))
		}
		il.proxyPID = nil
	}

	// Close strategy manager.
	if il.strategyMgr != nil {
		il.strategyMgr.Close()
		il.strategyMgr = nil
	}

	// Remove all activations belonging to this member from storage.
	if il.memberID != "" {
		il.removeMemberID(context.Background(), il.memberID)
	}
}
```

- [ ] **Step 26: Verify GREEN for shutdown tests**

```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run "TestIdentityLookup_ShutdownStopsPlacementFirst|TestIdentityLookup_ShutdownCleansStrategy" -count=1 -timeout 30s ./cluster/clusterproviders/natsstream/
```

- [ ] **Step 27: Commit**

---

## Chunk 5: Remove ErrNameExists Recovery and Update Existing Tests

### Task 5: Remove ErrNameExists recovery (now handled by placement actor)

**Files:**
- Modify: `cluster/clusterproviders/natsstream/natsstream_identity.go` (already done -- `spawnActivation` was removed in Step 16)
- Modify: `cluster/clusterproviders/natsstream/natsstream_identity_test.go`

- [ ] **Step 28: Remove or update the `TestSpawnActivation_ErrNameExists_ReRegisters` test**

Since `spawnActivation` is removed, this test is no longer applicable. The placement actor prevents duplicate spawns via its in-flight tracking set, so the `ErrNameExists` scenario cannot occur. Remove the test:

Delete `TestSpawnActivation_ErrNameExists_ReRegisters` from the test file.

- [ ] **Step 29: Verify compilation**

```bash
cd /home/cchamplin/development/protoactor-go && go build ./cluster/clusterproviders/natsstream/
```

- [ ] **Step 30: Commit**

---

## Chunk 6: Update Existing Tests to Work with New Architecture

### Task 6: Update existing tests that called internal methods

**Files:**
- Modify: `cluster/clusterproviders/natsstream/natsstream_identity_test.go`

- [ ] **Step 31: Update TestIdentityLookup_TryAcquireLock**

This test directly calls `il.tryAcquireLock` which still exists. No changes needed, but verify it still passes:

```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityLookup_TryAcquireLock -count=1 -timeout 30s ./cluster/clusterproviders/natsstream/
```

- [ ] **Step 32: Update TestIdentityLookup_StoreActivation**

This test directly calls `il.storeActivation` which still exists. No changes needed:

```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityLookup_StoreActivation -count=1 -timeout 30s ./cluster/clusterproviders/natsstream/
```

- [ ] **Step 33: Update TestIdentityLookup_RemoveMember_PurgesActivations**

This test directly calls `il.removeMemberID`. The `removeMemberID` method is unchanged. No changes needed:

```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityLookup_RemoveMember_PurgesActivations -count=1 -timeout 30s ./cluster/clusterproviders/natsstream/
```

- [ ] **Step 34: Update RemovePid tests**

The `RemovePid` method is unchanged. These tests should still pass:

```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run "TestRemovePid_" -count=1 -timeout 30s ./cluster/clusterproviders/natsstream/
```

- [ ] **Step 35: Update TestRemoveMemberID_ScansStreamForRemoteMember**

This test directly stores activations in the stream and calls `removeMemberID`. No changes needed:

```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestRemoveMemberID_ScansStreamForRemoteMember -count=1 -timeout 30s ./cluster/clusterproviders/natsstream/
```

- [ ] **Step 36: Commit**

---

## Chunk 7: End-to-End Flow Tests

### Task 7: Full Get -> placement actor -> spawn -> persist -> return PID flow

**Files:**
- Modify: `cluster/clusterproviders/natsstream/natsstream_identity_test.go`

- [ ] **Step 37: Write the test -- full end-to-end activation flow**

```go
func TestIdentityLookup_EndToEnd_ActivateAndRetrieve(t *testing.T) {
	srv := startEmbeddedNATS(t)
	kindProps := actor.PropsFromFunc(func(ctx actor.Context) {})
	_, c, il := setupClusterWithTopology(t, srv, "test-e2e-activate",
		[]*cluster.Kind{cluster.NewKind("TestKind", kindProps)})

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "order-123"}
	ctx := context.Background()

	// First Get -- should activate via placement actor.
	pid1 := il.Get(ci)
	require.NotNil(t, pid1, "first Get should return PID")

	// Verify the activation is stored in the stream.
	rec := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec, "activation should be persisted in stream")
	assert.Equal(t, c.ActorSystem.ID, rec.MemberID, "member ID should be ours")
	assert.Equal(t, pid1.Id, rec.PidID)
	assert.Equal(t, pid1.Address, rec.PidAddress)

	// Verify member tracking.
	il.memberKeysMu.Lock()
	keys := il.memberKeys[il.memberID]
	il.memberKeysMu.Unlock()
	assert.NotEmpty(t, keys, "member should have tracked keys")

	// Second Get -- should find existing activation (no new spawn).
	pid2 := il.Get(ci)
	require.NotNil(t, pid2, "second Get should return PID")
	assert.True(t, pid1.Equal(pid2), "second Get should return same PID")
}
```

**Run:**
```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityLookup_EndToEnd_ActivateAndRetrieve -count=1 -timeout 30s ./cluster/clusterproviders/natsstream/
```

**Expected:** GREEN -- the full flow works: Get -> lock -> strategy -> placement actor -> spawn -> persist callback -> return PID.

- [ ] **Step 38: Verify GREEN**

- [ ] **Step 39: Write the test -- strategy selects local, no remote hop**

```go
func TestIdentityLookup_StrategySelectsLocal(t *testing.T) {
	srv := startEmbeddedNATS(t)
	kindProps := actor.PropsFromFunc(func(ctx actor.Context) {})
	_, c, il := setupClusterWithTopology(t, srv, "test-strategy-local",
		[]*cluster.Kind{cluster.NewKind("TestKind", kindProps)})

	require.NotNil(t, il.strategyMgr, "strategy manager should be set")

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "strategy-local-1"}

	// With only one member (ourselves), strategy must select local.
	pid := il.Get(ci)
	require.NotNil(t, pid, "should activate successfully")

	// The PID should be on the local address.
	localAddr := c.ActorSystem.Address()
	assert.Equal(t, localAddr, pid.Address,
		"activation should be on local member")
}
```

**Run:**
```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityLookup_StrategySelectsLocal -count=1 -timeout 30s ./cluster/clusterproviders/natsstream/
```

**Expected:** GREEN.

- [ ] **Step 40: Verify GREEN**

- [ ] **Step 41: Commit**

---

## Chunk 8: Graceful Shutdown Verifies Actor Poisoning

### Task 8: Verify graceful shutdown poisons grains

**Files:**
- Modify: `cluster/clusterproviders/natsstream/natsstream_identity_test.go`

- [ ] **Step 42: Write the test -- shutdown poisons locally spawned grains**

```go
func TestIdentityLookup_ShutdownPoisonsLocalGrains(t *testing.T) {
	srv := startEmbeddedNATS(t)

	stopped := make(chan string, 10)
	kindProps := actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *actor.Stopping:
			// Extract identity from the PID name.
			stopped <- ctx.Self().Id
		}
	})
	_, c, il := setupClusterWithTopology(t, srv, "test-shutdown-poison",
		[]*cluster.Kind{cluster.NewKind("TestKind", kindProps)})

	// Activate two grains.
	ci1 := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "grain-a"}
	ci2 := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "grain-b"}

	pid1 := il.Get(ci1)
	pid2 := il.Get(ci2)
	require.NotNil(t, pid1)
	require.NotNil(t, pid2)

	// Verify both are alive.
	_, exists1 := c.ActorSystem.ProcessRegistry.GetLocal(pid1.Id)
	_, exists2 := c.ActorSystem.ProcessRegistry.GetLocal(pid2.Id)
	require.True(t, exists1, "grain-a should be alive")
	require.True(t, exists2, "grain-b should be alive")

	// Shutdown -- should poison both grains via placement actor.
	il.Shutdown()

	// Wait for both stop signals.
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	stoppedIDs := make(map[string]bool)
	for len(stoppedIDs) < 2 {
		select {
		case id := <-stopped:
			stoppedIDs[id] = true
		case <-timer.C:
			t.Fatalf("timed out waiting for grains to stop; got %d of 2", len(stoppedIDs))
		}
	}

	assert.True(t, stoppedIDs["TestKind/grain-a"], "grain-a should have been stopped")
	assert.True(t, stoppedIDs["TestKind/grain-b"], "grain-b should have been stopped")
}
```

**Run:**
```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run TestIdentityLookup_ShutdownPoisonsLocalGrains -count=1 -timeout 30s ./cluster/clusterproviders/natsstream/
```

**Expected:** GREEN -- the placement actor's `onStopping` handler poisons all tracked grains.

- [ ] **Step 43: Verify GREEN**

- [ ] **Step 44: Commit**

---

## Chunk 9: GrainEnumerator Compatibility

### Task 9: Verify GrainEnumerator still works with placement-mediated activations

**Files:**
- Modify: `cluster/clusterproviders/natsstream/natsstream_identity_test.go`

- [ ] **Step 45: Write the test -- ListGrains after placement-mediated activation**

```go
func TestIdentityLookup_ListGrainsAfterPlacementActivation(t *testing.T) {
	srv := startEmbeddedNATS(t)
	kindProps := actor.PropsFromFunc(func(ctx actor.Context) {})
	_, _, il := setupClusterWithTopology(t, srv, "test-list-grains",
		[]*cluster.Kind{cluster.NewKind("TestKind", kindProps)})

	// Activate two grains.
	ci1 := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "list-a"}
	ci2 := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "list-b"}
	pid1 := il.Get(ci1)
	pid2 := il.Get(ci2)
	require.NotNil(t, pid1)
	require.NotNil(t, pid2)

	// ListGrains should return both.
	grains, err := il.ListGrains()
	require.NoError(t, err)
	assert.Len(t, grains, 2, "should list 2 grains")

	// Verify grain details.
	grainMap := make(map[string]*cluster.GrainInfo)
	for _, g := range grains {
		grainMap[g.Identity] = g
	}

	ga := grainMap["list-a"]
	require.NotNil(t, ga, "grain list-a should be in results")
	assert.Equal(t, "TestKind", ga.Kind)
	assert.Equal(t, pid1.Id, ga.PID.Id)

	gb := grainMap["list-b"]
	require.NotNil(t, gb, "grain list-b should be in results")
	assert.Equal(t, "TestKind", gb.Kind)
	assert.Equal(t, pid2.Id, gb.PID.Id)
}

func TestIdentityLookup_ListGrainsByKindAfterPlacement(t *testing.T) {
	srv := startEmbeddedNATS(t)
	kindPropsA := actor.PropsFromFunc(func(ctx actor.Context) {})
	kindPropsB := actor.PropsFromFunc(func(ctx actor.Context) {})
	_, _, il := setupClusterWithTopology(t, srv, "test-list-by-kind",
		[]*cluster.Kind{
			cluster.NewKind("KindA", kindPropsA),
			cluster.NewKind("KindB", kindPropsB),
		})

	// Activate one grain of each kind.
	ciA := &cluster.ClusterIdentity{Kind: "KindA", Identity: "grain-a"}
	ciB := &cluster.ClusterIdentity{Kind: "KindB", Identity: "grain-b"}
	pidA := il.Get(ciA)
	pidB := il.Get(ciB)
	require.NotNil(t, pidA)
	require.NotNil(t, pidB)

	// Filter by KindA.
	grainsA, err := il.ListGrainsByKind("KindA")
	require.NoError(t, err)
	assert.Len(t, grainsA, 1, "should have 1 KindA grain")
	assert.Equal(t, "grain-a", grainsA[0].Identity)

	// Filter by KindB.
	grainsB, err := il.ListGrainsByKind("KindB")
	require.NoError(t, err)
	assert.Len(t, grainsB, 1, "should have 1 KindB grain")
	assert.Equal(t, "grain-b", grainsB[0].Identity)
}
```

**Run:**
```bash
cd /home/cchamplin/development/protoactor-go && go test -race -run "TestIdentityLookup_ListGrains" -count=1 -timeout 30s ./cluster/clusterproviders/natsstream/
```

**Expected:** GREEN -- the `PersistActivation` callback calls `addKeyToMember`, which populates the `memberKeys` map used by `ListGrains`.

- [ ] **Step 46: Verify GREEN**

- [ ] **Step 47: Commit**

---

## Chunk 10: Run Full Test Suite and Fix Any Regressions

### Task 10: Run all natsstream tests

- [ ] **Step 48: Run all natsstream unit tests**

```bash
cd /home/cchamplin/development/protoactor-go && go test -race -count=1 -timeout 60s ./cluster/clusterproviders/natsstream/
```

**Expected:** GREEN -- all existing tests pass, all new tests pass.

- [ ] **Step 49: Run full cluster package tests**

```bash
cd /home/cchamplin/development/protoactor-go && go test -race -count=1 -timeout 120s ./cluster/...
```

**Expected:** GREEN -- placement actor tests (from SP1c), storage lookup tests (from SP2), and all other cluster tests pass.

- [ ] **Step 50: Run natsstream integration tests (if Docker available)**

```bash
cd /home/cchamplin/development/protoactor-go && go test -race -tags integration -count=1 -timeout 120s ./cluster/clusterproviders/natsstream/
```

**Expected:** GREEN -- integration tests use the provider directly and should still work.

- [ ] **Step 51: Final commit with all changes**

---

## Summary of Changes

| File | Change |
|------|--------|
| `cluster/clusterproviders/natsstream/natsstream_identity.go` | Add `inflight` struct, `placementPID`/`proxyPID`/`strategyMgr`/`inflights` fields; refactor `Setup()` to spawn placement actor + proxy with natsstream persistence callbacks (stream publish for persist, stream purge for remove); refactor `Get()` with stale member validation, coalescing, strategy selection, placement actor routing via `activateViaPlacement()`; refactor `Shutdown()` to stop actors first, close strategies, then remove member; remove `spawnActivation()` |
| `cluster/clusterproviders/natsstream/natsstream_identity_test.go` | Remove `TestSpawnActivation_ErrNameExists_ReRegisters`; add 12 new tests: `SetupSpawnsPlacementAndProxy`, `ClientSetupSkipsPlacementActor`, `StaleActivationCleaned`, `DifferentIdentitiesNotCoalesced`, `CoalesceFirstFailAllGetNil`, `CoalesceConcurrentGets`, `ShutdownStopsPlacementFirst`, `ShutdownCleansStrategy`, `EndToEnd_ActivateAndRetrieve`, `StrategySelectsLocal`, `ShutdownPoisonsLocalGrains`, `ListGrainsAfterPlacementActivation`, `ListGrainsByKindAfterPlacement` |
| `cluster/clusterproviders/natsstream/testhelpers_test.go` | Add `setupClusterWithTopology` helper and `kindNames` utility |

## Key Design Decisions

1. **PersistActivation bypasses CAS sequence check**: The placement actor's `PersistActivation` callback publishes the activation record directly (no `WithExpectLastSequencePerSubject`). This is safe because the placement actor serializes all `ActivationRequest`s for the same identity via its in-flight set, and the lock is already held from the `Get()` path. The CAS check was only needed when `spawnActivation` could race with concurrent calls -- the placement actor eliminates that race.

2. **RemoveActivation uses stream purge**: The callback purges the identity subject from the stream and removes it from the in-memory member tracking map, matching the existing `removeKeyFromMember` pattern.

3. **Lock acquisition still used in Get()**: Even though the placement actor handles spawn serialization, we still acquire the NATS stream lock in `Get()` before sending the `ActivationRequest`. This provides distributed coordination across nodes -- without it, multiple nodes could simultaneously send `ActivationRequest` to the same placement actor, and the actor would reject all but the first. The lock ensures only one node enters the activation path.

4. **No client activation handler changes**: Unlike natskv, natsstream doesn't have a `handleActivationRequest` NATS subscription for clients. Client mode still uses `waitForActivation` (ordered consumer watching the stream). No changes needed here.

5. **Strategy manager subscribes to topology events**: The identity lookup subscribes to `ClusterTopology` events and calls `AddMember`/`RemoveMember` on the strategy manager. This is identical to the SP2 pattern.

6. **Coalescing includes panic recovery**: The inflight `defer` includes a `recover()` to ensure the `done` channel is always closed, even if the first caller panics. This prevents permanent blocking of coalesced waiters.
