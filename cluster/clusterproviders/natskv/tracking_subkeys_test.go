package natskv

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awevoke/protoactor-go/cluster"
)

// listTrackingKeys returns every key the tracking bucket holds for memberID,
// through the server-side filter the production read path uses.
func listTrackingKeys(t *testing.T, il *IdentityLookup, memberID string) []string {
	t.Helper()

	lister, err := il.memberTracker.ListKeysFiltered(context.Background(), trackingMemberFilter(memberID))
	require.NoError(t, err)

	out := make([]string, 0, 8)
	for k := range lister.Keys() {
		out = append(out, k)
	}

	return out
}

// TestTrackingSubKey_FiltersServerSide is the test that would have caught the
// '/' separator. '/' is not a NATS subject separator: ListKeysFiltered turns the
// filter into the consumer filter subject "$KV.<bucket>." + filter
// (nats.go/jetstream/kv.go WatchFiltered), and "memberID/>" is one LITERAL
// token, which matches nothing. With '.', "memberID.>" is a real wildcard.
func TestTrackingSubKey_FiltersServerSide(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, _ := newIdentityLookupForTest(t)

	il.addKeyToMember(ctx, "clusterA_node1", "proj-MarketList/default-abc")
	il.addKeyToMember(ctx, "clusterA_node1", "wk-Session/default-def")
	il.addKeyToMember(ctx, "clusterA_node2", "proj-MarketList/default-xyz")

	got := listTrackingKeys(t, il, "clusterA_node1")

	require.Len(t, got, 2,
		"the filter must be a server-side subject wildcard; with '/' it matches nothing")
	assert.ElementsMatch(t, []string{
		"clusterA_node1.proj-MarketList/default-abc",
		"clusterA_node1.wk-Session/default-def",
	}, got)
}

// TestTrackingKeyMember_SplitsOnTheFirstDot pins the halves the read path
// recovers. Identities legitimately contain dots (natskv_charset_test.go round
// -trips "a.b.c.d" and "v1.0.3=stable"), so the split must be on the FIRST dot,
// not the last and not every dot.
func TestTrackingKeyMember_SplitsOnTheFirstDot(t *testing.T) {
	t.Parallel()

	cases := []struct {
		key      string
		member   string
		identity string
		ok       bool
	}{
		{"clusterA_node1.wk-Session/default-abc", "clusterA_node1", "wk-Session/default-abc", true},
		{"clusterA_node1.MyKind/a.b.c.d", "clusterA_node1", "MyKind/a.b.c.d", true},
		{"clusterA_node1.MyKind/v1.0.3=stable", "clusterA_node1", "MyKind/v1.0.3=stable", true},
		// A legacy per-member record: the whole key IS the member id.
		{"clusterA_node1", "clusterA_node1", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			t.Parallel()

			member, identity, ok := trackingKeyMember(tc.key)

			assert.Equal(t, tc.member, member)
			assert.Equal(t, tc.identity, identity)
			assert.Equal(t, tc.ok, ok)
		})
	}
}

// TestMemberID_ContainsNoDot pins the assumption the '.' separator rests on. If
// a member id ever contained a dot, "memberID.>" would split at the wrong token
// and the filter would silently match the wrong set -- or nothing, and a legacy
// per-member record would be mistaken for an already-migrated sub-key.
//
// The value asserted is the one PRODUCTION builds: Setup does
// il.memberID = fmt.Sprintf("%s_%s", c.Config.Name, c.ActorSystem.ID), so this
// drives a real Setup rather than the fixture's canned string, which would pin
// nothing.
func TestMemberID_ContainsNoDot(t *testing.T) {
	_, c, il := setupPlacementTestCluster(t, "test-memberid-no-dot")

	require.NotEmpty(t, il.memberID)
	require.NotContains(t, il.memberID, ".",
		"a dot in the member id splits the tracking filter's first token")
	require.NotContains(t, c.ActorSystem.ID, ".",
		"the actor system id is the half of the member id this package does not choose")
}

// TestAddKeyToMember_IsSingleWrite: the tracking write was a
// Get -> unmarshal -> scan -> marshal -> CAS of a 262-393 KB JSON array on
// EVERY activation and passivation, 93% of spoke NATS ingress. A sub-key is
// one Put of an empty value.
//
// A KV Put is a JetStream PUBLISH to $KV.<bucket>.<key> (nats.go's kvs.Put ->
// js.Publish), which never crosses $JS.API.>; a KV Get is a DIRECT.GET, which
// does. So the write side is counted on the bucket's own subject and the read
// side on the API subject -- asserting a "$JS.API.STREAM.MSG.PUT." count would
// be zero by construction and could never pass.
func TestAddKeyToMember_IsSingleWrite(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, env := newIdentityLookupForTest(t)

	writes := newSubjectCounter(t, env.conn, "$KV."+env.trackingBucket+".>")
	api := newJSAPICounter(t, env.conn)

	writes.reset(t)
	api.reset(t)

	il.addKeyToMember(ctx, "clusterA_node1", "proj-MarketList/default-abc")

	if got := writes.count(t); got != 1 {
		t.Fatalf("KV writes for one addKeyToMember = %d, want exactly 1 (no read-modify-write)", got)
	}

	if got := api.countPrefix(t, "$JS.API.DIRECT.GET."); got != 0 {
		t.Fatalf("KV reads = %d, want 0 -- the sub-key write reads nothing", got)
	}

	if got := api.countPrefix(t, "$JS.API.STREAM.MSG.GET."); got != 0 {
		t.Fatalf("non-direct KV reads = %d, want 0", got)
	}
}

// TestRemoveKeyFromMember_IsSingleWrite is addKeyToMember's twin: passivation
// was the other half of the 262-393 KB read-modify-write.
func TestRemoveKeyFromMember_IsSingleWrite(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, env := newIdentityLookupForTest(t)

	il.addKeyToMember(ctx, "clusterA_node1", "proj-MarketList/default-abc")

	writes := newSubjectCounter(t, env.conn, "$KV."+env.trackingBucket+".>")
	api := newJSAPICounter(t, env.conn)

	writes.reset(t)
	api.reset(t)

	il.removeKeyFromMember(ctx, "clusterA_node1", "proj-MarketList/default-abc")

	if got := writes.count(t); got != 1 {
		t.Fatalf("KV writes for one removeKeyFromMember = %d, want exactly 1", got)
	}

	if got := api.countPrefix(t, "$JS.API.DIRECT.GET."); got != 0 {
		t.Fatalf("KV reads = %d, want 0 -- the sub-key delete reads nothing", got)
	}

	assert.Empty(t, listTrackingKeys(t, il, "clusterA_node1"),
		"the sub-key must be gone after removeKeyFromMember")
}

// TestRemoveKeyFromMember_WritesExpiringPurgeMarker: the sub-key delete runs at
// passivation rate (113/min on the spoke at soak). A plain Delete there would
// reintroduce the permanent-marker defect on the tracking bucket -- the exact
// class the tombstone TTL work removed from the identities bucket.
func TestRemoveKeyFromMember_WritesExpiringPurgeMarker(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, env := newIdentityLookupForTest(t)

	const (
		memberID = "clusterA_node1"
		key      = "wk-Session/default-abc"
	)

	il.addKeyToMember(ctx, memberID, key)
	il.removeKeyFromMember(ctx, memberID, key)

	headers := lastMsgHeaders(t, env.js, "KV_"+env.trackingBucket,
		"$KV."+env.trackingBucket+"."+trackingSubKey(memberID, key))

	assert.Equal(t, "PURGE", headers.Get("KV-Operation"),
		"a plain DEL marker is retained forever; PURGE + TTL expires")
	assert.Equal(t, "sub", headers.Get("Nats-Rollup"))
	assert.Equal(t, defaultTombstoneTTL.String(), headers.Get("Nats-TTL"),
		"the marker must carry the configured tombstone TTL")
}

// TestRemoveKeyFromMember_MarkerTTLDisabledOmitsTTL is the fallback shape: on a
// bucket whose stream has AllowMsgTTL: false the server REJECTS a Nats-TTL
// header, so attaching one unconditionally would make every passivation's
// tracking delete fail.
func TestRemoveKeyFromMember_MarkerTTLDisabledOmitsTTL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, env := newIdentityLookupForTest(t, withBucketMarkerTTL(0), withMarkerTTLEnabled(false))

	const (
		memberID = "clusterA_node1"
		key      = "wk-Session/default-abc"
	)

	il.addKeyToMember(ctx, memberID, key)
	il.removeKeyFromMember(ctx, memberID, key)

	headers := lastMsgHeaders(t, env.js, "KV_"+env.trackingBucket,
		"$KV."+env.trackingBucket+"."+trackingSubKey(memberID, key))

	assert.Equal(t, "PURGE", headers.Get("KV-Operation"))
	assert.Empty(t, headers.Get("Nats-TTL"),
		"a Nats-TTL on a bucket without AllowMsgTTL fails the delete outright")
}

// TestRemoveKeyFromMember_IsIdempotent pins the property that makes the
// revision-free delete correct: removing a sub-key that is not there is a
// success, not an error, and does not advance the write-failure streak.
func TestRemoveKeyFromMember_IsIdempotent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, _ := newIdentityLookupForTest(t)

	const (
		memberID = "clusterA_node1"
		key      = "wk-Session/default-abc"
	)

	il.addKeyToMember(ctx, memberID, key)
	il.removeKeyFromMember(ctx, memberID, key)
	il.removeKeyFromMember(ctx, memberID, key)
	il.removeKeyFromMember(ctx, memberID, "never-tracked/at-all")

	assert.Empty(t, listTrackingKeys(t, il, memberID))
	assert.Zero(t, il.streakLenForTest(),
		"an idempotent delete is not a write failure")
}

// TestConcurrentActivations_NoDroppedTrackingEntries: the CAS loop gave up
// after 3 attempts and silently returned, losing 13% of entries under
// concurrency. A single Put has nothing to lose.
func TestConcurrentActivations_NoDroppedTrackingEntries(t *testing.T) {
	t.Parallel()

	const n = 200

	ctx := context.Background()
	il, _ := newIdentityLookupForTest(t)

	var wg sync.WaitGroup

	for i := range n {
		wg.Add(1)

		go func() {
			defer wg.Done()

			il.addKeyToMember(ctx, "clusterA_node1", fmt.Sprintf("wk-Session/default-%d", i))
		}()
	}

	wg.Wait()

	assert.Len(t, listTrackingKeys(t, il, "clusterA_node1"),
		n, "every concurrent activation must be recorded")
}

// TestTrackingSubKeys_NoSizeCliffAt50kGrains: the legacy record approached the
// 1 MiB max_payload, whose failure mode is os.Exit(70). Sub-keys have no
// aggregate payload at all.
//
// Two halves, because the cliff and the fix are different kinds of fact. The
// first is behavioural: every tracking write the code issues is observed on the
// wire and its payload measured. The second is arithmetic on the legacy
// encoding at the 50k-grain scale the spoke actually reaches -- 50,000 real KV
// round trips would buy nothing the marshal does not already prove, and the
// quantity that matters is the SIZE of one write, which does not depend on how
// many of them happened.
func TestTrackingSubKeys_NoSizeCliffAt50kGrains(t *testing.T) {
	t.Parallel()

	const (
		liveWrites  = 500
		grains50k   = 50_000
		maxPayload  = 1024 * 1024
		memberID    = "clusterA_node1"
		keyTemplate = "wk-Session/default-%d"
	)

	ctx := context.Background()
	il, env := newIdentityLookupForTest(t)

	writes := newSubjectCounter(t, env.conn, "$KV."+env.trackingBucket+".>")
	writes.reset(t)

	for i := range liveWrites {
		il.addKeyToMember(ctx, memberID, fmt.Sprintf(keyTemplate, i))
	}

	require.Equal(t, liveWrites, writes.count(t), "one write per tracked grain, and only one")
	assert.Zero(t, writes.maxPayload(t),
		"the sub-key carries no value at all: the KEY is the fact")

	// The legacy encoding at spoke scale, for the record.
	keys := make([]string, grains50k)
	for i := range keys {
		keys[i] = fmt.Sprintf(keyTemplate, i)
	}

	legacy, err := json.Marshal(&memberRecord{Keys: keys})
	require.NoError(t, err)

	require.Greater(t, len(legacy), maxPayload,
		"the legacy per-member record at 50k grains exceeds the 1 MiB max_payload, "+
			"which is the cliff this change removes (%d bytes)", len(legacy))

	// The same 50k grains as sub-keys: the largest single write is one key.
	widest := 0

	for _, key := range keys {
		if n := len(trackingSubKey(memberID, key)); n > widest {
			widest = n
		}
	}

	assert.Less(t, widest, 1024,
		"the widest sub-key write at 50k grains is still a short subject and an empty body")
}

// TestListGrainsByMember_UsesServerSideFilter is the N+1 guard for the
// collector's 30s loop in EVERY process: one filtered enumeration, not a walk
// of the whole bucket.
func TestListGrainsByMember_UsesServerSideFilter(t *testing.T) {
	t.Parallel()

	il, env := newIdentityLookupForTest(t)

	const (
		memberA = "clusterA_node1"
		memberB = "clusterA_node2"
	)

	seedTrackedActivation(t, il, memberA, "wk-Session/a-1")
	seedTrackedActivation(t, il, memberA, "wk-Session/a-2")
	seedTrackedActivation(t, il, memberB, "wk-Session/b-1")

	consumers := newConsumerCreateRecorder(t, env.conn)
	consumers.reset(t)

	grains, err := il.ListGrainsByMember(memberA)
	require.NoError(t, err)
	require.Len(t, grains, 2, "only member A's grains")

	created := consumers.created(t)
	require.Len(t, created, 1,
		"one enumeration for the whole call: a per-member walk of the bucket would create more")

	filters := created[0].filters()
	require.Len(t, filters, 1)
	assert.Contains(t, filters[0], memberA,
		"the filter must scope to the member server-side")
	assert.True(t, strings.HasSuffix(filters[0], "."+memberA+".>"),
		"the filter subject must be $KV.<bucket>.<memberID>.> (got %q)", filters[0])
}

// TestListGrainsByMember_EmptyMemberIsNotAnError: under the legacy record a
// member that had hosted nothing produced ErrKeyNotFound, which every caller
// had to special-case. A filtered enumeration of nothing is simply empty.
func TestListGrainsByMember_EmptyMemberIsNotAnError(t *testing.T) {
	t.Parallel()

	il, _ := newIdentityLookupForTest(t)

	grains, err := il.ListGrainsByMember("clusterA_never_hosted_anything")

	require.NoError(t, err)
	assert.Empty(t, grains)
}

// TestListGrainsByMember_ToleratesLegacyRecord is half of the migration
// contract: a bucket written by the previous release must be readable by this
// one BEFORE the fan-out runs. Without this, every member's ByMember goes to
// zero the moment the new build starts -- and the cluster collector calls
// ByMember on a 30s loop in every process.
func TestListGrainsByMember_ToleratesLegacyRecord(t *testing.T) {
	t.Parallel()

	il, _ := newIdentityLookupForTest(t)

	const memberID = "clusterA_node1"

	writeIdentityRecord(t, il, memberID, "wk-Session/legacy-a")
	writeIdentityRecord(t, il, memberID, "wk-Session/legacy-b")
	writeLegacyMemberRecord(t, il, memberID, "wk-Session/legacy-a", "wk-Session/legacy-b")

	grains, err := il.ListGrainsByMember(memberID)
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"legacy-a", "legacy-b"}, identitiesOf(grains))
}

// TestListGrainsByMember_UnionsLegacyAndSubKeysWithoutDuplicates is the
// crash-window contract. A migration that dies between its first Put and its
// Purge leaves BOTH shapes in the bucket, with the sub-keys a subset of the
// legacy array and any grain activated since present only as a sub-key. The
// read path must return the union, and must return each grain once.
func TestListGrainsByMember_UnionsLegacyAndSubKeysWithoutDuplicates(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	il, _ := newIdentityLookupForTest(t)

	const memberID = "clusterA_node1"

	for _, id := range []string{"wk-Session/a", "wk-Session/b", "wk-Session/c"} {
		writeIdentityRecord(t, il, memberID, id)
	}

	// The legacy record the crashed migration was reading.
	writeLegacyMemberRecord(t, il, memberID, "wk-Session/a", "wk-Session/b")
	// The sub-keys it had already written, plus one activated afterwards.
	il.addKeyToMember(ctx, memberID, "wk-Session/a")
	il.addKeyToMember(ctx, memberID, "wk-Session/c")

	grains, err := il.ListGrainsByMember(memberID)
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"a", "b", "c"}, identitiesOf(grains),
		"the union of both shapes, and nothing lost")
}

// TestListGrains_UnionsBothShapesAcrossMembers is the whole-bucket twin of the
// per-member union.
func TestListGrains_UnionsBothShapesAcrossMembers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, _ := newIdentityLookupForTest(t)

	const (
		migrated = "clusterA_node1"
		legacy   = "clusterA_node2"
	)

	writeIdentityRecord(t, il, migrated, "wk-Session/new-1")
	il.addKeyToMember(ctx, migrated, "wk-Session/new-1")

	writeIdentityRecord(t, il, legacy, "wk-Session/old-1")
	writeLegacyMemberRecord(t, il, legacy, "wk-Session/old-1")

	grains, err := il.ListGrains()
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"new-1", "old-1"}, identitiesOf(grains))

	byMember := make(map[string]string, len(grains))
	for _, g := range grains {
		byMember[g.Identity] = g.MemberID
	}

	assert.Equal(t, migrated, byMember["new-1"])
	assert.Equal(t, legacy, byMember["old-1"], "a legacy key's member id is the key itself")
}

// TestListGrains_FanoutIsProportionalToLiveKeys documents a cost this task
// deliberately does NOT change: ListGrains issues one identities.Get per
// tracking key to build each GrainInfo. It is an operator enumeration, not a
// hot path -- the 30s collector loop calls ByMember, which this task reduces to
// O(that member's grains), and clustercollector.go caps it at 5000 anyway.
// The number is recorded here so it is on the record rather than assumed away.
//
// What DID change is the enumeration: ListGrains used to list the member keys
// and then enumerate once per member. It now enumerates the whole bucket once.
func TestListGrains_FanoutIsProportionalToLiveKeys(t *testing.T) {
	t.Parallel()

	const (
		perMember = 20
		members   = 2
		total     = perMember * members
	)

	ctx := context.Background()
	il, env := newIdentityLookupForTest(t)

	for m := range members {
		memberID := fmt.Sprintf("clusterA_node%d", m)

		for i := range perMember {
			key := fmt.Sprintf("wk-Session/m%d-%d", m, i)
			writeIdentityRecord(t, il, memberID, key)
			il.addKeyToMember(ctx, memberID, key)
		}
	}

	api := newJSAPICounter(t, env.conn)
	consumers := newConsumerCreateRecorder(t, env.conn)

	api.reset(t)
	consumers.reset(t)

	grains, err := il.ListGrains()
	require.NoError(t, err)
	require.Len(t, grains, total)

	gets := api.countPrefix(t, "$JS.API.DIRECT.GET.") + api.countPrefix(t, "$JS.API.STREAM.MSG.GET.")
	assert.Equal(t, total, gets,
		"one identities Get per live tracking key -- the retained, documented fan-out")

	assert.Len(t, consumers.created(t), 1,
		"one enumeration of the whole tracking bucket, not one per member")
}

// TestRemoveMemberID_RemovesIdentitiesAndTrackingSubKeys is member-departure
// cleanup. Under the legacy record one Delete removed all of a departed
// member's tracking state; under sub-keys the member's own sub-keys are what
// must go, or the tracking bucket grows with every member that ever crashed.
func TestRemoveMemberID_RemovesIdentitiesAndTrackingSubKeys(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, _ := newIdentityLookupForTest(t)

	const (
		gone      = "clusterA_node1"
		surviving = "clusterA_node2"
	)

	for _, id := range []string{"wk-Session/g-1", "wk-Session/g-2"} {
		writeIdentityRecord(t, il, gone, id)
		il.addKeyToMember(ctx, gone, id)
	}

	writeIdentityRecord(t, il, surviving, "wk-Session/s-1")
	il.addKeyToMember(ctx, surviving, "wk-Session/s-1")

	il.removeMemberID(ctx, gone)

	assert.Empty(t, listTrackingKeys(t, il, gone),
		"a departed member's tracking sub-keys must not outlive it")

	for _, id := range []string{"wk-Session/g-1", "wk-Session/g-2"} {
		_, err := il.identities.Get(ctx, id)
		assert.ErrorIs(t, err, jetstream.ErrKeyNotFound, "identity %q must be reaped", id)
	}

	assert.Len(t, listTrackingKeys(t, il, surviving), 1, "the surviving member is untouched")

	_, err := il.identities.Get(ctx, "wk-Session/s-1")
	assert.NoError(t, err, "the surviving member's identity is untouched")
}

// TestRemoveMemberID_ToleratesLegacyRecord: a member that departs before the
// migration reaches it still has to be cleaned up, from the legacy array.
func TestRemoveMemberID_ToleratesLegacyRecord(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, _ := newIdentityLookupForTest(t)

	const memberID = "clusterA_node1"

	writeIdentityRecord(t, il, memberID, "wk-Session/legacy-a")
	writeIdentityRecord(t, il, memberID, "wk-Session/legacy-b")
	writeLegacyMemberRecord(t, il, memberID, "wk-Session/legacy-a", "wk-Session/legacy-b")

	// One sub-key too: the half-migrated shape.
	il.addKeyToMember(ctx, memberID, "wk-Session/legacy-a")

	il.removeMemberID(ctx, memberID)

	for _, id := range []string{"wk-Session/legacy-a", "wk-Session/legacy-b"} {
		_, err := il.identities.Get(ctx, id)
		assert.ErrorIs(t, err, jetstream.ErrKeyNotFound, "identity %q must be reaped", id)
	}

	assert.Empty(t, listTrackingKeys(t, il, memberID))

	_, err := il.memberTracker.Get(ctx, memberID)
	assert.ErrorIs(t, err, jetstream.ErrKeyNotFound, "the legacy record goes with the member")
}

// TestTrackingMigration_LegacyRecordFannedOut: a bucket written by the previous
// release must be readable by this one. The migration fans the array out into
// sub-keys and purges the legacy key; both shapes are tolerated meanwhile.
func TestTrackingMigration_LegacyRecordFannedOut(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, _ := newIdentityLookupForTest(t)

	const memberID = "clusterA_node1"

	writeLegacyMemberRecord(t, il, memberID,
		"proj-MarketList/default-a",
		"proj-MarketList/default-b",
		"wk-Session/default-c",
	)

	require.True(t, il.migrateLegacyMemberRecords(ctx), "a complete fan-out reports done")

	assert.ElementsMatch(t, []string{
		memberID + ".proj-MarketList/default-a",
		memberID + ".proj-MarketList/default-b",
		memberID + ".wk-Session/default-c",
	}, listTrackingKeys(t, il, memberID))

	_, err := il.memberTracker.Get(ctx, memberID)
	assert.ErrorIs(t, err, jetstream.ErrKeyNotFound,
		"the legacy record must be purged once its keys are fanned out")
}

// TestTrackingMigration_IsIdempotent: the migration is driven from the janitor
// loop on whichever node holds leadership, so it must be safe to run
// repeatedly, and safe to run on a bucket that has nothing left to do.
func TestTrackingMigration_IsIdempotent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, _ := newIdentityLookupForTest(t)

	const memberID = "clusterA_node1"

	writeLegacyMemberRecord(t, il, memberID, "wk-Session/a", "wk-Session/b")

	want := []string{memberID + ".wk-Session/a", memberID + ".wk-Session/b"}

	for run := range 3 {
		require.True(t, il.migrateLegacyMemberRecords(ctx), "run %d must report done", run)
		assert.ElementsMatch(t, want, listTrackingKeys(t, il, memberID), "run %d", run)
	}

	assert.Zero(t, il.streakLenForTest(), "a repeat migration is not a write failure")
}

// TestTrackingMigration_ResumesAfterAnInterruptedRun is the crash-window
// contract from the other side: a run that stopped before its Purge left the
// legacy record in place, and the next run must finish the job without losing
// anything -- including keys the legacy record never had, because they were
// activated as sub-keys in between.
func TestTrackingMigration_ResumesAfterAnInterruptedRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, _ := newIdentityLookupForTest(t)

	const memberID = "clusterA_node1"

	writeLegacyMemberRecord(t, il, memberID, "wk-Session/a", "wk-Session/b")

	// What an interrupted run leaves behind: some sub-keys, the legacy record
	// still present, plus a grain activated after the crash.
	il.addKeyToMember(ctx, memberID, "wk-Session/a")
	il.addKeyToMember(ctx, memberID, "wk-Session/after-the-crash")

	require.True(t, il.migrateLegacyMemberRecords(ctx))

	assert.ElementsMatch(t, []string{
		memberID + ".wk-Session/a",
		memberID + ".wk-Session/b",
		memberID + ".wk-Session/after-the-crash",
	}, listTrackingKeys(t, il, memberID),
		"resuming must add what is missing and remove nothing")

	_, err := il.memberTracker.Get(ctx, memberID)
	assert.ErrorIs(t, err, jetstream.ErrKeyNotFound)
}

// TestTrackingMigration_RequestBudget is the N+1 guard on the migration itself.
// mrec.Keys is the 262-393 KB array this task removes -- tens of thousands of
// entries on the spoke -- and the fan-out runs on a leader node's maintenance
// loop. Unbounded, it would be one long write storm.
func TestTrackingMigration_RequestBudget(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, env := newIdentityLookupForTest(t)

	const memberID = "clusterA_node1"

	// 20k and not 50k because 50k CANNOT BE WRITTEN: a legacy record that size
	// is "nats: maximum payload exceeded", which is the cliff this change
	// removes and is pinned arithmetically by
	// TestTrackingSubKeys_NoSizeCliffAt50kGrains. 20k still fits under the
	// 1 MiB default and is ten times the per-pass write cap, which is what
	// this test is measuring.
	keys := make([]string, 20_000)
	for i := range keys {
		keys[i] = fmt.Sprintf("wk-Session/default-%d", i)
	}

	writeLegacyMemberRecord(t, il, memberID, keys...)

	writes := newSubjectCounter(t, env.conn, "$KV."+env.trackingBucket+".>")
	writes.reset(t)

	require.False(t, il.migrateLegacyMemberRecords(ctx),
		"a capped run must report that it is NOT done")

	if got := writes.count(t); got > maxMigrationPutsPerSetup {
		t.Fatalf("migration writes in one pass = %d, want <= %d", got, maxMigrationPutsPerSetup)
	}

	// The legacy record survives so the next pass resumes; nothing is lost.
	_, err := il.memberTracker.Get(ctx, memberID)
	require.NoError(t, err, "an incomplete fan-out must leave the legacy record for the next run")
}

// TestTrackingMigration_SkipsAlreadyMigratedKeys pins that the migration reads
// nothing per sub-key. Its enumeration sees every key in the bucket, and after
// the fan-out that is one key per grain -- probing each would be a fresh N+1 on
// every pass.
func TestTrackingMigration_SkipsAlreadyMigratedKeys(t *testing.T) {
	t.Parallel()

	const subKeys = 40

	ctx := context.Background()
	il, env := newIdentityLookupForTest(t)

	for i := range subKeys {
		il.addKeyToMember(ctx, "clusterA_node1", fmt.Sprintf("wk-Session/default-%d", i))
	}

	api := newJSAPICounter(t, env.conn)
	consumers := newConsumerCreateRecorder(t, env.conn)

	api.reset(t)
	consumers.reset(t)

	require.True(t, il.migrateLegacyMemberRecords(ctx))

	gets := api.countPrefix(t, "$JS.API.DIRECT.GET.") + api.countPrefix(t, "$JS.API.STREAM.MSG.GET.")
	assert.Zero(t, gets, "a sub-key needs no Get: its shape is visible in the key")

	assert.Len(t, consumers.created(t), 1, "one enumeration per pass")
}

// seedTrackedActivation writes an identity record for key owned by memberID and
// records the member's tracking sub-key, i.e. the state a completed activation
// leaves behind.
func seedTrackedActivation(t *testing.T, il *IdentityLookup, memberID, key string) {
	t.Helper()

	writeIdentityRecord(t, il, memberID, key)
	il.addKeyToMember(context.Background(), memberID, key)
}

// writeIdentityRecord writes a completed activation record (PidID set, so it is
// not a lock-only record) for key, owned by memberID.
func writeIdentityRecord(t *testing.T, il *IdentityLookup, memberID, key string) {
	t.Helper()

	data, err := json.Marshal(&activationRecord{
		PidID:      key,
		PidAddress: "127.0.0.1:8080",
		MemberID:   memberID,
	})
	require.NoError(t, err)

	_, err = il.identities.Put(context.Background(), key, data)
	require.NoError(t, err)
}

// writeLegacyMemberRecord writes the previous release's per-member JSON array,
// so the migration and the read path's legacy arm can be driven against the
// real encoding rather than a stand-in.
func writeLegacyMemberRecord(t *testing.T, il *IdentityLookup, memberID string, keys ...string) {
	t.Helper()

	data, err := json.Marshal(&memberRecord{Keys: keys})
	require.NoError(t, err)

	_, err = il.memberTracker.Put(context.Background(), memberID, data)
	require.NoError(t, err)
}

// identitiesOf projects the identity half out of a grain listing.
func identitiesOf(grains []*cluster.GrainInfo) []string {
	out := make([]string, 0, len(grains))
	for _, g := range grains {
		out = append(out, g.Identity)
	}

	return out
}
