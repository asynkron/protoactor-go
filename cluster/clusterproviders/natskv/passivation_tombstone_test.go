package natskv

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
)

// The passivation sites -- removeActivation (the placement actor's
// RemoveActivation callback) and RemovePid -- are the DOMINANT source of
// identity delete markers: every ordinary grain passivation goes through
// removeActivation, while casDelete only runs on janitor/cleanup reaps. Both
// are CAS deletes structurally identical to casDelete's, so both must leave the
// same expiring purge marker; otherwise the bucket keeps growing with every
// identity ever activated and the change looks ineffective at soak.

// seedActivation writes an activation record for ci pointing at pid, the shape
// both passivation sites read back and PID-match before deleting.
func seedActivation(
	t *testing.T,
	il *IdentityLookup,
	ci *cluster.ClusterIdentity,
	pid *actor.PID,
	memberID string,
) uint64 {
	t.Helper()

	data, err := json.Marshal(&activationRecord{
		PidID:      pid.Id,
		PidAddress: pid.Address,
		MemberID:   memberID,
	})
	require.NoError(t, err)

	rev, err := il.identities.Put(context.Background(), kvKey(ci), data)
	require.NoError(t, err)

	return rev
}

// markerHeaders returns the headers of whatever is currently stored on ci's
// subject in the identities bucket.
func markerHeaders(t *testing.T, env *natskvTestEnv, ci *cluster.ClusterIdentity) nats.Header {
	t.Helper()

	return lastMsgHeaders(t, env.js, "KV_"+env.identityBucket, "$KV."+env.identityBucket+"."+kvKey(ci))
}

// revBumpingKV re-writes a key's own value immediately after every Get, so the
// revision the caller carries away is already stale by the time it CASes on it.
// The value is unchanged, so the PID-match check still passes and the site
// under test still reaches its delete. This is the only way to drive the
// passivation sites' CAS guard from outside: unlike casDelete they read their
// own revision rather than taking one as an argument.
type revBumpingKV struct {
	jetstream.KeyValue

	bumped int
}

func (k *revBumpingKV) Get(ctx context.Context, key string) (jetstream.KeyValueEntry, error) {
	entry, err := k.KeyValue.Get(ctx, key)
	if err != nil {
		return entry, err
	}

	// Put is not overridden here, so this reaches the real bucket.
	if _, putErr := k.Put(ctx, key, entry.Value()); putErr != nil {
		return nil, putErr
	}

	k.bumped++

	return entry, nil
}

// TestRemoveActivation_WritesExpiringPurgeMarker: normal grain passivation must
// leave a marker that expires, not one retained forever.
func TestRemoveActivation_WritesExpiringPurgeMarker(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, env := newIdentityLookupForTest(t, withMarkerTTLEnabled(true))
	ci := testCI("TestKind", "passivated")
	pid := actor.NewPID("host-a:8080", "TestKind/passivated")

	seedActivation(t, il, ci, pid, "member-A")

	require.NoError(t, il.removeActivation(ctx, ci, pid))

	_, err := il.identities.Get(ctx, kvKey(ci))
	require.ErrorIs(t, err, jetstream.ErrKeyNotFound, "the passivated record must be gone")

	require.Equal(t, 1, streamMsgCount(t, env.js, "KV_"+env.identityBucket),
		"purge must collapse the subject to a single marker")

	hdr := markerHeaders(t, env, ci)
	assert.Equal(t, "PURGE", hdr.Get("KV-Operation"))
	assert.Equal(t, "sub", hdr.Get("Nats-Rollup"))
	assert.Equal(t, defaultTombstoneTTL.String(), hdr.Get("Nats-TTL"),
		"the passivation marker must carry the configured TombstoneTTL")
}

// TestRemoveActivation_StaleRevisionRejected_WritesNoMarker: the switch to
// Purge must not weaken the CAS guard. A record rewritten between the read and
// the delete belongs to a newer writer; the delete must miss, leave the record
// intact, write NO marker, and still be reported as benign (the site filters
// ErrKeyExists, which a rejected purge must keep returning).
func TestRemoveActivation_StaleRevisionRejected_WritesNoMarker(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, env := newIdentityLookupForTest(t, withMarkerTTLEnabled(true))
	ci := testCI("TestKind", "raced")
	pid := actor.NewPID("host-a:8080", "TestKind/raced")

	seedActivation(t, il, ci, pid, "member-A")

	real := il.identities
	racer := &revBumpingKV{KeyValue: real}
	il.identities = racer

	require.NoError(t, il.removeActivation(ctx, ci, pid),
		"a CAS miss is benign here: the record belongs to a newer writer now")
	require.Equal(t, 1, racer.bumped, "the fixture must actually have raced the delete")

	entry, err := real.Get(ctx, kvKey(ci))
	require.NoError(t, err, "a stale-revision purge must not remove the key")

	var rec activationRecord

	require.NoError(t, json.Unmarshal(entry.Value(), &rec))
	assert.Equal(t, pid.Id, rec.PidID, "the surviving record must be intact")

	hdr := markerHeaders(t, env, ci)
	assert.Empty(t, hdr.Get("KV-Operation"), "no marker may be written by a rejected purge")
	assert.Empty(t, hdr.Get("Nats-Rollup"), "a rejected purge must not roll the subject up")
}

// TestRemoveActivation_MarkerTTLDisabled_DeletesWithPermanentMarker: on a
// bucket that fell back, the site must still delete. Purge stamps Nats-TTL only
// when the flag says the bucket can take it; an unconditional PurgeTTL here
// would be rejected with 10166 and the passivated identity would never be
// removed.
func TestRemoveActivation_MarkerTTLDisabled_DeletesWithPermanentMarker(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, env := newIdentityLookupForTest(t, withBucketMarkerTTL(0), withMarkerTTLEnabled(false))
	ci := testCI("TestKind", "fallback")
	pid := actor.NewPID("host-a:8080", "TestKind/fallback")

	require.Positive(t, il.config.TombstoneTTL, "only the BUCKET fell back; the knob stays positive")

	seedActivation(t, il, ci, pid, "member-A")

	require.NoError(t, il.removeActivation(ctx, ci, pid),
		"passivation must succeed where per-message TTLs are disabled")

	_, err := il.identities.Get(ctx, kvKey(ci))
	require.ErrorIs(t, err, jetstream.ErrKeyNotFound)

	hdr := markerHeaders(t, env, ci)
	assert.Equal(t, "PURGE", hdr.Get("KV-Operation"))
	assert.Empty(t, hdr.Get("Nats-TTL"), "a fallback bucket's marker cannot carry an expiry")
}

// TestRemovePid_WritesExpiringPurgeMarker: the same for the dead-letter retry
// path, which deletes an identity record whose PID is confirmed gone.
func TestRemovePid_WritesExpiringPurgeMarker(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, env := newIdentityLookupForTest(t, withMarkerTTLEnabled(true))
	ci := testCI("TestKind", "removed-pid")
	pid := actor.NewPID("host-a:8080", "TestKind/removed-pid")

	seedActivation(t, il, ci, pid, "member-A")

	il.RemovePid(ci, pid)

	_, err := il.identities.Get(ctx, kvKey(ci))
	require.ErrorIs(t, err, jetstream.ErrKeyNotFound, "the record must be gone")

	require.Equal(t, 1, streamMsgCount(t, env.js, "KV_"+env.identityBucket))

	hdr := markerHeaders(t, env, ci)
	assert.Equal(t, "PURGE", hdr.Get("KV-Operation"))
	assert.Equal(t, "sub", hdr.Get("Nats-Rollup"))
	assert.Equal(t, defaultTombstoneTTL.String(), hdr.Get("Nats-TTL"))
}

// TestRemovePid_StaleRevisionRejected_WritesNoMarker mirrors the
// removeActivation case: RemovePid must not blind-delete a record that was
// re-activated between its read and its delete.
func TestRemovePid_StaleRevisionRejected_WritesNoMarker(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, env := newIdentityLookupForTest(t, withMarkerTTLEnabled(true))
	ci := testCI("TestKind", "removed-raced")
	pid := actor.NewPID("host-a:8080", "TestKind/removed-raced")

	seedActivation(t, il, ci, pid, "member-A")

	real := il.identities
	racer := &revBumpingKV{KeyValue: real}
	il.identities = racer

	il.RemovePid(ci, pid)

	require.Equal(t, 1, racer.bumped, "the fixture must actually have raced the delete")

	_, err := real.Get(ctx, kvKey(ci))
	require.NoError(t, err, "a stale-revision purge must not remove the key")

	hdr := markerHeaders(t, env, ci)
	assert.Empty(t, hdr.Get("KV-Operation"), "no marker may be written by a rejected purge")
}

// TestRemovePid_MarkerTTLDisabled_DeletesWithPermanentMarker: the fallback
// shape at the RemovePid site.
func TestRemovePid_MarkerTTLDisabled_DeletesWithPermanentMarker(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, env := newIdentityLookupForTest(t, withBucketMarkerTTL(0), withMarkerTTLEnabled(false))
	ci := testCI("TestKind", "removed-fallback")
	pid := actor.NewPID("host-a:8080", "TestKind/removed-fallback")

	require.Positive(t, il.config.TombstoneTTL)

	seedActivation(t, il, ci, pid, "member-A")

	il.RemovePid(ci, pid)

	_, err := il.identities.Get(ctx, kvKey(ci))
	require.ErrorIs(t, err, jetstream.ErrKeyNotFound,
		"RemovePid must succeed where per-message TTLs are disabled")

	hdr := markerHeaders(t, env, ci)
	assert.Equal(t, "PURGE", hdr.Get("KV-Operation"))
	assert.Empty(t, hdr.Get("Nats-TTL"))
}

// TestRejectedPurge_ClassifiesAsCASConflict pins the two error-shape facts the
// switch from Delete to Purge depends on at every site: a rejected purge still
// carries JSErrCodeStreamWrongLastSequence (so classifyWriteError still calls
// it benign and the write-failure watchdog does not trip on ordinary contention),
// and it still matches jetstream.ErrKeyExists (so removeActivation's filter
// still swallows it instead of returning it to the placement actor).
func TestRejectedPurge_ClassifiesAsCASConflict(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, _ := newIdentityLookupForTest(t, withMarkerTTLEnabled(true))
	key := kvKey(testCI("TestKind", "classify"))

	staleRev, err := il.identities.Put(ctx, key, []byte(`{"pidId":"p1"}`))
	require.NoError(t, err)

	_, err = il.identities.Put(ctx, key, []byte(`{"pidId":"p2"}`))
	require.NoError(t, err)

	err = il.identities.Purge(ctx, key, il.tombstoneOpts(staleRev)...)
	require.Error(t, err, "the CAS guard must still reject a stale revision through Purge")

	var apiErr *jetstream.APIError

	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, jetstream.JSErrCodeStreamWrongLastSequence, apiErr.ErrorCode,
		"classifyWriteError keys on this code, not on the operation")
	assert.Equal(t, writeCASConflict, classifyWriteError(err, true),
		"a rejected purge must still classify as a benign CAS conflict")
	assert.True(t, errors.Is(err, jetstream.ErrKeyExists),
		"removeActivation filters ErrKeyExists; a rejected purge must keep matching it")
}

// TestRemoveActivation_SkipsDeleteOnPidMismatch guards the behaviour the
// extraction of the callback into a method must not change: a record that now
// points at a different PID belongs to a newer activation and must survive.
func TestRemoveActivation_SkipsDeleteOnPidMismatch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, env := newIdentityLookupForTest(t, withMarkerTTLEnabled(true))
	ci := testCI("TestKind", "reactivated")

	seedActivation(t, il, ci, actor.NewPID("host-b:9090", "TestKind/reactivated"), "member-B")

	stalePid := actor.NewPID("host-a:8080", "TestKind/reactivated")
	require.NoError(t, il.removeActivation(ctx, ci, stalePid))

	entry, err := il.identities.Get(ctx, kvKey(ci))
	require.NoError(t, err, "node B's activation must survive a stale RemoveActivation")

	var rec activationRecord

	require.NoError(t, json.Unmarshal(entry.Value(), &rec))
	assert.Equal(t, "host-b:9090", rec.PidAddress)

	hdr := markerHeaders(t, env, ci)
	assert.Empty(t, hdr.Get("KV-Operation"), "a skipped delete must not write a marker")
}

// TestRemoveActivation_AbsentKeyIsNoError: nothing to passivate is not a
// failure, and must not become one when the delete changes shape.
func TestRemoveActivation_AbsentKeyIsNoError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, _ := newIdentityLookupForTest(t)
	ci := testCI("TestKind", "never-activated")

	require.NoError(t, il.removeActivation(ctx, ci, actor.NewPID("host-a:8080", "x")))
}
