package natskv

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdentityLookup_InterfaceCompliance(t *testing.T) {
	var _ cluster.IdentityLookup = (*IdentityLookup)(nil)
}

func TestIdentityLookup_kvKey(t *testing.T) {
	ci := &cluster.ClusterIdentity{Kind: "MyGrain", Identity: "abc-123"}
	key := kvKey(ci)
	// kvKey is "kind/identity" with ':' rewritten to '_'. The first '/' is
	// the kind|identity boundary; kinds are forbidden from containing '/' so
	// it is always unambiguous.
	assert.Equal(t, "MyGrain/abc-123", key)
}

func TestIdentityLookup_kvKey_PreservesSlashInIdentity(t *testing.T) {
	ci := &cluster.ClusterIdentity{Kind: "SomeKind", Identity: "org/team/user"}
	key := kvKey(ci)
	assert.Equal(t, "SomeKind/org/team/user", key)
}

func TestIdentityLookup_kvKey_EscapesColon(t *testing.T) {
	ci := &cluster.ClusterIdentity{Kind: "SomeKind", Identity: "tenant:domain"}
	key := kvKey(ci)
	// ':' is not a valid NATS KV key character, so it is rewritten to '_'.
	assert.Equal(t, "SomeKind/tenant_domain", key)
}

func TestIdentityLookup_AcquireLock(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	ctx := context.Background()
	identities, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_acquire_lock_identities",
	})
	require.NoError(t, err)

	il := &IdentityLookup{
		identities: identities,
		config:     newDefaultConfig(),
		semaphore:  make(chan struct{}, 200),
	}

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "id1"}

	// First acquire should succeed.
	lockID, revision, ok := il.tryAcquireLock(ctx, ci)
	assert.True(t, ok, "first lock acquire should succeed")
	assert.NotEmpty(t, lockID)
	assert.Greater(t, revision, uint64(0))

	// Second acquire should fail because the key already exists.
	_, _, ok2 := il.tryAcquireLock(ctx, ci)
	assert.False(t, ok2, "second lock acquire should fail")
}

func TestIdentityLookup_StoreActivation(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	ctx := context.Background()
	identities, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_store_activation_identities",
	})
	require.NoError(t, err)

	tracking, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_store_activation_tracking",
	})
	require.NoError(t, err)

	il := &IdentityLookup{
		identities:    identities,
		memberTracker: tracking,
		config:        newDefaultConfig(),
		semaphore:     make(chan struct{}, 200),
	}

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "id1"}

	// Acquire lock first.
	lockID, revision, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok)

	// Store activation with CAS.
	err = il.storeActivation(ctx, ci, lockID, revision, "member-1", "127.0.0.1:8080", "TestKind/id1")
	require.NoError(t, err)

	// Verify stored record.
	rec := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec, "should find stored activation")
	assert.Equal(t, "127.0.0.1:8080", rec.PidAddress)
	assert.Equal(t, "TestKind/id1", rec.PidID)
	assert.Equal(t, "member-1", rec.MemberID)
	assert.Empty(t, rec.LockID, "lock ID should be cleared after storing activation")
}

func TestIdentityLookup_GetExistingActivation(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	ctx := context.Background()
	identities, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_get_existing_identities",
	})
	require.NoError(t, err)

	tracking, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_get_existing_tracking",
	})
	require.NoError(t, err)

	il := &IdentityLookup{
		identities:    identities,
		memberTracker: tracking,
		config:        newDefaultConfig(),
		semaphore:     make(chan struct{}, 200),
	}

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "id1"}

	// No activation should exist yet.
	rec := il.getExistingActivation(ctx, ci)
	assert.Nil(t, rec, "should return nil when no activation exists")

	// Only a lock record (no PID) should also return nil.
	lockID, revision, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok)

	rec = il.getExistingActivation(ctx, ci)
	assert.Nil(t, rec, "should return nil for lock-only record")

	// Store activation.
	err = il.storeActivation(ctx, ci, lockID, revision, "member-1", "127.0.0.1:8080", "TestKind/id1")
	require.NoError(t, err)

	rec = il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec, "should find activation after storing")
	assert.Equal(t, "127.0.0.1:8080", rec.PidAddress)
	assert.Equal(t, "TestKind/id1", rec.PidID)
}

func TestIdentityLookup_RemoveMember(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	ctx := context.Background()
	identities, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_remove_member_identities",
	})
	require.NoError(t, err)

	tracking, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_remove_member_tracking",
	})
	require.NoError(t, err)

	il := &IdentityLookup{
		identities:    identities,
		memberTracker: tracking,
		config:        newDefaultConfig(),
		semaphore:     make(chan struct{}, 200),
	}

	memberID := "member-1"

	// Create two activations for the same member.
	ci1 := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "id1"}
	ci2 := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "id2"}

	lockID1, rev1, ok := il.tryAcquireLock(ctx, ci1)
	require.True(t, ok)
	err = il.storeActivation(ctx, ci1, lockID1, rev1, memberID, "127.0.0.1:8080", "TestKind/id1")
	require.NoError(t, err)

	lockID2, rev2, ok := il.tryAcquireLock(ctx, ci2)
	require.True(t, ok)
	err = il.storeActivation(ctx, ci2, lockID2, rev2, memberID, "127.0.0.1:8080", "TestKind/id2")
	require.NoError(t, err)

	// Both should exist.
	assert.NotNil(t, il.getExistingActivation(ctx, ci1))
	assert.NotNil(t, il.getExistingActivation(ctx, ci2))

	// Remove the member.
	il.removeMemberID(ctx, memberID)

	// Both should be gone.
	assert.Nil(t, il.getExistingActivation(ctx, ci1), "activation 1 should be removed")
	assert.Nil(t, il.getExistingActivation(ctx, ci2), "activation 2 should be removed")
}

func TestIdentityLookup_WaitForActivation(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	ctx := context.Background()
	identities, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_wait_activation_identities",
	})
	require.NoError(t, err)

	tracking, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_wait_activation_tracking",
	})
	require.NoError(t, err)

	il := &IdentityLookup{
		identities:    identities,
		memberTracker: tracking,
		config:        newDefaultConfig(),
		semaphore:     make(chan struct{}, 200),
	}

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "wait1"}

	// Acquire lock first (so the key exists).
	lockID, revision, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok)

	// Start watching in a goroutine.
	result := make(chan *activationRecord, 1)
	go func() {
		rec := il.waitForActivation(ctx, ci)
		result <- rec
	}()

	// Wait a bit then store the activation.
	time.Sleep(100 * time.Millisecond)
	err = il.storeActivation(ctx, ci, lockID, revision, "member-1", "127.0.0.1:8080", "TestKind/wait1")
	require.NoError(t, err)

	// Wait for result with timeout.
	select {
	case rec := <-result:
		require.NotNil(t, rec, "should receive activation")
		assert.Equal(t, "127.0.0.1:8080", rec.PidAddress)
		assert.Equal(t, "TestKind/wait1", rec.PidID)
		assert.Equal(t, "member-1", rec.MemberID)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for activation")
	}
}

func TestIdentityLookup_WaitForActivation_Timeout(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	ctx := context.Background()
	identities, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_wait_timeout_identities",
	})
	require.NoError(t, err)

	cfg := newDefaultConfig()
	cfg.LockTTL = 500 * time.Millisecond // Short TTL for test.

	il := &IdentityLookup{
		identities: identities,
		config:     cfg,
		semaphore:  make(chan struct{}, 200),
	}

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "timeout1"}

	// No activation will be stored, so it should time out.
	start := time.Now()
	rec := il.waitForActivation(ctx, ci)
	elapsed := time.Since(start)

	assert.Nil(t, rec, "should return nil on timeout")
	assert.GreaterOrEqual(t, elapsed, 400*time.Millisecond, "should wait at least close to LockTTL")
}

func TestIdentityLookup_AddKeyToMember(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	ctx := context.Background()
	tracking, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_add_key_tracking",
	})
	require.NoError(t, err)

	il := &IdentityLookup{
		memberTracker: tracking,
		config:        newDefaultConfig(),
		semaphore:     make(chan struct{}, 200),
	}

	memberID := "member-1"

	// Add first key.
	il.addKeyToMember(ctx, memberID, "TestKind.id1")

	// Verify it's tracked.
	entry, err := tracking.Get(ctx, memberID)
	require.NoError(t, err)

	var mrec memberRecord
	require.NoError(t, json.Unmarshal(entry.Value(), &mrec))
	assert.Equal(t, []string{"TestKind.id1"}, mrec.Keys)

	// Add second key.
	il.addKeyToMember(ctx, memberID, "TestKind.id2")

	entry, err = tracking.Get(ctx, memberID)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(entry.Value(), &mrec))
	assert.Len(t, mrec.Keys, 2)
	assert.Contains(t, mrec.Keys, "TestKind.id1")
	assert.Contains(t, mrec.Keys, "TestKind.id2")

	// Adding duplicate should not create duplicate entry.
	il.addKeyToMember(ctx, memberID, "TestKind.id1")

	entry, err = tracking.Get(ctx, memberID)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(entry.Value(), &mrec))
	assert.Len(t, mrec.Keys, 2)
}

func TestIdentityLookup_RemoveKeyFromMember(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	ctx := context.Background()
	tracking, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_remove_key_tracking",
	})
	require.NoError(t, err)

	il := &IdentityLookup{
		memberTracker: tracking,
		config:        newDefaultConfig(),
		semaphore:     make(chan struct{}, 200),
	}

	memberID := "member-1"

	// Add two keys.
	il.addKeyToMember(ctx, memberID, "TestKind.id1")
	il.addKeyToMember(ctx, memberID, "TestKind.id2")

	// Remove one.
	il.removeKeyFromMember(ctx, memberID, "TestKind.id1")

	entry, err := tracking.Get(ctx, memberID)
	require.NoError(t, err)

	var mrec memberRecord
	require.NoError(t, json.Unmarshal(entry.Value(), &mrec))
	assert.Equal(t, []string{"TestKind.id2"}, mrec.Keys)
}

func TestIdentityLookup_SetupError_GetReturnsNil(t *testing.T) {
	il := &IdentityLookup{
		config:    newDefaultConfig(),
		semaphore: make(chan struct{}, 1),
		setupErr:  fmt.Errorf("simulated setup failure"),
	}

	assert.NotPanics(t, func() {
		result := il.Get(&cluster.ClusterIdentity{Kind: "test", Identity: "1"})
		assert.Nil(t, result)
	})
}

func TestIdentityLookup_SetupError_RemovePidDoesNotPanic(t *testing.T) {
	il := &IdentityLookup{
		config:    newDefaultConfig(),
		semaphore: make(chan struct{}, 1),
		setupErr:  fmt.Errorf("simulated setup failure"),
	}

	assert.NotPanics(t, func() {
		il.RemovePid(&cluster.ClusterIdentity{Kind: "test", Identity: "1"}, actor.NewPID("addr", "id"))
	})
}

func TestIdentityLookup_SetupError_ShutdownDoesNotPanic(t *testing.T) {
	il := &IdentityLookup{
		config:    newDefaultConfig(),
		semaphore: make(chan struct{}, 1),
		setupErr:  fmt.Errorf("simulated setup failure"),
		memberID:  "test-member",
	}

	assert.NotPanics(t, func() {
		il.Shutdown()
	})
}

// --- Graceful shutdown cleanup tests ---
//
// These tests verify that IdentityLookup.Shutdown() properly cleans up
// all identity activations belonging to the shutting-down member.

func TestIdentityLookup_Shutdown_CleansUpOwnActivations(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	ctx := context.Background()
	identities, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_shutdown_cleanup_identities",
	})
	require.NoError(t, err)

	tracking, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_shutdown_cleanup_tracking",
	})
	require.NoError(t, err)

	memberID := "member-shutting-down"
	il := &IdentityLookup{
		identities:    identities,
		memberTracker: tracking,
		config:        newDefaultConfig(),
		semaphore:     make(chan struct{}, 200),
		memberID:      memberID,
	}

	// Create several activations via the normal lock+store path.
	grains := []*cluster.ClusterIdentity{
		{Kind: "Agg", Identity: "bet-1"},
		{Kind: "Proj", Identity: "market-list"},
		{Kind: "PM", Identity: "settlement-1"},
	}

	for _, ci := range grains {
		lockID, rev, ok := il.tryAcquireLock(ctx, ci)
		require.True(t, ok, "lock for %s", kvKey(ci))
		err := il.storeActivation(ctx, ci, lockID, rev, memberID, "127.0.0.1:8080", ci.Kind+"/"+ci.Identity)
		require.NoError(t, err, "store activation for %s", kvKey(ci))
	}

	// Verify all activations exist.
	for _, ci := range grains {
		rec := il.getExistingActivation(ctx, ci)
		require.NotNil(t, rec, "activation for %s should exist before shutdown", kvKey(ci))
	}

	// Graceful shutdown.
	il.Shutdown()

	// All activations should be cleaned up.
	for _, ci := range grains {
		_, err := identities.Get(ctx, kvKey(ci))
		assert.ErrorIs(t, err, jetstream.ErrKeyNotFound,
			"activation for %s should be deleted after shutdown", kvKey(ci))
	}

	// Member tracking record should be deleted.
	_, err = tracking.Get(ctx, memberID)
	assert.ErrorIs(t, err, jetstream.ErrKeyNotFound,
		"member tracking record should be deleted after shutdown")
}

func TestIdentityLookup_Shutdown_DoesNotAffectOtherMembers(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	ctx := context.Background()
	identities, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_shutdown_other_identities",
	})
	require.NoError(t, err)

	tracking, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_shutdown_other_tracking",
	})
	require.NoError(t, err)

	// Set up two "members", each with their own IdentityLookup.
	il1 := &IdentityLookup{
		identities:    identities,
		memberTracker: tracking,
		config:        newDefaultConfig(),
		semaphore:     make(chan struct{}, 200),
		memberID:      "member-1",
	}
	il2 := &IdentityLookup{
		identities:    identities,
		memberTracker: tracking,
		config:        newDefaultConfig(),
		semaphore:     make(chan struct{}, 200),
		memberID:      "member-2",
	}

	// member-1 activates a grain.
	ci1 := &cluster.ClusterIdentity{Kind: "Agg", Identity: "bet-1"}
	lockID1, rev1, ok := il1.tryAcquireLock(ctx, ci1)
	require.True(t, ok)
	require.NoError(t, il1.storeActivation(ctx, ci1, lockID1, rev1, "member-1", "h1:8080", "Agg/bet-1"))

	// member-2 activates a different grain.
	ci2 := &cluster.ClusterIdentity{Kind: "Agg", Identity: "bet-2"}
	lockID2, rev2, ok := il2.tryAcquireLock(ctx, ci2)
	require.True(t, ok)
	require.NoError(t, il2.storeActivation(ctx, ci2, lockID2, rev2, "member-2", "h2:8080", "Agg/bet-2"))

	// Shutdown member-1 only.
	il1.Shutdown()

	// member-1's activation should be gone.
	_, err = identities.Get(ctx, kvKey(ci1))
	assert.ErrorIs(t, err, jetstream.ErrKeyNotFound, "member-1 activation should be deleted")

	// member-2's activation should still exist.
	rec := il2.getExistingActivation(ctx, ci2)
	require.NotNil(t, rec, "member-2 activation should survive member-1 shutdown")
	assert.Equal(t, "Agg/bet-2", rec.PidID)
}

// TestRemovePid_SkipsDeleteWhenActorAliveLocally verifies that RemovePid
// does NOT delete the KV record when the actor is still running in the
// local process registry. This prevents the orphaned-actor bug where a
// timeout triggers RemovePid but the actor is just slow, not dead.
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
	lockID, rev, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok)
	err = il.storeActivation(ctx, ci, lockID, rev, il.memberID, pid.Address, pid.Id)
	require.NoError(t, err)

	// Call RemovePid — should be a no-op because actor is alive locally.
	il.RemovePid(ci, pid)

	// KV record must still exist.
	rec := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec,
		"RemovePid must NOT delete the KV record when the actor is alive locally")
	assert.Equal(t, pid.Id, rec.PidID)
}

// TestRemovePid_DeletesWhenActorNotAliveLocally verifies that RemovePid
// DOES delete the KV record when the PID points to a local address but
// the actor is no longer in the process registry (it was stopped/crashed).
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

	// Spawn and immediately stop the actor.
	pid, err := system.Root.SpawnNamed(actor.PropsFromFunc(func(ctx actor.Context) {}), "TestKind/grain-dead")
	require.NoError(t, err)
	system.Root.Poison(pid)
	time.Sleep(200 * time.Millisecond) // wait for actor to fully stop

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "grain-dead"}
	ctx := context.Background()

	// Store activation pointing to the dead local actor.
	lockID, rev, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok)
	err = il.storeActivation(ctx, ci, lockID, rev, il.memberID, pid.Address, pid.Id)
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

	// Store activation pointing to a remote host.
	lockID, rev, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok)
	err = il.storeActivation(ctx, ci, lockID, rev, "member-remote", remotePid.Address, remotePid.Id)
	require.NoError(t, err)

	// RemovePid should delete — we can't verify remote liveness.
	il.RemovePid(ci, remotePid)

	result := il.getExistingActivation(ctx, ci)
	assert.Nil(t, result,
		"RemovePid must delete the KV record for remote actors (can't check liveness)")
}

// --- SP3: Placement actor integration tests ---

// setupPlacementTestCluster creates a cluster with a placement actor, proxy,
// strategy manager, and a single member topology — ready for testing Get().
func setupPlacementTestCluster(t *testing.T, clusterName string) (*Provider, *cluster.Cluster, *IdentityLookup) {
	t.Helper()

	srv := startEmbeddedNATS(t)
	kindProps := actor.PropsFromFunc(func(ctx actor.Context) {})
	kind := cluster.NewKind("TestKind", kindProps)

	p, c := setupClusterWithKindsEmbedded(t, srv, clusterName,
		[]*cluster.Kind{kind})

	// Start remote so ActorSystem.Address() returns a real host:port.
	err := c.Remote.Start()
	require.NoError(t, err)

	// Initialize kinds so the placement actor can look them up.
	c.InitKindsForTest(kind)

	il := p.IdentityLookup()
	il.Setup(c, []string{"TestKind"}, false)

	// Publish a self-only topology so the strategy manager and
	// ValidateActivationMember know about this member.
	host, port, err := c.ActorSystem.GetHostPort()
	require.NoError(t, err)
	self := &cluster.Member{
		Host:  host,
		Port:  int32(port),
		Id:    il.memberID,
		Kinds: []string{"TestKind"},
	}
	c.MemberList.UpdateClusterTopology(cluster.Members{self})

	t.Cleanup(func() {
		il.Shutdown()
		c.Remote.Shutdown(true)
	})

	return p, c, il
}

func TestIdentityLookup_SetupSpawnsPlacementAndProxy(t *testing.T) {
	_, _, il := setupPlacementTestCluster(t, "test-sp3-setup")

	require.NotNil(t, il.placementPID.Load(), "placementPID should be set after Setup()")
	require.NotNil(t, il.proxyPID.Load(), "proxyPID should be set after Setup()")
	require.NotNil(t, il.strategyMgr.Load(), "strategyMgr should be set after Setup()")

	// Get() should work end-to-end.
	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "setup-test"}
	pid := il.Get(ci)
	require.NotNil(t, pid, "Get() should return a PID after Setup()")
}

func TestIdentityLookup_CoalesceConcurrentGets(t *testing.T) {
	_, _, il := setupPlacementTestCluster(t, "test-sp3-coalesce")

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "coalesce-1"}
	const concurrency = 10

	var wg sync.WaitGroup
	pids := make([]*actor.PID, concurrency)

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			pids[idx] = il.Get(ci)
		}(i)
	}
	wg.Wait()

	// All should have returned a non-nil PID.
	for i, pid := range pids {
		require.NotNil(t, pid, "goroutine %d returned nil PID", i)
	}

	// All should be the same PID (coalesced).
	for i := 1; i < concurrency; i++ {
		assert.True(t, pids[0].Equal(pids[i]),
			"PID[0]=%v != PID[%d]=%v — concurrent Gets should coalesce", pids[0], i, pids[i])
	}
}

func TestIdentityLookup_StaleActivationCleaned(t *testing.T) {
	_, c, il := setupPlacementTestCluster(t, "test-sp3-stale")

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "stale-1"}
	ctx := context.Background()

	// Insert a stale activation from a dead member directly into KV.
	staleRec := activationRecord{
		PidID:      "TestKind/stale-1",
		PidAddress: "dead-host:9999",
		MemberID:   "dead-member-xyz",
	}
	data, _ := json.Marshal(&staleRec)
	_, err := il.identities.Put(ctx, kvKey(ci), data)
	require.NoError(t, err)

	// Verify the stale activation exists.
	rec := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec, "stale activation should exist before Get()")

	// Get() should detect the stale member, clean it up, and re-activate.
	pid := il.Get(ci)
	require.NotNil(t, pid, "Get() should return a PID after cleaning stale activation")

	// The new PID should be on this node, not the dead one.
	assert.Equal(t, c.ActorSystem.Address(), pid.Address,
		"new activation should be on the local node, not the dead member")
}

func TestIdentityLookup_EndToEnd_ActivateAndRetrieve(t *testing.T) {
	_, _, il := setupPlacementTestCluster(t, "test-sp3-e2e")

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "order-123"}

	// First Get — should activate via placement actor.
	pid1 := il.Get(ci)
	require.NotNil(t, pid1, "first Get should return PID")

	// Second Get — should find existing activation (no new spawn).
	pid2 := il.Get(ci)
	require.NotNil(t, pid2, "second Get should return PID")
	assert.True(t, pid1.Equal(pid2), "second Get should return same PID")

	// Verify the activation is stored in the KV.
	ctx := context.Background()
	rec := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec, "activation should be persisted in KV")
	assert.Equal(t, il.memberID, rec.MemberID, "member ID should be ours")
}

func TestNatsKV_Peek_Alive(t *testing.T) {
	p, c, il := setupPlacementTestCluster(t, "peek-alive")

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "peek-alive-1"}
	pid := il.Get(ci)
	require.NotNil(t, pid, "Get should activate the grain")

	result, err := il.Peek(ci)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusAlive, result.Status)
	assert.Equal(t, "peek-alive-1", result.Identity)
	assert.Equal(t, "TestKind", result.Kind)
	assert.Equal(t, pid, result.PID)

	_, _ = p, c
}

func TestNatsKV_Peek_NotFound(t *testing.T) {
	_, _, il := setupPlacementTestCluster(t, "peek-notfound")

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "never-activated"}
	result, err := il.Peek(ci)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusNotFound, result.Status)
}

func TestNatsKV_Peek_MemberDead(t *testing.T) {
	_, _, il := setupPlacementTestCluster(t, "peek-member-dead")

	ctx := context.Background()
	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "dead-grain"}
	rec := activationRecord{
		MemberID:   "dead-member-id",
		PidID:      "TestKind/dead-grain",
		PidAddress: "dead-host:9999",
	}
	data, _ := json.Marshal(&rec)
	_, err := il.identities.Put(ctx, kvKey(ci), data)
	require.NoError(t, err)

	result, err := il.Peek(ci)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusMemberDead, result.Status)
	assert.Equal(t, "dead-grain", result.Identity)
}

func TestNatsKV_Peek_Stale(t *testing.T) {
	p, c, il := setupPlacementTestCluster(t, "peek-stale")

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "peek-stale-1"}
	pid := il.Get(ci)
	require.NotNil(t, pid)

	c.ActorSystem.Root.Poison(pid)

	require.Eventually(t, func() bool {
		r, err := il.Peek(ci)
		return err == nil && r.Status == cluster.PeekStatusStale
	}, 5*time.Second, 50*time.Millisecond)

	_ = p
}

// TestIdentityLookup_GetExistingActivation_ReturnsActivationFromOtherMember
// verifies that getExistingActivation returns activations stored by other
// members without proactively deleting them. Stale PID cleanup is handled
// by the normal topology event path (ClusterTopology.Left → removeMemberID)
// and by graceful shutdown (IdentityLookup.Shutdown → removeMemberID).
// Proactive deletion based on MemberList liveness would be unsafe because
// MemberList is eventually-consistent and could lag behind reality, causing
// double-activations during network partitions or slow member refreshes.
func TestIdentityLookup_GetExistingActivation_ReturnsActivationFromOtherMember(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	ctx := context.Background()
	identities, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_other_member_identities",
	})
	require.NoError(t, err)

	il := &IdentityLookup{
		identities: identities,
		config:     newDefaultConfig(),
		semaphore:  make(chan struct{}, 200),
		memberID:   "member-A",
	}

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "id1"}

	// Write an activation belonging to a different member.
	rec := activationRecord{
		PidID:      "TestKind/id1",
		PidAddress: "other-host:8080",
		MemberID:   "member-B",
	}
	data, err := json.Marshal(&rec)
	require.NoError(t, err)
	_, err = identities.Put(ctx, kvKey(ci), data)
	require.NoError(t, err)

	// getExistingActivation must return the activation as-is, even though
	// the owning member may or may not be alive. The caller (DefaultContext)
	// will get a DeadLetter if the PID is stale and retry via RemovePid.
	result := il.getExistingActivation(ctx, ci)
	require.NotNil(t, result, "must return activation from other member without proactive deletion")
	assert.Equal(t, "TestKind/id1", result.PidID)
	assert.Equal(t, "other-host:8080", result.PidAddress)
	assert.Equal(t, "member-B", result.MemberID)

	// Verify the KV entry was NOT deleted.
	_, err = identities.Get(ctx, kvKey(ci))
	assert.NoError(t, err, "KV entry must not be deleted by getExistingActivation")
}
