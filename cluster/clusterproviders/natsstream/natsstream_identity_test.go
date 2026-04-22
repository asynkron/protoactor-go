package natsstream

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdentityLookup_InterfaceCompliance(t *testing.T) {
	// Compile-time check is in natsstream_identity.go
	var _ cluster.IdentityLookup = (*IdentityLookup)(nil)
}

func TestIdentityLookup_KvKey(t *testing.T) {
	ci := &cluster.ClusterIdentity{Kind: "MyKind", Identity: "abc-123"}
	key := kvKey(ci)
	assert.Equal(t, "MyKind.abc-123", key)
	assert.False(t, strings.Contains(key, "/"), "key should not contain slashes")
}

func TestIdentityLookup_TryAcquireLock(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-identity-lock")

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	il := p.IdentityLookup()
	il.Setup(c, []string{"MyKind"}, false)

	// Wait for identity stream to be created
	time.Sleep(500 * time.Millisecond)

	ci := &cluster.ClusterIdentity{Kind: "MyKind", Identity: "abc-123"}
	ctx := context.Background()

	lockID, seq, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok, "should acquire lock successfully")
	assert.NotEmpty(t, lockID)
	assert.Greater(t, seq, uint64(0))

	// Second attempt should fail (lock already held).
	_, _, ok2 := il.tryAcquireLock(ctx, ci)
	assert.False(t, ok2, "second lock attempt should fail")
}

func TestIdentityLookup_StoreActivation(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-identity-store")

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	il := p.IdentityLookup()
	il.Setup(c, []string{"MyKind"}, false)

	time.Sleep(500 * time.Millisecond)

	ci := &cluster.ClusterIdentity{Kind: "MyKind", Identity: "def-456"}
	ctx := context.Background()

	lockID, seq, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok)

	storeErr := il.storeActivation(ctx, ci, lockID, seq, "member1", "127.0.0.1:8080", "MyKind/def-456")
	require.NoError(t, storeErr)

	// Verify we can read it back.
	rec := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec, "should find stored activation")
	assert.Equal(t, "MyKind/def-456", rec.PidID)
	assert.Equal(t, "127.0.0.1:8080", rec.PidAddress)
	assert.Equal(t, "member1", rec.MemberID)
}

func TestIdentityLookup_GetExisting_NotFound(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-identity-notfound")

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	il := p.IdentityLookup()
	il.Setup(c, []string{"MyKind"}, false)

	time.Sleep(500 * time.Millisecond)

	ci := &cluster.ClusterIdentity{Kind: "MyKind", Identity: "nonexistent"}
	ctx := context.Background()

	rec := il.getExistingActivation(ctx, ci)
	assert.Nil(t, rec, "non-existent identity should return nil")
}

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

// TestRemoveMemberID_ScansStreamForRemoteMember verifies that when a remote
// member departs, removeMemberID falls back to scanning the NATS identity
// stream to purge stale activations.
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

func TestIdentityLookup_RemoveMember_PurgesActivations(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-identity-remove")

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	il := p.IdentityLookup()
	il.Setup(c, []string{"MyKind"}, false)

	time.Sleep(500 * time.Millisecond)

	ctx := context.Background()
	ci := &cluster.ClusterIdentity{Kind: "MyKind", Identity: "to-remove"}

	// Acquire lock and store activation.
	lockID, seq, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok)
	require.NoError(t, il.storeActivation(ctx, ci, lockID, seq, "member-x", "127.0.0.1:8080", "MyKind/to-remove"))

	// Verify it exists.
	rec := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec)

	// Remove all activations for member-x.
	il.removeMemberID(ctx, "member-x")

	// Verify it's gone.
	rec = il.getExistingActivation(ctx, ci)
	assert.Nil(t, rec, "activation should be purged after removeMemberID")
}

// --- SP4: Placement actor integration tests ---

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
	_, _, il := setupPlacementTestCluster(t, "test-sp4-setup")

	require.NotNil(t, il.placementPID.Load(), "placementPID should be set after Setup()")
	require.NotNil(t, il.proxyPID.Load(), "proxyPID should be set after Setup()")
	require.NotNil(t, il.strategyMgr.Load(), "strategyMgr should be set after Setup()")

	// Get() should work end-to-end.
	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "setup-test"}
	pid := il.Get(ci)
	require.NotNil(t, pid, "Get() should return a PID after Setup()")
}

func TestIdentityLookup_CoalesceConcurrentGets(t *testing.T) {
	_, _, il := setupPlacementTestCluster(t, "test-sp4-coalesce")

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
	_, c, il := setupPlacementTestCluster(t, "test-sp4-stale")

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "stale-1"}
	ctx := context.Background()

	// Insert a stale activation from a dead member directly into stream.
	staleRec := activationRecord{
		PidID:      "TestKind/stale-1",
		PidAddress: "dead-host:9999",
		MemberID:   "dead-member-xyz",
	}
	data, _ := json.Marshal(&staleRec)
	subject := il.identitySubject(ci)
	_, err := il.provider.js.Publish(ctx, subject, data)
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
	_, _, il := setupPlacementTestCluster(t, "test-sp4-e2e")

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "order-123"}

	// First Get — should activate via placement actor.
	pid1 := il.Get(ci)
	require.NotNil(t, pid1, "first Get should return PID")

	// Second Get — should find existing activation (no new spawn).
	pid2 := il.Get(ci)
	require.NotNil(t, pid2, "second Get should return PID")
	assert.True(t, pid1.Equal(pid2), "second Get should return same PID")

	// Verify the activation is stored in the stream.
	ctx := context.Background()
	rec := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec, "activation should be persisted in stream")
	assert.Equal(t, il.memberID, rec.MemberID, "member ID should be ours")
}

func TestNatsStream_Peek_Alive(t *testing.T) {
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

	_ = p
	_ = c
}

func TestNatsStream_Peek_NotFound(t *testing.T) {
	_, _, il := setupPlacementTestCluster(t, "peek-notfound")

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "never-activated"}
	result, err := il.Peek(ci)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusNotFound, result.Status)
}

func TestNatsStream_Peek_MemberDead(t *testing.T) {
	_, _, il := setupPlacementTestCluster(t, "peek-member-dead")

	ctx := context.Background()
	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "dead-grain"}
	rec := activationRecord{
		MemberID:   "dead-member-id",
		PidID:      "TestKind/dead-grain",
		PidAddress: "dead-host:9999",
	}
	data, _ := json.Marshal(&rec)
	subject := il.identitySubject(ci)
	_, err := il.provider.js.Publish(ctx, subject, data)
	require.NoError(t, err)

	result, err := il.Peek(ci)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusMemberDead, result.Status)
	assert.Equal(t, "dead-grain", result.Identity)
}

func TestNatsStream_Peek_Stale(t *testing.T) {
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
