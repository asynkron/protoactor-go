# RemovePid Liveness Fix Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the race where `RemovePid` deletes a KV/stream identity record for an actor that is still alive locally, causing orphaned state and resolution failures.

**Architecture:** Three-layer defense: (1) `DefaultContext.Request()` only calls `RemovePid` on dead letter, not timeout; (2) natskv and natsstream `RemovePid` skip deletion when the actor is alive in the local process registry; (3) keep `ErrNameExists` recovery in `spawnActivation` as belt-and-suspenders. Also backfill tests for two untested commits (`d0ed38a6`, `4b0a3586`).

**Tech Stack:** Go, NATS embedded server (unit tests), testcontainers (integration tests), testify

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `cluster/default_context.go` | Modify (lines 139-158) | Split timeout vs dead-letter error handling |
| `cluster/default_context_removepid_test.go` | Modify | Add timeout-does-not-call-RemovePid test |
| `cluster/clusterproviders/natskv/natskv_identity.go` | Modify (RemovePid, lines 206-247) | Add local liveness guard |
| `cluster/clusterproviders/natskv/natskv_identity_test.go` | Modify | Add RemovePid liveness tests, ErrNameExists test |
| `cluster/clusterproviders/natskv/natskv_identity_integration_test.go` | Create | Integration test: slow actor identity preserved |
| `cluster/clusterproviders/natsstream/natsstream_identity.go` | Modify (RemovePid, lines 149-175) | Add local liveness guard |
| `cluster/clusterproviders/natsstream/natsstream_identity_test.go` | Modify | Add RemovePid liveness tests, ErrNameExists test, purgeActivationsForMember test |
| `cluster/clusterproviders/natsstream/testhelpers_test.go` | Modify | Add setupClusterWithKindsEmbedded helper for unit tests needing kinds |

---

## Chunk 1: DefaultContext — Split Timeout vs Dead Letter

### Task 1: Add failing test — timeout must not call RemovePid

**Files:**
- Modify: `cluster/default_context_removepid_test.go`

- [ ] **Step 1: Write the failing test**

Add this test to `cluster/default_context_removepid_test.go`. It creates a slow actor that takes longer to respond than the request timeout, verifies the request fails with timeout, and asserts that `RemovePid` was NOT called.

```go
// TestDefaultContext_Request_Timeout_DoesNotCallRemovePid verifies that
// DefaultContext.Request() does NOT call IdentityLookup.RemovePid() when a
// request times out. Timeouts mean the actor may still be alive (just slow),
// so deleting its identity record would create an orphaned actor with no
// KV/stream record, breaking future lookups.
func TestDefaultContext_Request_Timeout_DoesNotCallRemovePid(t *testing.T) {
	system := actor.NewActorSystem()

	lookup := &trackingIdentityLookup{}
	cp := newInmemoryProvider()

	cfg := Configure("test-timeout-no-removepid", cp, lookup,
		remote.Configure("127.0.0.1", 0),
		WithKinds(NewKind("slow", actor.PropsFromFunc(func(ctx actor.Context) {
			if _, ok := ctx.Message().(*PingMessage); ok {
				// Deliberately slow — will exceed the request timeout.
				time.Sleep(1 * time.Second)
				ctx.Respond(&PingMessage{})
			}
		}))),
	)

	c := New(system, cfg)
	c.Remote = remote.NewRemote(system, c.Config.RemoteConfig)

	err := c.StartMember()
	require.NoError(t, err)
	t.Cleanup(func() { c.Shutdown(true) })

	// Spawn a slow actor and register it in the identity lookup.
	kind := c.GetClusterKind("slow")
	require.NotNil(t, kind)
	ci := NewClusterIdentity("slow-grain-1", "slow")
	props := WithClusterIdentity(kind.Props, ci)
	pid, err := system.Root.SpawnNamed(props, "slow-grain-1")
	require.NoError(t, err)
	t.Cleanup(func() { system.Root.Poison(pid) })

	lookup.m.Store("slow-grain-1", pid)

	// Request with a short timeout — will time out because actor sleeps 1s.
	_, _ = c.Request("slow-grain-1", "slow", &PingMessage{},
		WithTimeout(200*time.Millisecond),
		WithRetryCount(1),
	)

	// RemovePid must NOT have been called — the actor is alive, just slow.
	calls := lookup.getRemovePidCalls()
	assert.Empty(t, calls,
		"RemovePid must NOT be called on timeout — the actor may still "+
			"be alive. Only dead letters should trigger RemovePid.")
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run TestDefaultContext_Request_Timeout_DoesNotCallRemovePid -v -count=1 ./cluster/`

Expected: FAIL — the current code calls `RemovePid` on both timeout AND dead letter.

- [ ] **Step 3: Fix DefaultContext.Request() to only call RemovePid on dead letter**

Modify `cluster/default_context.go` lines 139-158. Split the error handling so that timeouts only clear the PID cache, while dead letters also call RemovePid:

```go
			if err != nil {
				dcc.cluster.Logger().Error("cluster.RequestFuture failed", slog.Any("error", err), slog.Any("pid", pid))

				isDeadLetter := errors.Is(err, actor.ErrDeadLetter) || errors.Is(err, remote.ErrDeadLetter)
				isTimeout := errors.Is(err, actor.ErrTimeout) || errors.Is(err, remote.ErrTimeout)

				if isDeadLetter || isTimeout {
					counter = callConfig.RetryAction(counter)
					dcc.cluster.PidCache.Remove(identity, kind)

					// Only call RemovePid on dead letter — the actor process
					// is confirmed gone. Timeouts may indicate a slow-but-alive
					// actor; deleting its identity record would orphan the
					// process (alive locally but unresolvable via identity lookup).
					if isDeadLetter {
						dcc.cluster.IdentityLookup.RemovePid(
							NewClusterIdentity(identity, kind), pid,
						)
					}

					if dcc.cluster.metricsEnabled {
						_ctx := context.Background()
						attrs := append(
							actor.SystemLabels(dcc.cluster.ActorSystem),
							attribute.String("clusterkind", kind),
							attribute.String("messagetype", actor.MessageName(message)),
						)
						dcc.cluster.metrics.ClusterRequestRetryCount.Add(_ctx, 1, metric.WithAttributes(attrs...))
					}
					continue
				}
				break selectloop
			}
```

- [ ] **Step 4: Run all DefaultContext tests to verify green**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run 'TestDefaultContext_Request' -v -count=1 -race ./cluster/`

Expected: All three tests pass — the new timeout test passes (no RemovePid called), and the existing dead-letter tests still pass (RemovePid IS called on dead letter).

- [ ] **Step 5: Commit**

```bash
git add cluster/default_context.go cluster/default_context_removepid_test.go
git commit -m "fix(cluster): only call RemovePid on dead letter, not timeout

Timeouts indicate a slow-but-alive actor. Calling RemovePid on timeout
deletes the identity record while the actor process is still running,
creating an orphaned state where the actor is alive locally but
unresolvable via identity lookup. Subsequent Get() calls fail with
ErrNameExists when trying to re-spawn.

Dead letters confirm the actor process is gone, making RemovePid safe."
```

---

## Chunk 2: natskv — RemovePid Liveness Guard + Backfill Tests

### Task 2: Add failing tests for natskv RemovePid liveness guard

**Files:**
- Modify: `cluster/clusterproviders/natskv/natskv_identity_test.go`

- [ ] **Step 1: Write three failing tests**

Add these tests to `cluster/clusterproviders/natskv/natskv_identity_test.go`:

```go
// TestRemovePid_SkipsDeleteWhenActorAliveLocally verifies that RemovePid
// does NOT delete the KV record when the actor is still running in the
// local process registry. This prevents the orphaned-actor bug where a
// timeout triggers RemovePid but the actor is just slow, not dead.
func TestRemovePid_SkipsDeleteWhenActorAliveLocally(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	ctx := context.Background()
	identities, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_liveness_skip_identities",
	})
	require.NoError(t, err)

	tracking, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_liveness_skip_tracking",
	})
	require.NoError(t, err)

	// Create a real actor system so we have a process registry.
	system := actor.NewActorSystem()

	// Spawn a real actor that stays alive.
	pid, err := system.Root.SpawnNamed(actor.PropsFromFunc(func(ctx actor.Context) {}), "TestKind/grain-alive")
	require.NoError(t, err)
	t.Cleanup(func() { system.Root.Poison(pid) })

	il := &IdentityLookup{
		identities:    identities,
		memberTracker: tracking,
		config:        newDefaultConfig(),
		semaphore:     make(chan struct{}, 200),
		memberID:      "member-local",
		cluster:       &cluster.Cluster{ActorSystem: system},
	}

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "grain-alive"}

	// Store an activation record pointing to the local actor.
	lockID, rev, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok)
	err = il.storeActivation(ctx, ci, lockID, rev, "member-local", pid.Address, pid.Id)
	require.NoError(t, err)

	// Call RemovePid — should be a no-op because actor is alive locally.
	il.RemovePid(ci, pid)

	// KV record must still exist.
	rec := il.getExistingActivation(ctx, ci)
	assert.NotNil(t, rec,
		"RemovePid must NOT delete the KV record when the actor is alive locally")
	assert.Equal(t, pid.Id, rec.PidID)
}

// TestRemovePid_DeletesWhenActorNotAliveLocally verifies that RemovePid
// DOES delete the KV record when the PID points to a local address but
// the actor is no longer in the process registry (it was stopped/crashed).
func TestRemovePid_DeletesWhenActorNotAliveLocally(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	ctx := context.Background()
	identities, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_liveness_dead_identities",
	})
	require.NoError(t, err)

	tracking, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_liveness_dead_tracking",
	})
	require.NoError(t, err)

	system := actor.NewActorSystem()

	// Spawn and immediately stop the actor so it's in the registry briefly
	// then gone.
	pid, err := system.Root.SpawnNamed(actor.PropsFromFunc(func(ctx actor.Context) {}), "TestKind/grain-dead")
	require.NoError(t, err)
	system.Root.Poison(pid)
	time.Sleep(200 * time.Millisecond) // wait for actor to fully stop

	il := &IdentityLookup{
		identities:    identities,
		memberTracker: tracking,
		config:        newDefaultConfig(),
		semaphore:     make(chan struct{}, 200),
		memberID:      "member-local",
		cluster:       &cluster.Cluster{ActorSystem: system},
	}

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "grain-dead"}

	// Plant activation record pointing to the dead local actor.
	rec := activationRecord{
		PidID:      pid.Id,
		PidAddress: pid.Address,
		MemberID:   "member-local",
	}
	data, err := json.Marshal(&rec)
	require.NoError(t, err)
	_, err = identities.Put(ctx, kvKey(ci), data)
	require.NoError(t, err)

	// RemovePid should succeed — actor is dead.
	il.RemovePid(ci, pid)

	result := il.getExistingActivation(ctx, ci)
	assert.Nil(t, result,
		"RemovePid must delete the KV record when the local actor is dead")
}

// TestRemovePid_DeletesWhenActorRemote verifies that RemovePid deletes the
// KV record when the PID points to a remote address. We can't check remote
// liveness, so we trust the caller (DefaultContext confirmed dead letter).
func TestRemovePid_DeletesWhenActorRemote(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	ctx := context.Background()
	identities, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_liveness_remote_identities",
	})
	require.NoError(t, err)

	tracking, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_liveness_remote_tracking",
	})
	require.NoError(t, err)

	system := actor.NewActorSystem()

	il := &IdentityLookup{
		identities:    identities,
		memberTracker: tracking,
		config:        newDefaultConfig(),
		semaphore:     make(chan struct{}, 200),
		memberID:      "member-local",
		cluster:       &cluster.Cluster{ActorSystem: system},
	}

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "grain-remote"}
	remotePid := actor.NewPID("remote-host:9999", "TestKind/grain-remote")

	// Plant activation record pointing to a remote host.
	rec := activationRecord{
		PidID:      remotePid.Id,
		PidAddress: remotePid.Address,
		MemberID:   "member-remote",
	}
	data, err := json.Marshal(&rec)
	require.NoError(t, err)
	_, err = identities.Put(ctx, kvKey(ci), data)
	require.NoError(t, err)

	// RemovePid should delete — we can't verify remote liveness.
	il.RemovePid(ci, remotePid)

	result := il.getExistingActivation(ctx, ci)
	assert.Nil(t, result,
		"RemovePid must delete the KV record for remote actors (can't check liveness)")
}
```

- [ ] **Step 2: Run the tests to verify two fail, one passes**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run 'TestRemovePid_(SkipsDelete|DeletesWhenActor)' -v -count=1 ./cluster/clusterproviders/natskv/`

Expected:
- `TestRemovePid_SkipsDeleteWhenActorAliveLocally` — FAIL (current code deletes regardless of liveness)
- `TestRemovePid_DeletesWhenActorNotAliveLocally` — PASS (current code always deletes)
- `TestRemovePid_DeletesWhenActorRemote` — PASS (current code always deletes)

### Task 3: Implement natskv RemovePid liveness guard

**Files:**
- Modify: `cluster/clusterproviders/natskv/natskv_identity.go` (lines 206-247)

- [ ] **Step 3: Add liveness guard to RemovePid**

Replace the `RemovePid` method. Add the liveness check at the top, before any KV operations:

```go
func (il *IdentityLookup) RemovePid(ci *cluster.ClusterIdentity, pid *actor.PID) {
	if il.setupErr != nil {
		slog.Error("natskv identity: cannot RemovePid, setup failed", slog.Any("error", il.setupErr))
		return
	}

	// If the actor is running locally, do NOT delete its identity record.
	// This prevents the orphan bug: a timeout triggers RemovePid, but the
	// actor is alive (just slow). Deleting the KV record would leave the
	// process alive but unresolvable.
	if il.cluster != nil && pid.Address == il.cluster.ActorSystem.Address() {
		_, exists := il.cluster.ActorSystem.ProcessRegistry.GetLocal(pid.Id)
		if exists {
			slog.Debug("natskv identity: RemovePid skipped, actor is alive locally",
				slog.String("kind", ci.Kind),
				slog.String("identity", ci.Identity),
				slog.String("pid", pid.String()))
			return
		}
	}

	ctx := context.Background()
	key := kvKey(ci)

	// Read the entry to validate the PID before deleting.
	entry, err := il.identities.Get(ctx, key)
	if err != nil {
		return // Not found, nothing to remove.
	}

	var rec activationRecord
	if err := json.Unmarshal(entry.Value(), &rec); err != nil {
		return
	}

	// Only delete if the stored PID matches the one being removed.
	if rec.PidID != pid.Id || rec.PidAddress != pid.Address {
		return
	}

	if rec.MemberID != "" {
		il.removeKeyFromMember(ctx, rec.MemberID, key)
	}

	if err := il.identities.Delete(ctx, key, jetstream.LastRevision(entry.Revision())); err != nil {
		if !errors.Is(err, jetstream.ErrKeyNotFound) && !errors.Is(err, jetstream.ErrKeyExists) {
			slog.Error("natskv identity: RemovePid delete failed",
				slog.String("key", key), slog.Any("error", err))
		}
	}
}
```

- [ ] **Step 4: Run the liveness tests to verify green**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run 'TestRemovePid_(SkipsDelete|DeletesWhenActor)' -v -count=1 ./cluster/clusterproviders/natskv/`

Expected: All three PASS.

- [ ] **Step 5: Run all natskv unit tests to verify no regressions**

Run: `cd /home/cchamplin/development/protoactor-go && go test -v -count=1 -race ./cluster/clusterproviders/natskv/`

Expected: All PASS.

- [ ] **Step 6: Commit**

```bash
git add cluster/clusterproviders/natskv/natskv_identity.go cluster/clusterproviders/natskv/natskv_identity_test.go
git commit -m "fix(natskv): add liveness guard to RemovePid

Skip KV record deletion when the actor is alive in the local process
registry. This prevents the orphan bug where a timeout triggers
RemovePid but the actor is just slow — deleting the KV record would
leave the process alive but unresolvable via identity lookup.

Remote PIDs are still deleted (can't verify remote liveness)."
```

### Task 4: Backfill test for natskv ErrNameExists recovery (commit d0ed38a6)

**Files:**
- Modify: `cluster/clusterproviders/natskv/natskv_identity_test.go`

- [ ] **Step 1: Write a test for ErrNameExists recovery in spawnActivation**

This tests the belt-and-suspenders path: if somehow a KV record is deleted but the actor is still alive, and a new `Get()` call reaches `spawnActivation`, the ErrNameExists path should re-register the existing PID.

This test needs a fully started cluster so `TryGetClusterKind` works. We build a real cluster using the natskv provider with the embedded NATS server, passing the kind via `cluster.WithKinds` at `Configure` time (before `StartMember` calls `initKinds`). We then construct a separate `IdentityLookup` that shares the same KV buckets but is controlled by the test.

```go
// TestSpawnActivation_ErrNameExists_ReRegisters verifies that when
// SpawnNamed returns ErrNameExists (actor already running locally but
// KV record was deleted), spawnActivation re-registers the existing PID
// in the identity store rather than returning nil.
func TestSpawnActivation_ErrNameExists_ReRegisters(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc, js := connectNATS(t, srv)

	ctx := context.Background()
	identities, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_errname_reregister_identities",
	})
	require.NoError(t, err)

	tracking, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_errname_reregister_tracking",
	})
	require.NoError(t, err)

	// Build a real cluster with the natskv provider so TryGetClusterKind works.
	kindProps := actor.PropsFromFunc(func(ctx actor.Context) {})
	provider, err := New(nc)
	require.NoError(t, err)

	system := actor.NewActorSystem()
	remoteConfig := remote.Configure("127.0.0.1", 0)
	clusterConfig := cluster.Configure("test-errname", provider, provider.IdentityLookup(),
		remoteConfig,
		cluster.WithKinds(cluster.NewKind("TestKind", kindProps)),
	)
	c := cluster.NewCluster(system, clusterConfig)
	c.Remote = remote.NewRemote(system, remoteConfig)
	require.NoError(t, c.StartMember())
	t.Cleanup(func() { c.Shutdown(true) })

	// Create a test IdentityLookup pointing at our test KV buckets
	// but using the real cluster (so TryGetClusterKind works).
	il := &IdentityLookup{
		identities:    identities,
		memberTracker: tracking,
		config:        newDefaultConfig(),
		semaphore:     make(chan struct{}, 200),
		memberID:      "member-local",
		cluster:       c,
	}

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "grain-exists"}

	// Pre-spawn the actor so SpawnNamed will return ErrNameExists.
	props := cluster.WithClusterIdentity(kindProps, ci)
	existingPid, err := system.Root.SpawnNamed(props, "TestKind/grain-exists")
	require.NoError(t, err)
	t.Cleanup(func() { system.Root.Poison(existingPid) })

	// Acquire lock (simulates what Get() does before calling spawnActivation).
	lockID, rev, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok)

	// spawnActivation should detect ErrNameExists and re-register.
	pid := il.spawnActivation(ci, lockID, rev)
	require.NotNil(t, pid, "spawnActivation should return the existing PID, not nil")
	assert.Equal(t, existingPid.Id, pid.Id)
	assert.Equal(t, existingPid.Address, pid.Address)

	// The activation should be stored in the KV.
	rec := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec, "activation must be stored in KV after ErrNameExists recovery")
	assert.Equal(t, existingPid.Id, rec.PidID)
}
```

- [ ] **Step 2: Run test to verify it passes (this tests existing behavior)**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run TestSpawnActivation_ErrNameExists_ReRegisters -v -count=1 ./cluster/clusterproviders/natskv/`

Expected: PASS — this validates the existing ErrNameExists fix from commit `d0ed38a6`.

- [ ] **Step 3: Commit**

```bash
git add cluster/clusterproviders/natskv/natskv_identity_test.go
git commit -m "test(natskv): backfill test for ErrNameExists recovery in spawnActivation

Covers the belt-and-suspenders path from commit d0ed38a6 where
SpawnNamed returns ErrNameExists because the actor is alive locally
but the KV record was deleted. Verifies the existing PID is
re-registered in the identity store."
```

---

## Chunk 3: natsstream — RemovePid Liveness Guard + Backfill Tests

### Task 5: Add failing tests for natsstream RemovePid liveness guard

**Files:**
- Modify: `cluster/clusterproviders/natsstream/natsstream_identity_test.go`

- [ ] **Step 1: Write three liveness tests**

Add to `cluster/clusterproviders/natsstream/natsstream_identity_test.go`. These mirror the natskv tests but use the natsstream setup pattern:

```go
// TestRemovePid_SkipsDeleteWhenActorAliveLocally verifies that RemovePid
// does NOT purge the stream record when the actor is still running locally.
func TestRemovePid_SkipsDeleteWhenActorAliveLocally(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-liveness-skip")

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	il := p.IdentityLookup()
	il.Setup(c, []string{"TestKind"}, false)
	time.Sleep(500 * time.Millisecond)

	// Spawn a real actor that stays alive.
	system := c.ActorSystem
	pid, err := system.Root.SpawnNamed(actor.PropsFromFunc(func(ctx actor.Context) {}), "TestKind/grain-alive")
	require.NoError(t, err)
	t.Cleanup(func() { system.Root.Poison(pid) })

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "grain-alive"}
	ctx := context.Background()

	// Store activation pointing to the live local actor.
	lockID, seq, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok)
	require.NoError(t, il.storeActivation(ctx, ci, lockID, seq, il.memberID, pid.Address, pid.Id))

	// RemovePid should be a no-op — actor is alive.
	il.RemovePid(ci, pid)

	rec := il.getExistingActivation(ctx, ci)
	assert.NotNil(t, rec,
		"RemovePid must NOT purge the stream record when the actor is alive locally")
}

// TestRemovePid_DeletesWhenActorNotAliveLocally verifies RemovePid DOES
// purge when the local actor is dead (stopped/crashed).
func TestRemovePid_DeletesWhenActorNotAliveLocally(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-liveness-dead")

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	il := p.IdentityLookup()
	il.Setup(c, []string{"TestKind"}, false)
	time.Sleep(500 * time.Millisecond)

	system := c.ActorSystem

	// Spawn and immediately stop.
	pid, err := system.Root.SpawnNamed(actor.PropsFromFunc(func(ctx actor.Context) {}), "TestKind/grain-dead")
	require.NoError(t, err)
	system.Root.Poison(pid)
	time.Sleep(200 * time.Millisecond)

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "grain-dead"}
	ctx := context.Background()

	// Plant stale activation.
	lockID, seq, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok)
	require.NoError(t, il.storeActivation(ctx, ci, lockID, seq, il.memberID, pid.Address, pid.Id))

	// RemovePid should succeed — actor is dead.
	il.RemovePid(ci, pid)

	rec := il.getExistingActivation(ctx, ci)
	assert.Nil(t, rec, "RemovePid must purge the stream record when local actor is dead")
}

// TestRemovePid_DeletesWhenActorRemote verifies RemovePid purges the
// stream record for remote PIDs (can't check remote liveness).
func TestRemovePid_DeletesWhenActorRemote(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-liveness-remote")

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	il := p.IdentityLookup()
	il.Setup(c, []string{"TestKind"}, false)
	time.Sleep(500 * time.Millisecond)

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "grain-remote"}
	ctx := context.Background()
	remotePid := actor.NewPID("remote-host:9999", "TestKind/grain-remote")

	// Plant activation for remote PID.
	lockID, seq, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok)
	require.NoError(t, il.storeActivation(ctx, ci, lockID, seq, "member-remote", remotePid.Address, remotePid.Id))

	// RemovePid should purge — can't verify remote liveness.
	il.RemovePid(ci, remotePid)

	rec := il.getExistingActivation(ctx, ci)
	assert.Nil(t, rec, "RemovePid must purge stream record for remote actors")
}
```

- [ ] **Step 2: Run the tests to verify the skip test fails**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run 'TestRemovePid_(SkipsDelete|DeletesWhenActor)' -v -count=1 ./cluster/clusterproviders/natsstream/`

Expected: `SkipsDeleteWhenActorAliveLocally` FAILS, other two PASS.

### Task 6: Implement natsstream RemovePid liveness guard

**Files:**
- Modify: `cluster/clusterproviders/natsstream/natsstream_identity.go` (lines 149-175)

- [ ] **Step 3: Add liveness guard to natsstream RemovePid**

Replace `RemovePid` method:

```go
func (il *IdentityLookup) RemovePid(ci *cluster.ClusterIdentity, pid *actor.PID) {
	// If the actor is running locally, do NOT delete its identity record.
	if il.cluster != nil && pid.Address == il.cluster.ActorSystem.Address() {
		_, exists := il.cluster.ActorSystem.ProcessRegistry.GetLocal(pid.Id)
		if exists {
			slog.Debug("natsstream identity: RemovePid skipped, actor is alive locally",
				slog.String("kind", ci.Kind),
				slog.String("identity", ci.Identity),
				slog.String("pid", pid.String()))
			return
		}
	}

	ctx := context.Background()
	subject := il.identitySubject(ci)

	rec := il.getExistingActivation(ctx, ci)
	if rec == nil {
		return
	}

	if rec.PidID != pid.Id || rec.PidAddress != pid.Address {
		return
	}

	if rec.MemberID != "" {
		il.removeKeyFromMember(rec.MemberID, subject)
	}

	if err := il.identityStream.Purge(ctx, jetstream.WithPurgeSubject(subject)); err != nil {
		slog.Error("natsstream identity: RemovePid purge failed",
			slog.String("subject", subject), slog.Any("error", err))
	}
}
```

- [ ] **Step 4: Run liveness tests to verify green**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run 'TestRemovePid_(SkipsDelete|DeletesWhenActor)' -v -count=1 ./cluster/clusterproviders/natsstream/`

Expected: All three PASS.

- [ ] **Step 5: Run all natsstream unit tests to verify no regressions**

Run: `cd /home/cchamplin/development/protoactor-go && go test -v -count=1 -race ./cluster/clusterproviders/natsstream/`

Expected: All PASS.

- [ ] **Step 6: Commit**

```bash
git add cluster/clusterproviders/natsstream/natsstream_identity.go cluster/clusterproviders/natsstream/natsstream_identity_test.go
git commit -m "fix(natsstream): add liveness guard to RemovePid

Mirror the natskv fix: skip stream record deletion when the actor is
alive in the local process registry."
```

### Task 7: Backfill tests for natsstream purgeActivationsForMember (commit 4b0a3586)

**Files:**
- Modify: `cluster/clusterproviders/natsstream/natsstream_identity_test.go`

- [ ] **Step 1: Write test for stream-scanning fallback in removeMemberID**

```go
// TestRemoveMemberID_ScansStreamForRemoteMember verifies that when a remote
// member departs, removeMemberID falls back to scanning the NATS identity
// stream to purge stale activations. This covers the code path added in
// commit 4b0a3586 where the local memberKeys map has no entries for the
// departed member.
func TestRemoveMemberID_ScansStreamForRemoteMember(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-scan-remote-member")

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	il := p.IdentityLookup()
	il.Setup(c, []string{"TestKind"}, false)
	time.Sleep(500 * time.Millisecond)

	ctx := context.Background()

	// Simulate a remote member's activations by publishing directly to the
	// identity stream (bypassing local memberKeys tracking).
	remoteMemberID := "remote-member-departed"
	ci1 := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "remote-grain-1"}
	ci2 := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "remote-grain-2"}

	for _, ci := range []*cluster.ClusterIdentity{ci1, ci2} {
		rec := activationRecord{
			PidID:      ci.Kind + "/" + ci.Identity,
			PidAddress: "remote-host:9999",
			MemberID:   remoteMemberID,
		}
		data, err := json.Marshal(&rec)
		require.NoError(t, err)
		_, err = p.js.Publish(ctx, il.identitySubject(ci), data)
		require.NoError(t, err)
	}

	// Also add a local member's activation that should NOT be purged.
	ciLocal := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "local-grain"}
	lockID, seq, ok := il.tryAcquireLock(ctx, ciLocal)
	require.True(t, ok)
	require.NoError(t, il.storeActivation(ctx, ciLocal, lockID, seq, il.memberID, "127.0.0.1:8080", "TestKind/local-grain"))

	// Verify all three activations exist.
	require.NotNil(t, il.getExistingActivation(ctx, ci1))
	require.NotNil(t, il.getExistingActivation(ctx, ci2))
	require.NotNil(t, il.getExistingActivation(ctx, ciLocal))

	// Verify local memberKeys has NO entries for the remote member.
	il.memberKeysMu.Lock()
	remoteKeys := il.memberKeys[remoteMemberID]
	il.memberKeysMu.Unlock()
	require.Empty(t, remoteKeys, "local node should not track remote member's keys")

	// Remove the remote member — should trigger stream scan fallback.
	il.removeMemberID(ctx, remoteMemberID)

	// Remote member's activations should be purged.
	assert.Nil(t, il.getExistingActivation(ctx, ci1),
		"remote member's activation should be purged via stream scan")
	assert.Nil(t, il.getExistingActivation(ctx, ci2),
		"remote member's activation should be purged via stream scan")

	// Local member's activation should be untouched.
	assert.NotNil(t, il.getExistingActivation(ctx, ciLocal),
		"local member's activation must NOT be purged when removing a different member")
}
```

- [ ] **Step 2: Run to verify it passes (tests existing behavior from 4b0a3586)**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run TestRemoveMemberID_ScansStreamForRemoteMember -v -count=1 ./cluster/clusterproviders/natsstream/`

Expected: PASS — validates the existing stream-scanning code.

### Task 8: Backfill test for natsstream ErrNameExists recovery (commit 4b0a3586)

**Files:**
- Modify: `cluster/clusterproviders/natsstream/natsstream_identity_test.go`

- [ ] **Step 1: Write test for ErrNameExists recovery in natsstream spawnActivation**

This is structurally identical to the natskv version but uses the natsstream setup:

First, add a `setupClusterWithKindsEmbedded` helper to `testhelpers_test.go` so we can register kinds with an embedded NATS server (the existing `setupClusterWithKinds` requires a testcontainer URL and is integration-only):

```go
// setupClusterWithKindsEmbedded is like setupCluster but also registers kinds
// via cluster.WithKinds. This is needed for unit tests that call
// spawnActivation (which requires TryGetClusterKind to succeed).
func setupClusterWithKindsEmbedded(t *testing.T, srv *server.Server, clusterName string, kinds []*cluster.Kind, opts ...Option) (*Provider, *cluster.Cluster) {
	t.Helper()

	nc, _ := connectNATS(t, srv)

	p, err := New(nc, opts...)
	require.NoError(t, err)

	system := actor.NewActorSystem()
	remoteConfig := remote.Configure("127.0.0.1", 0)
	clusterConfig := cluster.Configure(clusterName, p, p.IdentityLookup(), remoteConfig,
		cluster.WithKinds(kinds...),
	)
	c := cluster.NewCluster(system, clusterConfig)
	c.Remote = remote.NewRemote(system, remoteConfig)

	return p, c
}
```

Then the test:

```go
// TestSpawnActivation_ErrNameExists_ReRegisters verifies that when
// SpawnNamed returns ErrNameExists, spawnActivation re-registers the
// existing PID in the identity stream.
func TestSpawnActivation_ErrNameExists_ReRegisters(t *testing.T) {
	srv := startEmbeddedNATS(t)
	kindProps := actor.PropsFromFunc(func(ctx actor.Context) {})
	p, c := setupClusterWithKindsEmbedded(t, srv, "test-errname-reregister",
		[]*cluster.Kind{cluster.NewKind("TestKind", kindProps)})

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	il := p.IdentityLookup()
	il.Setup(c, []string{"TestKind"}, false)
	time.Sleep(500 * time.Millisecond)

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "grain-exists"}
	ctx := context.Background()

	// Pre-spawn the actor so SpawnNamed will return ErrNameExists.
	props := cluster.WithClusterIdentity(kindProps, ci)
	existingPid, err := c.ActorSystem.Root.SpawnNamed(props, "TestKind/grain-exists")
	require.NoError(t, err)
	t.Cleanup(func() { c.ActorSystem.Root.Poison(existingPid) })

	// Acquire lock.
	lockID, seq, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok)

	// spawnActivation should detect ErrNameExists and re-register.
	pid := il.spawnActivation(ci, lockID, seq)
	require.NotNil(t, pid, "should return existing PID, not nil")
	assert.Equal(t, existingPid.Id, pid.Id)
	assert.Equal(t, existingPid.Address, pid.Address)

	// Activation should be in the stream.
	rec := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec, "activation must be stored after ErrNameExists recovery")
	assert.Equal(t, existingPid.Id, rec.PidID)
}
```

- [ ] **Step 2: Run to verify it passes**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run TestSpawnActivation_ErrNameExists_ReRegisters -v -count=1 ./cluster/clusterproviders/natsstream/`

Expected: PASS.

- [ ] **Step 3: Run all natsstream unit tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test -v -count=1 -race ./cluster/clusterproviders/natsstream/`

Expected: All PASS.

- [ ] **Step 4: Commit**

```bash
git add cluster/clusterproviders/natsstream/natsstream_identity_test.go cluster/clusterproviders/natsstream/testhelpers_test.go
git commit -m "test(natsstream): backfill tests for stream scan and ErrNameExists recovery

- Test removeMemberID stream-scanning fallback for remote member cleanup
- Test spawnActivation ErrNameExists re-registration path
- Add setupClusterWithKindsEmbedded helper for unit tests needing kinds
Both cover untested code from commit 4b0a3586."
```

---

## Chunk 4: Integration Test — Slow Actor Identity Preserved

### Task 9: natskv integration test for slow actor

**Files:**
- Create: `cluster/clusterproviders/natskv/natskv_identity_liveness_integration_test.go`

- [ ] **Step 1: Write integration test**

```go
//go:build integration

package natskv

import (
	"context"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SlowPingMessage is a request that causes a delayed response.
type SlowPingMessage struct{}

// SlowPingResponse is the response to a SlowPingMessage.
type SlowPingResponse struct{ From string }

// TestIntegration_SlowActor_IdentityPreserved verifies the end-to-end fix:
// when a cluster request times out because the actor is slow (not dead),
// the identity record must NOT be deleted. After retry with a longer timeout,
// the same actor should respond successfully.
func TestIntegration_SlowActor_IdentityPreserved(t *testing.T) {
	natsURL := startNATSContainer(t)

	nc, err := nats.Connect(natsURL)
	require.NoError(t, err)
	t.Cleanup(func() { nc.Close() })

	provider, err := New(nc)
	require.NoError(t, err)

	system := actor.NewActorSystem()
	remoteConfig := remote.Configure("127.0.0.1", 0)

	slowKind := cluster.NewKind("SlowActor", actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *SlowPingMessage:
			// Simulate slow processing — takes 2 seconds.
			time.Sleep(2 * time.Second)
			ctx.Respond(&SlowPingResponse{From: ctx.Self().Id})
		}
	}))

	clusterConfig := cluster.Configure("integ-slow-actor", provider, provider.IdentityLookup(),
		remoteConfig,
		cluster.WithKinds(slowKind),
	)
	c := cluster.NewCluster(system, clusterConfig)
	require.NoError(t, c.StartMember())
	t.Cleanup(func() { c.Shutdown(true) })

	// Allow cluster to settle.
	time.Sleep(1 * time.Second)

	// First request: short timeout → should fail with timeout.
	_, err = c.Request("slow-grain-1", "SlowActor", &SlowPingMessage{},
		cluster.WithTimeout(500*time.Millisecond),
		cluster.WithRetryCount(1),
	)
	// Expect timeout error.
	require.Error(t, err, "request with short timeout should fail")

	// The identity record should still exist — the actor is alive.
	il := provider.IdentityLookup()
	ci := &cluster.ClusterIdentity{Kind: "SlowActor", Identity: "slow-grain-1"}
	ctx := context.Background()
	rec := il.getExistingActivation(ctx, ci)
	assert.NotNil(t, rec,
		"identity record must survive a timeout — the actor is alive, just slow")

	// Second request: long timeout → should succeed, same actor responds.
	resp, err := c.Request("slow-grain-1", "SlowActor", &SlowPingMessage{},
		cluster.WithTimeout(10*time.Second),
		cluster.WithRetryCount(3),
	)
	require.NoError(t, err, "request with long timeout should succeed")
	require.NotNil(t, resp)

	slowResp, ok := resp.(*SlowPingResponse)
	require.True(t, ok, "response should be SlowPingResponse")
	assert.Contains(t, slowResp.From, "SlowActor/slow-grain-1",
		"response should come from the original actor, not a re-spawned one")
}
```

- [ ] **Step 2: Run the integration test**

Run: `cd /home/cchamplin/development/protoactor-go && go test -tags integration -run TestIntegration_SlowActor_IdentityPreserved -v -count=1 -timeout 120s ./cluster/clusterproviders/natskv/`

Expected: PASS — this is the end-to-end validation that the three-layer fix works.

- [ ] **Step 3: Commit**

```bash
git add cluster/clusterproviders/natskv/natskv_identity_liveness_integration_test.go
git commit -m "test(natskv): add integration test for slow actor identity preservation

End-to-end test: a slow actor causes request timeouts, but its identity
record must survive. A subsequent request with a longer timeout succeeds
and is served by the original actor (not a re-spawned one)."
```

### Task 10: Final verification — run all tests

- [ ] **Step 1: Run all unit tests across all modules**

Run: `cd /home/cchamplin/development/protoactor-go && go test -race -count=1 ./...`

Expected: All PASS.

- [ ] **Step 2: Run natskv integration tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test -tags integration -race -count=1 -timeout 300s ./cluster/clusterproviders/natskv/`

Expected: All PASS.

- [ ] **Step 3: Run natsstream integration tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test -tags integration -race -count=1 -timeout 300s ./cluster/clusterproviders/natsstream/`

Expected: All PASS.
