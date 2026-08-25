package natskv

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedSweepFixture writes n completed activation records owned by memberID and
// returns their identity keys. The records carry a PID, so the sweep takes the
// membership branch rather than the lock-reap branch.
func seedSweepFixture(t *testing.T, il *IdentityLookup, memberID string, n int) []string {
	t.Helper()

	keys := make([]string, 0, n)

	for i := range n {
		key := fmt.Sprintf("wk-Session/%s-%d", memberID, i)
		writeIdentityRecord(t, il, memberID, key)
		keys = append(keys, key)
	}

	return keys
}

// putMemberKey writes memberID's key into the members bucket, the way the
// provider's registerSelf does.
func putMemberKey(t *testing.T, env *natskvTestEnv, memberID string) {
	t.Helper()

	_, err := env.memberBucket.Put(context.Background(),
		env.provider.memberKey(memberID), []byte(`{}`))
	require.NoError(t, err)
}

// TestJanitorSweep_OneMemberListPerSweep is the sweep's second-leg guard: the
// sweep used to probe the members bucket once per activation record
// (MemberKeyExists -> memberBucket.Get), making it 1+2N. One ListKeys answers
// every probe, taking it to 2+N.
func TestJanitorSweep_OneMemberListPerSweep(t *testing.T) {
	t.Parallel()

	const (
		records  = 50
		memberID = "clusterA_node1"
	)

	ctx := context.Background()
	il, env := newIdentityLookupForTest(t, withMemberBucket())

	seedSweepFixture(t, il, memberID, records)
	putMemberKey(t, env, memberID)

	api := newJSAPICounter(t, env.conn)
	consumers := newConsumerCreateRecorder(t, env.conn)

	api.reset(t)
	consumers.reset(t)

	il.janitorSweep(ctx, &janitorAbsenceClock{})

	// One identities Get per key, and NO per-record member probe. Exact, not an
	// upper bound: an inequality sitting on the expected value reports drift in
	// one direction only.
	identityGets := api.countPrefix(t, "$JS.API.DIRECT.GET.KV_"+env.identityBucket+".") +
		api.countPrefix(t, "$JS.API.STREAM.MSG.GET.KV_"+env.identityBucket)
	assert.Equal(t, records, identityGets, "one identities Get per key")

	memberGets := api.countPrefix(t, "$JS.API.DIRECT.GET.KV_"+env.memberBucketName+".") +
		api.countPrefix(t, "$JS.API.STREAM.MSG.GET.KV_"+env.memberBucketName)
	assert.Zero(t, memberGets,
		"the membership question is answered once by the enumeration, not once per record")

	// Two enumerations per sweep: identities, then members. Never more.
	assert.Len(t, consumers.created(t), 2,
		"one ListKeys on identities and one on members, per sweep")
}

// TestJanitorSweep_AbsentMemberStillReaped keeps the behaviour the hoist must
// not change: an activation whose member is gone is still reaped, on the same
// two-observation + grace rule.
func TestJanitorSweep_AbsentMemberStillReaped(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, env := newIdentityLookupForTest(t, withMemberBucket())

	const (
		gone  = "clusterA_gone"
		alive = "clusterA_alive"
	)

	// A live member so the snapshot is non-empty: an empty snapshot is "no
	// information", not "every member is gone".
	putMemberKey(t, env, alive)
	writeIdentityRecord(t, il, alive, "wk-Session/alive-1")

	key := "wk-Session/gone-1"
	writeIdentityRecord(t, il, gone, key)

	t0 := time.Now()
	il.config.ActivationAbsentGrace = 60 * time.Second

	ac := &janitorAbsenceClock{}

	// Sweep 1: first observation, kept regardless.
	il.now = func() time.Time { return t0 }
	il.janitorSweep(ctx, ac)

	_, err := il.identities.Get(ctx, key)
	require.NoError(t, err, "sweep 1 (count=1): the record must be kept")

	// Sweep 2: second observation past the grace: reaped.
	il.now = func() time.Time { return t0.Add(61 * time.Second) }
	il.janitorSweep(ctx, ac)

	_, err = il.identities.Get(ctx, key)
	assert.ErrorIs(t, err, jetstream.ErrKeyNotFound,
		"sweep 2 (count>=2, age>=grace): an activation whose member is gone must be reaped")

	_, err = il.identities.Get(ctx, "wk-Session/alive-1")
	assert.NoError(t, err, "the live member's activation must survive")
}

// TestJanitorSweep_ReapConfirmsMembershipWithOnePointGet pins where the
// remaining membership Get lives: on the REAP EDGE, once, and nowhere else.
//
// The snapshot is a watcher-backed enumeration, and a truncated one surfaces no
// error -- the provider's own reconcileMembers guards against exactly that by
// confirming each prune candidate with an authoritative point Get before it
// removes anything. The sweep deletes live activation records, so it does the
// same: the absence CLOCK advances from the cheap snapshot, but the
// irreversible delete is confirmed. That is O(reaps), which is zero in steady
// state, not O(records).
func TestJanitorSweep_ReapConfirmsMembershipWithOnePointGet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, env := newIdentityLookupForTest(t, withMemberBucket())

	const (
		gone  = "clusterA_gone"
		alive = "clusterA_alive"
	)

	putMemberKey(t, env, alive)
	writeIdentityRecord(t, il, gone, "wk-Session/gone-1")

	t0 := time.Now()
	il.config.ActivationAbsentGrace = 60 * time.Second

	ac := &janitorAbsenceClock{}

	api := newJSAPICounter(t, env.conn)

	// Sweep 1 reaps nothing, so it must ask the members bucket nothing.
	il.now = func() time.Time { return t0 }
	api.reset(t)
	il.janitorSweep(ctx, ac)

	assert.Zero(t, memberBucketGets(t, api, env),
		"a sweep that reaps nothing makes no point Get on the members bucket")

	// Sweep 2 reaps one record, so it confirms exactly once.
	il.now = func() time.Time { return t0.Add(61 * time.Second) }
	api.reset(t)
	il.janitorSweep(ctx, ac)

	assert.Equal(t, 1, memberBucketGets(t, api, env),
		"exactly one confirmation per reap")
}

// TestJanitorSweep_MemberSnapshotUnavailableReapsNothing is the fail-closed
// rule. A membership answer the sweep could not obtain makes every activation
// look absent, which would reap the whole bucket; so no answer means no
// activation cleanup at all, and the absence clock does not advance.
func TestJanitorSweep_MemberSnapshotUnavailableReapsNothing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, env := newIdentityLookupForTest(t, withMemberBucket())

	const gone = "clusterA_gone"

	key := "wk-Session/gone-1"
	writeIdentityRecord(t, il, gone, key)

	// No members bucket at all: MemberKeysSnapshot has nothing to say.
	env.provider.memberBucket = nil

	t0 := time.Now()
	il.config.ActivationAbsentGrace = 0

	ac := &janitorAbsenceClock{}

	for i := range 5 {
		il.now = func() time.Time { return t0.Add(time.Duration(i) * time.Minute) }
		il.janitorSweep(ctx, ac)
	}

	_, err := il.identities.Get(ctx, key)
	assert.NoError(t, err,
		"five sweeps with no membership answer must reap nothing, however long the grace has elapsed")
}

// TestJanitorSweep_EmptyMemberBucketReapsNothing is the same rule for the
// snapshot that succeeded and came back empty. The sweep runs only on the
// leader, and a leader is itself a member, so an empty members bucket is a
// broken observation, not a cluster with no members.
func TestJanitorSweep_EmptyMemberBucketReapsNothing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, _ := newIdentityLookupForTest(t, withMemberBucket())

	const gone = "clusterA_gone"

	key := "wk-Session/gone-1"
	writeIdentityRecord(t, il, gone, key)

	t0 := time.Now()
	il.config.ActivationAbsentGrace = 0

	ac := &janitorAbsenceClock{}

	for i := range 5 {
		il.now = func() time.Time { return t0.Add(time.Duration(i) * time.Minute) }
		il.janitorSweep(ctx, ac)
	}

	_, err := il.identities.Get(ctx, key)
	assert.NoError(t, err, "an empty members snapshot must not be read as 'every member is gone'")
}

// TestJanitorSweep_LockReapsRunWithoutMembership pins that the membership half
// failing does not disable the other half: lock reaping never consults
// membership, so it must still run.
func TestJanitorSweep_LockReapsRunWithoutMembership(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	il, env := newIdentityLookupForTest(t, withMemberBucket())

	env.provider.memberBucket = nil

	key := "wk-Session/aged-lock"
	writeLockOnlyRecord(t, il, "clusterA_gone", key)

	il.config.HardReapAge = 60 * time.Second
	il.now = func() time.Time { return time.Now().Add(5 * time.Minute) }

	il.janitorSweep(ctx, &janitorAbsenceClock{})

	_, err := il.identities.Get(ctx, key)
	assert.ErrorIs(t, err, jetstream.ErrKeyNotFound,
		"an aged lock is reaped on age alone; membership is not consulted")
}

// TestMemberKeysSnapshot_InvertsTheMemberKey pins the one assumption the
// snapshot rests on: memberKey is KeyPrefix + ".members." + memberID, so the
// mapping is invertible. A key without that prefix is skipped rather than
// turned into a phantom member.
func TestMemberKeysSnapshot_InvertsTheMemberKey(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	_, env := newIdentityLookupForTest(t, withMemberBucket())

	putMemberKey(t, env, "clusterA_node1")
	putMemberKey(t, env, "clusterA_node2")

	_, err := env.memberBucket.Put(ctx, "some.foreign.key", []byte(`{}`))
	require.NoError(t, err)

	members, err := env.provider.MemberKeysSnapshot(ctx)
	require.NoError(t, err)

	assert.Equal(t, map[string]struct{}{
		"clusterA_node1": {},
		"clusterA_node2": {},
	}, members, "a key that is not a member key must not become a phantom member")
}

// TestMemberKeysSnapshot_NoBucketIsNoAnswer pins the nil the fail-closed arm
// keys on: a provider with no members bucket returns no map and no error, which
// the sweep reads as "no information", not "no members".
func TestMemberKeysSnapshot_NoBucketIsNoAnswer(t *testing.T) {
	t.Parallel()

	_, env := newIdentityLookupForTest(t, withMemberBucket())

	env.provider.memberBucket = nil

	members, err := env.provider.MemberKeysSnapshot(context.Background())

	require.NoError(t, err)
	assert.Nil(t, members)
}

// memberBucketGets counts point reads of the members bucket, on either the
// direct-get or the classic stream-msg-get subject.
func memberBucketGets(t *testing.T, api *jsAPICounter, env *natskvTestEnv) int {
	t.Helper()

	return api.countPrefix(t, "$JS.API.DIRECT.GET.KV_"+env.memberBucketName+".") +
		api.countPrefix(t, "$JS.API.STREAM.MSG.GET.KV_"+env.memberBucketName)
}

// writeLockOnlyRecord writes a lock-only record (no PidID) for key.
func writeLockOnlyRecord(t *testing.T, il *IdentityLookup, memberID, key string) {
	t.Helper()

	data, err := json.Marshal(&activationRecord{LockID: "held-lock", MemberID: memberID})
	require.NoError(t, err)

	_, err = il.identities.Put(context.Background(), key, data)
	require.NoError(t, err)
}

// TestTrackingMigration_RunsFromTheJanitorLoop pins WHERE the legacy fan-out is
// driven from, because the obvious alternative does not work.
//
// Setup cannot drive it: Cluster.StartMember calls IdentityLookup.Setup before
// ClusterProvider.StartMember, and leader election runs inside the latter, so a
// leadership check made at Setup time reads false on every node -- the first
// assertion below is that fact, observed rather than assumed. A one-shot
// leader-gated goroutine started from Setup would therefore migrate nothing,
// anywhere, ever. The janitor loop is the first leader-gated thing that exists,
// so the migration rides it.
func TestTrackingMigration_RunsFromTheJanitorLoop(t *testing.T) {
	p, _, il := setupPlacementTestCluster(t, "test-tracking-migration-janitor",
		WithJanitorInterval(10*time.Millisecond),
	)

	ctx := context.Background()

	require.False(t, p.IsLeader(),
		"Setup runs before the provider starts, so no node is the leader yet: "+
			"a migration gated on leadership AT Setup would never run")

	const memberID = "clusterA_node1"

	writeLegacyMemberRecord(t, il, memberID, "wk-Session/a", "wk-Session/b")

	// Non-leader: the janitor is ticking and must migrate nothing.
	time.Sleep(150 * time.Millisecond)
	assert.Empty(t, listTrackingKeys(t, il, memberID),
		"a non-leader must not fan out the legacy record")

	p.isLeader.Store(true)

	require.Eventually(t, func() bool {
		return len(listTrackingKeys(t, il, memberID)) == 2
	}, 5*time.Second, 20*time.Millisecond,
		"the janitor loop must fan the legacy record out once this node leads")

	_, err := il.memberTracker.Get(ctx, memberID)
	assert.ErrorIs(t, err, jetstream.ErrKeyNotFound, "and purge it when it is done")
}
