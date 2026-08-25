package natskv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/remote"
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

// --- Fix wave 1 ---

// keyListerGoroutines reports how many nats.go key-lister goroutines exist and
// the stacks of those parked SENDING into their channel -- the exact shape an
// abandoned lister leaves behind. The state is matched rather than a goroutine
// count, which would be both noisy and unable to say what leaked.
func keyListerGoroutines() (live int, parked []string) {
	// Started large and grown until the dump is comfortably short of the
	// buffer end: runtime.Stack truncates at a whole goroutine, so a dump that
	// merely fits closely may already have dropped the newest goroutines --
	// which are exactly the ones a leak leaves behind.
	buf := make([]byte, 1<<20)

	for {
		n := runtime.Stack(buf, true)
		if n < len(buf)-(1<<16) {
			buf = buf[:n]

			break
		}

		buf = make([]byte, 2*len(buf))
	}

	// The frame name covers ListKeys and ListKeysFiltered, which share
	// keyLister; "chan send" is the goroutine header's wait reason.
	for _, g := range strings.Split(string(buf), "\n\n") {
		if !strings.Contains(g, "jetstream.(*kvs).ListKeys") {
			continue
		}

		live++

		if strings.Contains(g, "chan send") {
			parked = append(parked, g)
		}
	}

	return live, parked
}

// requireKeyListersDrained waits for the lister goroutines to reach a TERMINAL
// state and requires that it is the right one. A lister is asynchronous: once
// its consumer stops ranging it either finishes and exits, or fills its
// 256-entry buffer and parks on the send forever. Sampling "nothing is parked"
// immediately after the pass returns proves nothing, because the leak takes a
// few milliseconds to establish; this waits for one of the two outcomes.
func requireKeyListersDrained(t *testing.T, why string) {
	t.Helper()

	deadline := time.Now().Add(30 * time.Second)

	for {
		live, parked := keyListerGoroutines()
		if len(parked) > 0 {
			t.Fatalf("%d key-lister goroutine(s) parked on a channel send: %s\n%s",
				len(parked), why, strings.Join(parked, "\n"))
		}

		if live == 0 {
			return // every lister goroutine ran to completion and exited
		}

		if time.Now().After(deadline) {
			t.Fatalf("%d key-lister goroutine(s) never reached a terminal state", live)
		}

		time.Sleep(20 * time.Millisecond)
	}
}

// TestTrackingMigration_DrainsTheKeyListerWhenCapped is the goroutine-leak
// guard on the migration's own enumeration.
//
// nats.go's key lister runs a goroutine that forwards keys into a 256-entry
// buffered channel, and the send is OUTSIDE its select (v1.52.0
// jetstream/kv.go:1414-1423):
//
//	case entry := <-watcher.Updates():
//	        if entry == nil { return }
//	        kl.keys <- entry.Key()   // blocking, and not a select case
//	case <-ctx.Done():
//	        return
//
// So a caller that stops ranging before the keys are exhausted parks that
// goroutine forever as soon as the buffer fills: cancelling the context cannot
// release a goroutine that is not in the select, and the deferred
// watcher.Stop() inside it never runs, so the server-side consumer leaks with
// it. On this path that is one goroutine and one consumer per janitor tick,
// for as long as the migration keeps being capped -- i.e. the whole migration
// on any bucket big enough to need one. Every other lister site in this
// package drains to exhaustion; this one must too.
//
// The shape that triggers it: the first legacy record spends the per-pass
// write cap, the second legacy record meets the cap check, and more than the
// channel's 256 buffer is still unread behind it.
func TestTrackingMigration_DrainsTheKeyListerWhenCapped(t *testing.T) {
	ctx := context.Background()
	il, _ := newIdentityLookupForTest(t)

	const (
		spender  = "clusterA_node1" // its fan-out spends the write cap
		stopper  = "clusterA_node2" // the record the cap check is reached on
		trailing = 2000             // unread behind it, far past the lister's 256 buffer
	)

	keys := make([]string, maxMigrationPutsPerSetup+1)
	for i := range keys {
		keys[i] = fmt.Sprintf("wk-Session/capped-%d", i)
	}

	writeLegacyMemberRecord(t, il, spender, keys...)
	writeLegacyMemberRecord(t, il, stopper, "wk-Session/stopper-a")

	for i := range trailing {
		_, err := il.memberTracker.Put(ctx,
			trackingSubKey("clusterA_node3", fmt.Sprintf("wk-Session/tail-%d", i)), nil)
		require.NoError(t, err)
	}

	require.False(t, il.migrateLegacyMemberRecords(ctx), "a capped pass is not done")

	requireKeyListersDrained(t,
		"the capped migration pass abandoned its enumeration instead of draining it")

	// Draining is not doing the work: the cap still holds.
	_, err := il.memberTracker.Get(ctx, stopper)
	require.NoError(t, err, "a capped pass must leave the legacy record it did not reach")
}

// keyListerBuffer is the capacity nats.go gives a key lister's channel
// (v1.52.0 jetstream/kv.go:1411, make(chan string, 256)).
const keyListerBuffer = 256

// blockingKeyLister reproduces the ONE mechanic that makes an abandoned
// enumeration a leak, exactly as upstream implements it: a feeder goroutine
// pushing into a 256-entry buffered channel with the send OUTSIDE the select
// that watches the context (v1.52.0 jetstream/kv.go:1414-1423). Once the buffer
// is full the feeder is parked in a channel send, where a cancelled context
// cannot reach it and the deferred watcher.Stop() it carries never runs.
//
// It exists because the real lister leaks only when the timing cooperates --
// whether it has delivered more than the buffer holds by the moment its
// consumer walks away is a scheduling race. The contract ("drain it") is not a
// race, so it is pinned here deterministically, and
// TestTrackingMigration_DrainsTheKeyListerWhenCapped observes the real thing.
type blockingKeyLister struct {
	keys chan string
	done chan struct{}
}

func newBlockingKeyLister(keys []string) *blockingKeyLister {
	kl := &blockingKeyLister{
		keys: make(chan string, keyListerBuffer),
		done: make(chan struct{}),
	}

	go func() {
		defer close(kl.done)
		defer close(kl.keys)

		for _, k := range keys {
			// No context arm: upstream has one, and it is precisely what a
			// goroutine already parked in this send cannot reach. Modelling
			// the arm would make the fake's own send-versus-cancel race the
			// subject of the test instead of the contract being pinned.
			kl.keys <- k
		}
	}()

	return kl
}

func (k *blockingKeyLister) Keys() <-chan string { return k.keys }

func (k *blockingKeyLister) Stop() error { return nil }

// blockingListerKV serves that lister from a real bucket, so every other
// operation the migration performs -- Get, Put, Purge -- still runs against
// real JetStream.
type blockingListerKV struct {
	jetstream.KeyValue

	keys   []string
	lister *blockingKeyLister
}

func (k *blockingListerKV) ListKeys(_ context.Context, _ ...jetstream.WatchOpt) (jetstream.KeyLister, error) {
	k.lister = newBlockingKeyLister(k.keys)

	return k.lister, nil
}

// TestTrackingMigration_CappedPassStillConsumesEveryKey is the deterministic
// half of the drain contract: when the pass stops doing work it must keep
// READING, to exhaustion, because the lister it walks away from cannot be
// stopped from the outside.
func TestTrackingMigration_CappedPassStillConsumesEveryKey(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, _ := newIdentityLookupForTest(t)

	const (
		spender = "clusterA_node1" // its fan-out spends the per-pass write cap
		stopper = "clusterA_node2" // the record the cap check is reached on
	)

	keys := make([]string, maxMigrationPutsPerSetup+1)
	for i := range keys {
		keys[i] = fmt.Sprintf("wk-Session/capped-%d", i)
	}

	writeLegacyMemberRecord(t, il, spender, keys...)
	writeLegacyMemberRecord(t, il, stopper, "wk-Session/stopper-a")

	// Enumerated behind the two legacy records, and more numerous than the
	// lister's buffer: with an early break these are what the feeder is left
	// holding.
	enumerated := []string{spender, stopper}
	for i := range 2 * keyListerBuffer {
		enumerated = append(enumerated, trackingSubKey("clusterA_node3", fmt.Sprintf("wk-Session/tail-%d", i)))
	}

	tracker := &blockingListerKV{KeyValue: il.memberTracker, keys: enumerated}
	il.memberTracker = tracker

	require.False(t, il.migrateLegacyMemberRecords(ctx), "a capped pass is not done")

	select {
	case <-tracker.lister.done:
	case <-time.After(10 * time.Second):
		t.Fatal("the key lister's feeder goroutine is still parked on its channel send: " +
			"the capped pass abandoned the enumeration instead of draining it")
	}

	// Draining is not doing the work: the cap still holds.
	_, err := il.memberTracker.Get(ctx, stopper)
	require.NoError(t, err, "a capped pass must leave the legacy record it did not reach")
}

// setupClusterWithSystemID drives a real IdentityLookup.Setup against a cluster
// whose ActorSystem.ID carries systemID.
//
// That id is the half of the member id this package does not choose:
// production sets it from the node's own name (the superproject's actorsystem
// component calls actor.WithSystemID(hostname())), so a host whose name is an
// FQDN is exactly how a dotted member id arrives in a real deployment.
func setupClusterWithSystemID(t *testing.T, clusterName, systemID string) *IdentityLookup {
	t.Helper()

	srv := startEmbeddedNATS(t)
	nc, _ := connectNATS(t, srv)

	p, err := New(nc)
	require.NoError(t, err)

	system := actor.NewActorSystem(actor.WithSystemID(systemID))
	remoteConfig := remote.Configure("127.0.0.1", 0)
	kind := cluster.NewKind("TestKind", actor.PropsFromFunc(func(actor.Context) {}))

	c := cluster.NewCluster(system, cluster.Configure(clusterName, p, p.IdentityLookup(), remoteConfig,
		cluster.WithKinds(kind)))
	c.Remote = remote.NewRemote(system, remoteConfig)

	require.NoError(t, c.Remote.Start())
	c.InitKindsForTest(kind)

	il := p.IdentityLookup()
	il.Setup(c, []string{"TestKind"}, false)

	t.Cleanup(func() {
		il.Shutdown()
		c.Remote.Shutdown(true)
	})

	return il
}

// TestSetup_RejectsDottedMemberID is the enforcement TestMemberID_ContainsNoDot
// only pins the environment for. A dot in the member id is not cosmetic: '.'
// is the tracking sub-key separator, so trackingKeyMember splits at the FIRST
// dot and every one of that member's sub-keys is attributed to a prefix of its
// id and an identity no identities Get can resolve. Measured consequence:
// ByMember reports zero grains for the node -- in every process, on the
// collector's 30s loop -- and removeMemberID stops removing the identity
// records of a member that has left. Both are silent.
//
// A hostname that is an FQDN produces exactly that member id, so the
// precondition is enforced at startup rather than documented.
func TestSetup_RejectsDottedMemberID(t *testing.T) {
	t.Parallel()

	il := setupClusterWithSystemID(t, "testcluster", "node.host.example.com")

	require.Error(t, il.setupErr, "a dotted member id must fail Setup loudly")
	assert.Contains(t, il.setupErr.Error(), il.memberID,
		"the error must name the offending member id")
	assert.Contains(t, il.setupErr.Error(), "'.'",
		"and say what is wrong with it")

	// Not a silent failure: every read path surfaces it.
	_, err := il.ListGrains()
	require.ErrorIs(t, err, il.setupErr)

	assert.Nil(t, il.memberTracker,
		"the guard must run before the buckets are created")
}

// TestSetup_UndottedMemberIDIsUnaffected is the other half of the row: the
// guard rejects the dotted id and nothing else.
func TestSetup_UndottedMemberIDIsUnaffected(t *testing.T) {
	t.Parallel()

	il := setupClusterWithSystemID(t, "testcluster", "node-host-example-com")

	require.NoError(t, il.setupErr)
	require.NotEmpty(t, il.memberID)
	require.NotContains(t, il.memberID, ".")
	require.NotNil(t, il.memberTracker, "Setup must have completed")
}

// TestListGrains_DeduplicatesAnIdentityHeldInBothShapes is the dedupe row the
// cross-member union test cannot cover: during the migration one member holds
// the same identity as a legacy array entry AND as a sub-key, and an operator
// enumeration must show it once. Without the dedupe the same grain is reported
// twice, with two identities Gets to build it.
func TestListGrains_DeduplicatesAnIdentityHeldInBothShapes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, _ := newIdentityLookupForTest(t)

	const (
		memberID = "clusterA_node1"
		key      = "wk-Session/dup"
	)

	writeIdentityRecord(t, il, memberID, key)
	// The crash window this exists for: fanned out, not yet purged.
	il.addKeyToMember(ctx, memberID, key)
	writeLegacyMemberRecord(t, il, memberID, key)

	grains, err := il.ListGrains()
	require.NoError(t, err)

	require.Len(t, grains, 1, "one identity in both shapes is still one grain")
	assert.Equal(t, "dup", grains[0].Identity)
	assert.Equal(t, memberID, grains[0].MemberID)
}

// TestListGrains_EmptyBucketIsNotAnError pins the behaviour that the removed
// ErrNoKeysFound branch claimed to provide. jetstream's ListKeys never returns
// that sentinel -- only the legacy KeyValue.Keys() does -- so an empty bucket
// arrives as an enumeration that yields nothing.
func TestListGrains_EmptyBucketIsNotAnError(t *testing.T) {
	t.Parallel()

	il, _ := newIdentityLookupForTest(t)

	grains, err := il.ListGrains()

	require.NoError(t, err)
	assert.Empty(t, grains)
}

// failingKV wraps a real KeyValue so one operation on one key can be made to
// fail. The transitional legacy-record paths are error paths against a bucket
// another release is still writing, and this drives them against a real
// bucket rather than a stand-in for the whole interface.
type failingKV struct {
	jetstream.KeyValue

	failGetKey   string
	failPurgeKey string
	err          error
}

func (k failingKV) Get(ctx context.Context, key string) (jetstream.KeyValueEntry, error) {
	if k.failGetKey != "" && key == k.failGetKey {
		return nil, k.err
	}

	return k.KeyValue.Get(ctx, key)
}

func (k failingKV) Purge(ctx context.Context, key string, opts ...jetstream.KVDeleteOpt) error {
	if k.failPurgeKey != "" && key == k.failPurgeKey {
		return k.err
	}

	return k.KeyValue.Purge(ctx, key, opts...)
}

// TestTrackingMigration_PurgeFailureIsNotDone is the bookkeeping honesty row.
// The pass reports whether the bucket is free of legacy records, and its
// caller latches on that answer: a "done" returned while a legacy record is
// still sitting there stops the migration for the life of the janitor
// goroutine and logs "tracking migration complete" over a bucket that is not.
//
// A failed or CAS-lost Purge is exactly that case -- an old-release node
// rewrote the array between the read and the purge, or the write failed -- so
// the pass must report itself unfinished and come back next tick.
func TestTrackingMigration_PurgeFailureIsNotDone(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, _ := newIdentityLookupForTest(t)

	const memberID = "clusterA_node1"

	writeLegacyMemberRecord(t, il, memberID, "wk-Session/a", "wk-Session/b")

	il.memberTracker = failingKV{
		KeyValue:     il.memberTracker,
		failPurgeKey: memberID,
		err:          errors.New("purge rejected"),
	}

	require.False(t, il.migrateLegacyMemberRecords(ctx),
		"a legacy record still in the bucket is not a finished migration")

	// The fan-out is not rolled back: the read path unions both shapes, and the
	// next pass re-Puts (idempotent) and re-purges.
	assert.ElementsMatch(t,
		[]string{memberID + ".wk-Session/a", memberID + ".wk-Session/b"},
		listTrackingKeys(t, il, memberID))

	_, err := il.memberTracker.Get(ctx, memberID)
	require.NoError(t, err, "the legacy record survived, which is why the pass is not done")
}

// TestRemoveMemberID_ContinuesWhenTheLegacyGetFails pins that the two
// transitional legacy-record readers are best-effort in the SAME direction.
// ListGrains' expandLegacyMemberRecord already skips a legacy record it cannot
// read; memberTracking used to turn the same transient error into an aborted
// removeMemberID -- so a single failed Get of a bridge record left every
// identity record of a departed member behind, with the sub-key enumeration
// that answered the question sitting unused.
func TestRemoveMemberID_ContinuesWhenTheLegacyGetFails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, _ := newIdentityLookupForTest(t)

	const memberID = "clusterA_node1"

	seedTrackedActivation(t, il, memberID, "wk-Session/a")

	il.memberTracker = failingKV{
		KeyValue:   il.memberTracker,
		failGetKey: memberID,
		err:        errors.New("legacy get failed"),
	}

	il.removeMemberID(ctx, memberID)

	_, err := il.identities.Get(ctx, "wk-Session/a")
	assert.ErrorIs(t, err, jetstream.ErrKeyNotFound,
		"the sub-key enumeration answered; a failed legacy Get must not discard it")

	assert.Empty(t, listTrackingKeys(t, il, memberID),
		"and the member's own tracking state goes with it")
}
