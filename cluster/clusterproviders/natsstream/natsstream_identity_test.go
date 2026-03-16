package natsstream

import (
	"context"
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
