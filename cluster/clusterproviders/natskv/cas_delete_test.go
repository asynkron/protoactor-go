package natskv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCASDeleteMissIsTerminalNoOp verifies that a stale caller (holding an
// old revision) cannot delete a record that was rewritten after its read.
// This covers the site-468 persist-already-succeeded interleaving: if
// PersistActivation succeeds and then the caller deletes with the lock
// revision, the live activation must survive.
func TestCASDeleteMissIsTerminalNoOp(t *testing.T) {
	il := buildBareIdentityLookup(t)
	ctx := context.Background()
	ci := testCI("k", "id")

	_, lockRev, _ := il.tryAcquireLock(ctx, ci)
	if err := il.storeActivation(ctx, ci, "lid", lockRev, "m2", "addr2", "pid2"); err != nil {
		t.Fatal(err)
	}

	// casDelete with the now-stale lock revision must be a terminal no-op.
	il.casDelete(ctx, kvKey(ci), lockRev, "test")

	entry, err := il.identities.Get(ctx, kvKey(ci))
	if err != nil {
		t.Fatalf("record was deleted by a stale CAS: %v", err)
	}
	var rec activationRecord
	_ = json.Unmarshal(entry.Value(), &rec)
	if rec.PidID != "pid2" {
		t.Errorf("activation lost: %+v", rec)
	}
}

// TestGetExistingActivationWithRevReturnsRevision verifies that
// getExistingActivationWithRev returns the current KV revision along with
// the activation record.
func TestGetExistingActivationWithRevReturnsRevision(t *testing.T) {
	il := buildBareIdentityLookup(t)
	ctx := context.Background()
	ci := testCI("k", "id")

	_, rev, _ := il.tryAcquireLock(ctx, ci)
	_ = il.storeActivation(ctx, ci, "lid", rev, "m1", "a", "p")

	rec, gotRev := il.getExistingActivationWithRev(ctx, ci)
	if rec == nil || gotRev == 0 {
		t.Fatal("expected record + revision")
	}

	entry, _ := il.identities.Get(ctx, kvKey(ci))
	if gotRev != entry.Revision() {
		t.Errorf("rev %d != %d", gotRev, entry.Revision())
	}
}

// TestConcurrentStoreVsReapResolvesViaCAS verifies the CAS semantics for
// concurrent store vs delete: exactly one succeeds; the result is either a
// live activation or an absent key, never a corruption.
func TestConcurrentStoreVsReapResolvesViaCAS(t *testing.T) {
	il := buildBareIdentityLookup(t)
	ctx := context.Background()
	ci := testCI("k", "concurrent")

	_, lockRev, _ := il.tryAcquireLock(ctx, ci)

	// Goroutine A: store activation.
	storeErr := make(chan error, 1)
	go func() {
		storeErr <- il.storeActivation(ctx, ci, "lid", lockRev, "m1", "addr1", "pid1")
	}()

	// Goroutine B: casDelete on the same lock revision.
	deleteResult := make(chan struct{}, 1)
	go func() {
		il.casDelete(ctx, kvKey(ci), lockRev, "concurrent-test")
		deleteResult <- struct{}{}
	}()

	<-storeErr
	<-deleteResult

	// Post-condition: the key must be in a consistent state.
	// Either storeActivation won (key contains the activation) or casDelete
	// won (key is absent). No corruption is acceptable.
	entry, err := il.identities.Get(ctx, kvKey(ci))
	if err != nil {
		// casDelete won -- key is absent. Valid.
		return
	}
	// storeActivation won. Verify the record is coherent.
	var rec activationRecord
	if jsonErr := json.Unmarshal(entry.Value(), &rec); jsonErr != nil {
		t.Errorf("key present but unmarshal failed: %v", jsonErr)
	}
}

// TestRemoveMemberIDCannotDeleteUnreadRecords verifies that after the
// tracking read, if a key is rewritten by a new activation from a different
// member, removeMemberID must leave it intact because its CAS is against
// the pre-rewrite revision.
func TestRemoveMemberIDCannotDeleteUnreadRecords(t *testing.T) {
	il := buildBareIdentityLookup(t)
	ctx := context.Background()
	ci := testCI("k", "race")
	key := kvKey(ci)

	// Activate under member m1 so m1 tracks key.
	_, lockRev, _ := il.tryAcquireLock(ctx, ci)
	if err := il.storeActivation(ctx, ci, "lid", lockRev, "test-member-bare", "addr1", "pid1"); err != nil {
		t.Fatalf("storeActivation: %v", err)
	}
	// At this point addKeyToMember was called inside storeActivation.

	// Now overwrite with a new activation from m2 (higher revision).
	entry, _ := il.identities.Get(ctx, key)
	newRev := entry.Revision()

	// Simulate m2 writing a fresh activation at the current revision.
	if err := il.storeActivation(ctx, ci, "lid2", newRev, "member-m2", "addr2", "pid2"); err != nil {
		t.Fatalf("storeActivation m2: %v", err)
	}

	// removeMemberID for "test-member-bare" should see key still present but
	// its CAS (based on the old read-time revision) must miss and leave the
	// new m2 activation intact.
	il.removeMemberID(ctx, "test-member-bare")

	// Key must still be present with m2's data.
	finalEntry, err := il.identities.Get(ctx, key)
	if err != nil {
		t.Fatalf("key was deleted; m2 activation was wrongly removed: %v", err)
	}
	var rec activationRecord
	_ = json.Unmarshal(finalEntry.Value(), &rec)
	if rec.MemberID != "member-m2" {
		t.Errorf("expected member-m2 activation; got %+v", rec)
	}
}

// TestCasDelete_WritesExpiringPurgeMarker: a Delete leaves a marker that lives
// forever, so the identities bucket grew monotonically with every identity ever
// activated. Purge with a TTL collapses the subject to one marker that expires
// on its own.
func TestCasDelete_WritesExpiringPurgeMarker(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, env := newIdentityLookupForTest(t)
	key := kvKey(testCI("TestKind", "marker-grain"))

	rev, err := il.identities.Put(ctx, key, []byte(`{"pid":"p1"}`))
	require.NoError(t, err)

	il.casDelete(ctx, key, rev, "test")

	_, err = il.identities.Get(ctx, key)
	require.ErrorIs(t, err, jetstream.ErrKeyNotFound, "the record must be gone")

	entries := streamMsgCount(t, env.js, "KV_"+env.identityBucket)
	require.Equal(t, 1, entries, "purge must collapse the subject to a single marker")

	hdr := lastMsgHeaders(t, env.js, "KV_"+env.identityBucket, "$KV."+env.identityBucket+"."+key)
	assert.Equal(t, "PURGE", hdr.Get("KV-Operation"))
	assert.Equal(t, "sub", hdr.Get("Nats-Rollup"))
	assert.NotEmpty(t, hdr.Get("Nats-TTL"), "the marker must carry its own expiry")
}

// TestCasDelete_PurgeWithLastRevision_RejectsStaleRevision pins that the rollup
// does not defeat the CAS guard. casDelete must never blind-delete: a key
// modified after our read belongs to somebody else now. Both LastRevision and
// PurgeTTL are KVDeleteOpt, and the server evaluates the expected-last-subject
// -sequence header before it stores anything, so a rejected purge never gets to
// roll the subject up.
func TestCasDelete_PurgeWithLastRevision_RejectsStaleRevision(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, env := newIdentityLookupForTest(t)
	key := kvKey(testCI("TestKind", "stale-grain"))

	staleRev, err := il.identities.Put(ctx, key, []byte(`{"pid":"p1"}`))
	require.NoError(t, err)

	_, err = il.identities.Put(ctx, key, []byte(`{"pid":"p2"}`))
	require.NoError(t, err)

	il.casDelete(ctx, key, staleRev, "test")

	entry, err := il.identities.Get(ctx, key)
	require.NoError(t, err, "a stale-revision purge must not remove the key")
	assert.JSONEq(t, `{"pid":"p2"}`, string(entry.Value()), "the newer write must survive intact")

	hdr := lastMsgHeaders(t, env.js, "KV_"+env.identityBucket, "$KV."+env.identityBucket+"."+key)
	assert.Empty(t, hdr.Get("KV-Operation"), "no marker may be written by a rejected purge")
}

// TestTombstoneOpts_GatedByMarkerTTLFlag pins that the PurgeTTL option is
// governed by whether the bucket REALLY got marker TTLs, not by the configured
// TombstoneTTL alone. Both lookups here carry the same positive TombstoneTTL;
// only the flag differs.
func TestTombstoneOpts_GatedByMarkerTTLFlag(t *testing.T) {
	t.Parallel()

	off, _ := newIdentityLookupForTest(t, withMarkerTTLEnabled(false))
	on, _ := newIdentityLookupForTest(t, withMarkerTTLEnabled(true))

	require.Positive(t, off.config.TombstoneTTL, "the configured TTL is not what gates the option")
	require.Len(t, off.tombstoneOpts(1), 1,
		"a bucket that fell back gets the CAS guard only; a PurgeTTL would fail every delete")
	require.Len(t, on.tombstoneOpts(1), 2, "the CAS guard plus PurgeTTL")
}

// TestCasDelete_OnServerWithoutMarkerTTL_StillDeletes is the guard on the
// fallback path itself. Purge stamps Nats-TTL, and nats-server rejects a
// TTL header on a stream with AllowMsgTTL: false (server/stream.go:6358,
// JSMessageTTLDisabledErr 10166) -- so an unconditional PurgeTTL would make
// EVERY identity delete fail on exactly the servers the fallback exists for.
//
// The bucket here is created through the production helper with no TTL, so its
// stream genuinely carries AllowMsgTTL: false -- the same shape a pre-API-
// level-1 server would have produced. The rejection is asserted directly
// before the delete is, so the hazard is proven real rather than assumed.
func TestCasDelete_OnServerWithoutMarkerTTL_StillDeletes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	il, _ := newIdentityLookupForTest(t, withTombstoneTTL(0), withMarkerTTLEnabled(false))

	// The hazard, demonstrated on this exact bucket.
	hazardRev, err := il.identities.Put(ctx, "kind/hazard", []byte(`{"pidId":"p0"}`))
	require.NoError(t, err)

	err = il.identities.Purge(ctx, "kind/hazard",
		jetstream.LastRevision(hazardRev), jetstream.PurgeTTL(time.Hour))
	require.Error(t, err, "an unconditional PurgeTTL must be rejected here")

	var apiErr *jetstream.APIError

	require.ErrorAs(t, err, &apiErr)
	// nats.go names no constant for it; 10166 is JSMessageTTLDisabledErr
	// (nats-server server/jetstream_errors_generated.go), "per-message TTL is
	// disabled".
	assert.Equal(t, jetstream.ErrorCode(10166), apiErr.ErrorCode)
	assert.Contains(t, apiErr.Description, "per-message TTL is disabled")

	_, err = il.identities.Get(ctx, "kind/hazard")
	require.NoError(t, err, "the rejected purge left the record in place -- an identity leak")

	// What casDelete actually does on such a bucket.
	require.Len(t, il.tombstoneOpts(hazardRev), 1, "only the CAS guard may be attached")

	rev, err := il.identities.Put(ctx, "kind/ident", []byte(`{"pidId":"p1"}`))
	require.NoError(t, err)

	il.casDelete(ctx, "kind/ident", rev, "test")

	_, err = il.identities.Get(ctx, "kind/ident")
	require.ErrorIs(t, err, jetstream.ErrKeyNotFound,
		"the delete must succeed even where per-message TTLs are disabled")
}

// TestCasDelete_OnServerWithMarkerTTL_UsesPurgeTTL is the same test the other
// way round: where marker TTLs ARE supported, the marker must carry its expiry.
func TestCasDelete_OnServerWithMarkerTTL_UsesPurgeTTL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, env := newIdentityLookupForTest(t, withMarkerTTLEnabled(true))

	rev, err := il.identities.Put(ctx, "kind/ident", []byte(`{"pidId":"p1"}`))
	require.NoError(t, err)

	il.casDelete(ctx, "kind/ident", rev, "test")

	_, err = il.identities.Get(ctx, "kind/ident")
	require.ErrorIs(t, err, jetstream.ErrKeyNotFound)

	hdr := lastMsgHeaders(t, env.js, "KV_"+env.identityBucket, "$KV."+env.identityBucket+".kind/ident")
	assert.Equal(t, "PURGE", hdr.Get("KV-Operation"))
	assert.Equal(t, defaultTombstoneTTL.String(), hdr.Get("Nats-TTL"),
		"the marker's expiry must be the configured TombstoneTTL")
}

// TestIdentityBucket_MarkerCountBoundedAfterTTL: with a short TombstoneTTL, the
// retained message count returns to the live-key count instead of accumulating.
// This is the whole point of the change -- the hub bucket at soak held 4,856
// live keys plus 2,160 markers that could never be removed.
func TestIdentityBucket_MarkerCountBoundedAfterTTL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	// One second is the server's floor for SubjectDeleteMarkerTTL
	// (server/stream.go:1774), which is why minTombstoneTTL exists.
	il, env := newIdentityLookupForTest(t, withTombstoneTTL(minTombstoneTTL))
	stream := "KV_" + env.identityBucket

	const liveKeys, deletedKeys = 3, 5

	for i := range liveKeys {
		_, err := il.identities.Put(ctx, kvKey(testCI("Live", strconv.Itoa(i))), []byte(`{"pid":"live"}`))
		require.NoError(t, err)
	}

	for i := range deletedKeys {
		key := kvKey(testCI("Dead", strconv.Itoa(i)))

		rev, err := il.identities.Put(ctx, key, []byte(`{"pid":"dead"}`))
		require.NoError(t, err)

		il.casDelete(ctx, key, rev, "test")
	}

	require.Equal(t, liveKeys+deletedKeys, streamMsgCount(t, env.js, stream),
		"immediately after the deletes, one marker per deleted key is retained")

	require.Eventually(t, func() bool {
		return streamMsgCount(t, env.js, stream) == liveKeys
	}, 30*time.Second, 250*time.Millisecond,
		"markers must expire server-side back to the live-key count")

	for i := range liveKeys {
		_, err := il.identities.Get(ctx, kvKey(testCI("Live", strconv.Itoa(i))))
		require.NoError(t, err, "expiring markers must not touch live records")
	}
}

// TestJanitorSweep_IssuesNoStreamPurgeRequests is the N+1 guard: PurgeDeletes
// issues one $JS.API.STREAM.PURGE per delete marker (nats.go kv.go:1528-1580),
// which on the hub bucket at soak is 2,160 sequential round trips. The sweep
// must issue zero -- the marker lifecycle is the server's job now, not a
// periodic client-side scan.
func TestJanitorSweep_IssuesNoStreamPurgeRequests(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, env := newIdentityLookupForTest(t, withMemberBucket())

	// The reap branch needs a members bucket that ANSWERS and does not contain
	// "departed-member". A provider with no members bucket at all is not that
	// shape: the sweep reads a missing answer as missing information and reaps
	// nothing, precisely so a lost bucket handle cannot reap the whole
	// identities bucket. So the bucket exists and holds one live member.
	_, err := env.memberBucket.Put(ctx, env.provider.memberKey("live-member"), []byte(`{}`))
	require.NoError(t, err)

	const activations = 4

	for i := range activations {
		rec, marshalErr := json.Marshal(&activationRecord{
			PidID:      "Dead/grain-" + strconv.Itoa(i),
			PidAddress: "127.0.0.1:1",
			MemberID:   "departed-member",
		})
		require.NoError(t, marshalErr)

		_, putErr := il.identities.Put(ctx, kvKey(testCI("Dead", strconv.Itoa(i))), rec)
		require.NoError(t, putErr)
	}

	counter := newJSAPICounter(t, env.conn)

	// The absent-member branch needs two observations AND the grace elapsed,
	// so the sweep runs twice against a clock the test advances.
	now := time.Now()
	il.now = func() time.Time { return now }

	ac := &janitorAbsenceClock{}
	il.janitorSweep(ctx, ac)

	now = now.Add(2 * il.config.ActivationAbsentGrace)
	il.janitorSweep(ctx, ac)

	require.NoError(t, env.conn.Flush())

	assert.Zero(t, counter.countPrefix(t, "$JS.API.STREAM.PURGE."),
		"the sweep must not purge the stream once per marker")

	// The sweep must actually have done the work whose cost is being measured.
	for i := range activations {
		_, getErr := il.identities.Get(ctx, kvKey(testCI("Dead", strconv.Itoa(i))))
		require.ErrorIs(t, getErr, jetstream.ErrKeyNotFound, "the sweep must have reaped the record")
	}

	assert.Equal(t, activations, streamMsgCount(t, env.js, "KV_"+env.identityBucket),
		"each reap leaves exactly one rolled-up marker")
}

// TestCreateBucketWithMarkerTTL_UnsupportedFallsBackAndWarns: a server below
// JetStream API level 1 returns ErrLimitMarkerTTLNotSupported from
// CreateOrUpdateKeyValue (nats.go jetstream/kv.go:658-668). Setup must degrade
// to today's no-TTL bucket, not fail -- the cluster provider failing to start is
// far worse than tombstones accumulating -- and it must REPORT that it degraded,
// because casDelete's PurgeTTL depends on the answer.
//
// The unsupported server is simulated by a jetstream.JetStream stub whose
// CreateOrUpdateKeyValue returns ErrLimitMarkerTTLNotSupported for a config
// carrying LimitMarkerTTL and succeeds for one that does not; the embedded
// server advertises API level 4 and supports marker TTLs, so the failure cannot
// be provoked directly.
func TestCreateBucketWithMarkerTTL_UnsupportedFallsBackAndWarns(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	srv := startEmbeddedNATS(t)
	_, real := connectNATS(t, srv)

	js := &ttlRejectingJS{JetStream: real}

	var buf lockedBuffer

	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	kv, enabled, err := createBucketWithMarkerTTL(ctx, js, jetstream.KeyValueConfig{
		Bucket:   "test_fallback_bucket",
		Replicas: 1,
	}, time.Hour, logger)

	require.NoError(t, err, "an unsupported server must not fail Setup")
	require.NotNil(t, kv)
	require.False(t, enabled, "the caller must learn the TTL was NOT enabled")
	require.Equal(t, 1, js.rejected, "the TTL config was attempted exactly once")
	require.Equal(t, 1, js.plainAttempts, "and the ladder's second rung was taken exactly once")

	require.Contains(t, buf.String(), "does not support KV marker TTLs")

	status, err := kv.Status(ctx)
	require.NoError(t, err)
	require.Zero(t, status.LimitMarkerTTL(), "the fallback bucket carries no marker TTL")
}

// TestCreateBucketWithMarkerTTL_RealFailureIsFatal pins the other half of the
// fallback's contract: only ErrLimitMarkerTTLNotSupported degrades. Any other
// error is a genuine Setup failure (NATS down, bad config) and must propagate,
// because silently continuing past it would hide a broken write path.
func TestCreateBucketWithMarkerTTL_RealFailureIsFatal(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	srv := startEmbeddedNATS(t)
	_, real := connectNATS(t, srv)

	kv, enabled, err := createBucketWithMarkerTTL(ctx, real, jetstream.KeyValueConfig{
		Bucket:   "not a valid bucket name",
		Replicas: 1,
	}, time.Hour, discardLogger())

	require.Error(t, err)
	require.Nil(t, kv)
	require.False(t, enabled)
}

// TestCasDelete_FallbackShape_DeletesWithPermanentMarker is the true fallback
// shape end-to-end: a bucket whose stream really carries AllowMsgTTL: false
// while the configured TombstoneTTL stays POSITIVE. That is the only shape in
// which tombstoneOpts' markerTTLEnabled term is load-bearing -- with the TTL
// knob also zeroed, the `TombstoneTTL > 0` term alone would suppress PurgeTTL
// and dropping the flag from the gate would go unnoticed. Here dropping it
// makes the server reject the purge with 10166 and the identity leaks.
func TestCasDelete_FallbackShape_DeletesWithPermanentMarker(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, env := newIdentityLookupForTest(t, withBucketMarkerTTL(0), withMarkerTTLEnabled(false))

	require.Positive(t, il.config.TombstoneTTL,
		"only the BUCKET fell back; the configured knob stays positive")

	status, err := il.identities.Status(ctx)
	require.NoError(t, err)
	require.Zero(t, status.LimitMarkerTTL(), "the bucket must genuinely lack marker TTLs")

	key := kvKey(testCI("TestKind", "fallback-shape"))

	rev, err := il.identities.Put(ctx, key, []byte(`{"pidId":"p1"}`))
	require.NoError(t, err)

	il.casDelete(ctx, key, rev, "test")

	_, err = il.identities.Get(ctx, key)
	require.ErrorIs(t, err, jetstream.ErrKeyNotFound,
		"the delete must succeed: a PurgeTTL here would be rejected with 10166 and leak the identity")

	hdr := lastMsgHeaders(t, env.js, "KV_"+env.identityBucket, "$KV."+env.identityBucket+"."+key)
	assert.Equal(t, "PURGE", hdr.Get("KV-Operation"))
	assert.Equal(t, "sub", hdr.Get("Nats-Rollup"))
	assert.Empty(t, hdr.Get("Nats-TTL"),
		"a bucket without marker TTLs cannot carry an expiry -- the marker is permanent, as before")

	assert.Equal(t, 1, streamMsgCount(t, env.js, "KV_"+env.identityBucket),
		"the fallback keeps exactly the old behaviour: one retained marker")
}

// TestCreateBucketWithMarkerTTL_NonCapabilityErrorDoesNotFallBack pins the
// half of the fallback contract that an invalid-bucket-name test cannot reach:
// a name the server rejects fails BOTH attempts, so removing the
// ErrLimitMarkerTTLNotSupported check would still surface an error. Here only
// the TTL-carrying attempt fails, and it fails with something that is NOT the
// capability error -- the helper must propagate it and never try the second
// rung, because a fallback that hid a dead NATS connection would hide a broken
// identity write path.
func TestCreateBucketWithMarkerTTL_NonCapabilityErrorDoesNotFallBack(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	srv := startEmbeddedNATS(t)
	_, real := connectNATS(t, srv)

	js := &ttlRejectingJS{JetStream: real, ttlErr: errKVUnavailable}

	kv, enabled, err := createBucketWithMarkerTTL(ctx, js, jetstream.KeyValueConfig{
		Bucket:   "test_fatal_bucket",
		Replicas: 1,
	}, time.Hour, discardLogger())

	require.ErrorIs(t, err, errKVUnavailable, "a non-capability error must propagate unchanged")
	require.Nil(t, kv)
	require.False(t, enabled)
	require.Equal(t, 1, js.rejected)
	require.Zero(t, js.plainAttempts,
		"only ErrLimitMarkerTTLNotSupported degrades; anything else must fail Setup outright")
}

// errKVUnavailable stands in for a genuinely broken JetStream (dead connection,
// bad account) as opposed to one that merely lacks marker-TTL support.
var errKVUnavailable = errors.New("kv unavailable")

// ttlRejectingJS fails only the TTL-carrying bucket attempt and passes
// everything else through. With the zero-value ttlErr it reproduces a
// pre-API-level-1 server (ErrLimitMarkerTTLNotSupported); with any other error
// it reproduces a genuinely broken JetStream, which must NOT be swallowed by
// the fallback. plainAttempts counts the fallback attempts, so a test can pin
// whether the ladder was taken at all.
type ttlRejectingJS struct {
	jetstream.JetStream

	// ttlErr is returned for a config carrying LimitMarkerTTL. nil means
	// ErrLimitMarkerTTLNotSupported.
	ttlErr error

	rejected      int
	plainAttempts int
}

func (j *ttlRejectingJS) CreateOrUpdateKeyValue(
	ctx context.Context, cfg jetstream.KeyValueConfig,
) (jetstream.KeyValue, error) {
	if cfg.LimitMarkerTTL != 0 {
		j.rejected++

		if j.ttlErr != nil {
			return nil, j.ttlErr
		}

		return nil, jetstream.ErrLimitMarkerTTLNotSupported
	}

	j.plainAttempts++

	return j.JetStream.CreateOrUpdateKeyValue(ctx, cfg)
}

// lockedBuffer is a goroutine-safe io.Writer for capturing slog output.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}
