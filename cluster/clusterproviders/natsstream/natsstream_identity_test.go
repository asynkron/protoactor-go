package natsstream

import (
	"context"
	"encoding/json"
	"strings"
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

// TestSpawnActivation_ErrNameExists_ReRegisters verifies that when
// SpawnNamed returns ErrNameExists, spawnActivation re-registers the
// existing PID in the identity stream.
func TestSpawnActivation_ErrNameExists_ReRegisters(t *testing.T) {
	srv := startEmbeddedNATS(t)
	kindProps := actor.PropsFromFunc(func(ctx actor.Context) {})
	p, c := setupClusterWithKindsEmbedded(t, srv, "test-errname-reregister",
		[]*cluster.Kind{cluster.NewKind("TestKind", kindProps)})
	c.InitKindsForTest(cluster.NewKind("TestKind", kindProps))

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
