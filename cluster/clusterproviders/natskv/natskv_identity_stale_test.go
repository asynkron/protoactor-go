package natskv

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIdentityLookup_StaleActivation_BlocksReactivation demonstrates that
// when a node dies without cleaning up its identity claims, repeated calls
// to Get() keep returning the stale (dead) PID. The only way to unblock
// reactivation is via RemovePid() — which is never called by
// DefaultContext.Request() in the current code.
func TestIdentityLookup_StaleActivation_BlocksReactivation(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	ctx := context.Background()
	identities, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_stale_block_identities",
	})
	require.NoError(t, err)

	tracking, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_stale_block_tracking",
	})
	require.NoError(t, err)

	il := &IdentityLookup{
		identities:    identities,
		memberTracker: tracking,
		config:        newDefaultConfig(),
		semaphore:     make(chan struct{}, 200),
		memberID:      "member-alive",
	}

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "grain-1"}
	deadMember := "member-dead"

	// Simulate a stale activation left behind by a crashed node.
	// This is what remains in NATS KV when a node dies without calling
	// IdentityLookup.Shutdown().
	staleRec := activationRecord{
		PidID:      "TestKind/grain-1",
		PidAddress: "dead-host:9999",
		MemberID:   deadMember,
	}
	data, err := json.Marshal(&staleRec)
	require.NoError(t, err)
	_, err = identities.Put(ctx, kvKey(ci), data)
	require.NoError(t, err)

	// Also set up member tracking for the dead member (as the dead node
	// would have done during its lifetime).
	trackRec := memberRecord{Keys: []string{kvKey(ci)}}
	trackData, err := json.Marshal(&trackRec)
	require.NoError(t, err)
	_, err = tracking.Put(ctx, deadMember, trackData)
	require.NoError(t, err)

	// First Get() returns the stale PID — this is the bug manifestation.
	// The caller (DefaultContext) would send a message to this dead PID,
	// get a dead letter, and retry. But without RemovePid(), every retry
	// returns the same stale PID.
	rec := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec, "stale activation should be returned by Get()")
	assert.Equal(t, "dead-host:9999", rec.PidAddress,
		"Get() returns the dead PID from the stale activation")

	// Second call also returns the stale PID — proving the activation persists.
	rec2 := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec2, "stale activation should persist across multiple Get() calls")
	assert.Equal(t, "dead-host:9999", rec2.PidAddress)

	// Now call RemovePid — this is what DefaultContext.Request() SHOULD do
	// when it detects a dead letter, but currently doesn't.
	stalePid := actor.NewPID("dead-host:9999", "TestKind/grain-1")
	il.RemovePid(ci, stalePid)

	// After RemovePid, the stale activation should be gone.
	rec3 := il.getExistingActivation(ctx, ci)
	assert.Nil(t, rec3,
		"after RemovePid(), the stale activation must be cleared so that "+
			"a subsequent Get() can acquire the lock and spawn a new actor")

	// Verify a new lock can be acquired (reactivation is unblocked).
	_, _, ok := il.tryAcquireLock(ctx, ci)
	assert.True(t, ok,
		"after RemovePid() clears the stale activation, a new lock "+
			"should be acquirable for reactivation on a surviving node")
}

// TestIdentityLookup_StaleActivation_PersistsWithoutRemovePid is the
// negative test: it proves that without an explicit RemovePid() call,
// stale activations survive indefinitely in the KV store, blocking
// any new lock acquisition for the same grain identity.
func TestIdentityLookup_StaleActivation_PersistsWithoutRemovePid(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	ctx := context.Background()
	identities, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_stale_persist_identities",
	})
	require.NoError(t, err)

	tracking, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_stale_persist_tracking",
	})
	require.NoError(t, err)

	il := &IdentityLookup{
		identities:    identities,
		memberTracker: tracking,
		config:        newDefaultConfig(),
		semaphore:     make(chan struct{}, 200),
		memberID:      "member-alive",
	}

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "grain-stuck"}

	// Plant stale activation.
	staleRec := activationRecord{
		PidID:      "TestKind/grain-stuck",
		PidAddress: "dead-host:9999",
		MemberID:   "member-dead",
	}
	data, err := json.Marshal(&staleRec)
	require.NoError(t, err)
	_, err = identities.Put(ctx, kvKey(ci), data)
	require.NoError(t, err)

	// Attempting to acquire a lock fails because the key already exists.
	_, _, ok := il.tryAcquireLock(ctx, ci)
	assert.False(t, ok,
		"lock acquisition must fail when a stale activation exists "+
			"in the KV store — this is why RemovePid() is critical")

	// getExistingActivation returns the stale PID every time.
	for i := 0; i < 5; i++ {
		rec := il.getExistingActivation(ctx, ci)
		require.NotNil(t, rec, "stale activation persists on attempt %d", i+1)
		assert.Equal(t, "dead-host:9999", rec.PidAddress)
	}
}
