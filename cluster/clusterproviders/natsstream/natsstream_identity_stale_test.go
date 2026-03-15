package natsstream

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIdentityLookup_StaleActivation_BlocksReactivation demonstrates that
// when a node dies without cleaning up its identity claims in the NATS stream,
// repeated calls to Get() keep returning the stale (dead) PID. The only way
// to unblock reactivation is via RemovePid() — which is never called by
// DefaultContext.Request() in the current code.
func TestIdentityLookup_StaleActivation_BlocksReactivation(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-stale-block")

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	il := p.IdentityLookup()
	il.Setup(c, []string{"TestKind"}, false)

	time.Sleep(500 * time.Millisecond)

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "grain-1"}
	ctx := context.Background()

	// Simulate a stale activation left behind by a crashed node.
	// Publish directly to the identity stream to plant the stale record.
	subject := il.identitySubject(ci)
	staleRec := activationRecord{
		PidID:      "TestKind/grain-1",
		PidAddress: "dead-host:9999",
		MemberID:   "member-dead",
	}
	data, err := json.Marshal(&staleRec)
	require.NoError(t, err)
	_, err = p.js.Publish(ctx, subject, data)
	require.NoError(t, err)

	// Track it in the member's key tracking.
	il.addKeyToMember("member-dead", subject)

	// First call returns the stale PID.
	rec := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec, "stale activation should be returned")
	assert.Equal(t, "dead-host:9999", rec.PidAddress,
		"Get() returns the dead PID from the stale activation")

	// Second call also returns the stale PID — activation persists.
	rec2 := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec2, "stale activation should persist across calls")
	assert.Equal(t, "dead-host:9999", rec2.PidAddress)

	// Now call RemovePid — this is what DefaultContext SHOULD do on dead letter.
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
// negative test: without an explicit RemovePid() call, stale activations
// survive indefinitely in the NATS stream, blocking lock acquisition.
func TestIdentityLookup_StaleActivation_PersistsWithoutRemovePid(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-stale-persist")

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	il := p.IdentityLookup()
	il.Setup(c, []string{"TestKind"}, false)

	time.Sleep(500 * time.Millisecond)

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "grain-stuck"}
	ctx := context.Background()

	// Plant stale activation in the stream.
	subject := il.identitySubject(ci)
	staleRec := activationRecord{
		PidID:      "TestKind/grain-stuck",
		PidAddress: "dead-host:9999",
		MemberID:   "member-dead",
	}
	data, err := json.Marshal(&staleRec)
	require.NoError(t, err)
	_, err = p.js.Publish(ctx, subject, data)
	require.NoError(t, err)

	// Lock acquisition fails because the subject already has a message.
	_, _, ok := il.tryAcquireLock(ctx, ci)
	assert.False(t, ok,
		"lock acquisition must fail when a stale activation exists "+
			"in the stream — this is why RemovePid() is critical")

	// getExistingActivation returns the stale PID every time.
	for i := 0; i < 5; i++ {
		rec := il.getExistingActivation(ctx, ci)
		require.NotNil(t, rec, "stale activation persists on attempt %d", i+1)
		assert.Equal(t, "dead-host:9999", rec.PidAddress)
	}
}

// TestIdentityLookup_RemovePid_ClearsActivationAndTracking verifies that
// RemovePid removes both the activation from the identity stream AND the
// key from the member tracking map.
func TestIdentityLookup_RemovePid_ClearsActivationAndTracking(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-removepid-full")

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	il := p.IdentityLookup()
	il.Setup(c, []string{"TestKind"}, false)

	time.Sleep(500 * time.Millisecond)

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "grain-2"}
	ctx := context.Background()

	// Create a real activation via the normal lock+store path.
	lockID, seq, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok)
	err = il.storeActivation(ctx, ci, lockID, seq, "member-1", "127.0.0.1:8080", "TestKind/grain-2")
	require.NoError(t, err)

	// Verify activation exists.
	rec := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec)

	// Verify tracking.
	il.memberKeysMu.Lock()
	keys := il.memberKeys["member-1"]
	il.memberKeysMu.Unlock()
	require.NotEmpty(t, keys, "member-1 should have tracked keys")

	// RemovePid should clear both.
	pid := actor.NewPID("127.0.0.1:8080", "TestKind/grain-2")
	il.RemovePid(ci, pid)

	rec = il.getExistingActivation(ctx, ci)
	assert.Nil(t, rec, "activation should be removed after RemovePid")

	il.memberKeysMu.Lock()
	keys = il.memberKeys["member-1"]
	il.memberKeysMu.Unlock()

	subject := il.identitySubject(ci)
	for _, k := range keys {
		assert.NotEqual(t, subject, k,
			"member tracking should no longer include the removed identity")
	}
}
