package natskv

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/cluster"
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
	assert.NotContains(t, key, "/")
	assert.Contains(t, key, ".")
}

func TestIdentityLookup_kvKey_ReplacesSlash(t *testing.T) {
	ci := &cluster.ClusterIdentity{Kind: "SomeKind", Identity: "some-id"}
	key := kvKey(ci)
	// AsKey() returns "SomeKind/some-id", kvKey replaces "/" with "."
	assert.Equal(t, "SomeKind.some-id", key)
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
