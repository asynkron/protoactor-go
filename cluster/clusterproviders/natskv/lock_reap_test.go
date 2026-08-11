package natskv

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/cluster"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sanitizeClusterName replaces characters invalid in NATS bucket names with dashes.
func sanitizeClusterName(name string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, name)
}

// writeLockRecord writes a raw lock-only record (PidID=="") into the identities
// bucket and returns the KV entry so tests can examine Created() for age faking.
func writeLockRecord(t *testing.T, il *IdentityLookup, ci *cluster.ClusterIdentity, ownerMemberID string) jetstream.KeyValueEntry {
	t.Helper()
	ctx := context.Background()
	rec := activationRecord{
		LockID:   "test-lock-id",
		MemberID: ownerMemberID,
	}
	data, err := json.Marshal(&rec)
	require.NoError(t, err)
	_, err = il.identities.Create(ctx, kvKey(ci), data)
	require.NoError(t, err)
	entry, err := il.identities.Get(ctx, kvKey(ci))
	require.NoError(t, err)
	return entry
}

// TestLockReapOnGet exercises all six reap-condition branches of maybeReapLock.
// Age is faked via il.now. Owner presence is controlled via topology on a real
// cluster for the "ownerInList" cases, and ignored for bare-struct tests (the
// hard branch fires before the owner-absence check).
func TestLockReapOnGet(t *testing.T) {
	cases := []struct {
		name        string
		ownerInList bool
		age         time.Duration
		wantReaped  bool
	}{
		{"dead owner, aged past grace", false, 35 * time.Second, true},
		{"dead owner, young", false, 5 * time.Second, false},   // convergence guard
		{"live owner, young", true, 5 * time.Second, false},
		{"live owner, past hard threshold", true, 65 * time.Second, true}, // slow-live-aged
		{"legacy no-owner record, young", false, 20 * time.Second, false}, // hard branch only
		{"legacy no-owner record, past hard", false, 65 * time.Second, true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Use the full cluster harness so ValidateActivationMember works.
			clusterName := sanitizeClusterName("test-lock-reap-" + tc.name)
			_, c, il := setupPlacementTestCluster(t, clusterName)
			ctx := context.Background()

			// ownerMemberID: for "legacy no-owner" cases, use empty string.
			ownerMemberID := "owner-member-reap-" + tc.name
			isLegacy := tc.name == "legacy no-owner record, young" || tc.name == "legacy no-owner record, past hard"
			if isLegacy {
				ownerMemberID = ""
			}

			// Publish topology: include or exclude ownerMemberID.
			host, port, err := c.ActorSystem.GetHostPort()
			require.NoError(t, err)
			selfMember := &cluster.Member{
				Host:  host,
				Port:  int32(port),
				Id:    il.memberID,
				Kinds: []string{"TestKind"},
			}
			members := cluster.Members{selfMember}
			if tc.ownerInList && !isLegacy {
				members = append(members, &cluster.Member{
					Host:  "owner-host",
					Port:  9999,
					Id:    ownerMemberID,
					Kinds: []string{"TestKind"},
				})
			}
			c.MemberList.UpdateClusterTopology(members)

			ci := testCI("TestKind", "reap-test-"+sanitizeClusterName(tc.name))
			entry := writeLockRecord(t, il, ci, ownerMemberID)

			// Fake time: make entry appear older by the test-case age.
			entryCreated := entry.Created()
			il.now = func() time.Time { return entryCreated.Add(tc.age) }

			reaped := il.maybeReapLock(ctx, ci)
			assert.Equal(t, tc.wantReaped, reaped,
				"case %q: maybeReapLock mismatch (ownerInList=%v, age=%v)", tc.name, tc.ownerInList, tc.age)

			// After a successful reap, the key should be absent.
			if tc.wantReaped {
				_, err := il.identities.Get(ctx, kvKey(ci))
				assert.ErrorIs(t, err, jetstream.ErrKeyNotFound,
					"case %q: key should be deleted after reap", tc.name)
			}
		})
	}
}

// TestMaybeReapLockIgnoresFullActivations verifies that maybeReapLock is a no-op
// when the record has a PID (i.e., it is a completed activation, not a lock).
func TestMaybeReapLockIgnoresFullActivations(t *testing.T) {
	il := buildBareIdentityLookup(t)
	ctx := context.Background()
	ci := testCI("TestKind", "full-activation")

	// Write a completed activation record.
	rec := activationRecord{
		PidID:      "TestKind/full-activation",
		PidAddress: "host:8080",
		MemberID:   "some-member",
	}
	data, err := json.Marshal(&rec)
	require.NoError(t, err)
	_, err = il.identities.Put(ctx, kvKey(ci), data)
	require.NoError(t, err)

	// Fake old age so we would reap if this were a lock.
	il.now = func() time.Time { return time.Now().Add(70 * time.Second) }

	reaped := il.maybeReapLock(ctx, ci)
	assert.False(t, reaped, "maybeReapLock must not reap completed activations")

	// Record must still be present.
	after, err := il.identities.Get(ctx, kvKey(ci))
	require.NoError(t, err, "activation must not be deleted")
	var afterRec activationRecord
	require.NoError(t, json.Unmarshal(after.Value(), &afterRec))
	assert.Equal(t, "TestKind/full-activation", afterRec.PidID)
}

// TestMaybeReapLockAbsentKey verifies that maybeReapLock returns false (not
// reaped) when the key does not exist.
func TestMaybeReapLockAbsentKey(t *testing.T) {
	il := buildBareIdentityLookup(t)
	ctx := context.Background()
	ci := testCI("TestKind", "absent-key")

	reaped := il.maybeReapLock(ctx, ci)
	assert.False(t, reaped, "maybeReapLock on absent key should return false")
}

// TestPostReapRetryReentersExistingActivationCheck verifies the bounded retry
// loop in resolveIdentity: after an orphan lock is reaped, if a foreign
// activation is written before the retry, Get must return the foreign PID
// instead of attempting to re-lock.
func TestPostReapRetryReentersExistingActivationCheck(t *testing.T) {
	_, c, il := setupPlacementTestCluster(t, "test-post-reap-retry")
	ctx := context.Background()

	ci := testCI("TestKind", "post-reap-grain")
	key := kvKey(ci)

	// Publish topology with self only (no foreign member in list).
	host, port, err := c.ActorSystem.GetHostPort()
	require.NoError(t, err)
	selfMember := &cluster.Member{
		Host:  host,
		Port:  int32(port),
		Id:    il.memberID,
		Kinds: []string{"TestKind"},
	}
	c.MemberList.UpdateClusterTopology(cluster.Members{selfMember})

	// Write an orphan lock (absent owner, aged past grace).
	orphanOwner := "absent-member-orphan"
	rec := activationRecord{
		LockID:   "orphan-lock",
		MemberID: orphanOwner,
	}
	data, err := json.Marshal(&rec)
	require.NoError(t, err)
	_, err = il.identities.Create(ctx, key, data)
	require.NoError(t, err)

	entry, err := il.identities.Get(ctx, key)
	require.NoError(t, err)
	entryCreated := entry.Created()
	// Fake age past grace period.
	il.now = func() time.Time { return entryCreated.Add(40 * time.Second) }

	// Reap the lock directly to confirm it works.
	reaped := il.maybeReapLock(ctx, ci)
	require.True(t, reaped, "lock must be reaped before injecting foreign activation")

	// Now inject a "foreign" activation as if another node activated between
	// the reap and the retry.
	foreignRec := activationRecord{
		PidID:      "TestKind/post-reap-grain",
		PidAddress: "foreign-host:9000",
		MemberID:   il.memberID, // use self so ValidateActivationMember passes
	}
	foreignData, err := json.Marshal(&foreignRec)
	require.NoError(t, err)
	_, err = il.identities.Put(ctx, key, foreignData)
	require.NoError(t, err)

	// Reset clock so normal time is used.
	il.now = time.Now

	// Get should return the foreign activation (existing-activation check on retry).
	pid := il.Get(ci)
	require.NotNil(t, pid, "Get must return the foreign PID after reap + retry")
	assert.Equal(t, "foreign-host:9000", pid.Address)
}

// TestCreatedResetsAfterReapAndRelock verifies that after reaping an aged lock
// and a successor re-creating it, entryAge of the new entry is near zero (not
// the original entry's age).
func TestCreatedResetsAfterReapAndRelock(t *testing.T) {
	il := buildBareIdentityLookup(t)
	ctx := context.Background()
	ci := testCI("TestKind", "reap-relock")

	// Write an old lock: owner absent (empty MemberID, legacy path).
	rec := activationRecord{LockID: "old-lock"}
	data, err := json.Marshal(&rec)
	require.NoError(t, err)
	_, err = il.identities.Create(ctx, kvKey(ci), data)
	require.NoError(t, err)

	entry, err := il.identities.Get(ctx, kvKey(ci))
	require.NoError(t, err)
	oldCreated := entry.Created()

	// Fake time: 65s past creation -> hard reap fires.
	il.now = func() time.Time { return oldCreated.Add(65 * time.Second) }

	reaped := il.maybeReapLock(ctx, ci)
	require.True(t, reaped, "old lock should be reaped")

	// Successor acquires lock (simulate re-lock).
	il.now = time.Now
	_, _, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok, "should be able to re-acquire lock after reap")

	// entryAge of new entry should be near zero.
	newEntry, err := il.identities.Get(ctx, kvKey(ci))
	require.NoError(t, err)
	age := entryAge(newEntry, il.now())
	assert.Less(t, age, 2*time.Second,
		"new entry age should be near zero after reap + relock, got %v", age)
}

// TestWaiterWindowIsSeparateFromLockTTL verifies that waitForActivation's
// timeout uses WaiterWindow (not LockTTL): a waiter on a never-completing lock
// returns after ~WaiterWindow, not ~LockTTL.
func TestWaiterWindowIsSeparateFromLockTTL(t *testing.T) {
	il := buildBareIdentityLookup(t)
	ctx := context.Background()

	// Set a short WaiterWindow and a long LockTTL.
	il.config.WaiterWindow = 200 * time.Millisecond
	il.config.LockTTL = 5 * time.Second

	ci := testCI("TestKind", "waiter-window")

	// Write a lock record that never becomes a full activation.
	rec := activationRecord{LockID: "stuck-lock", MemberID: "some-member"}
	data, err := json.Marshal(&rec)
	require.NoError(t, err)
	_, err = il.identities.Create(ctx, kvKey(ci), data)
	require.NoError(t, err)

	start := time.Now()
	result := il.waitForActivation(ctx, ci)
	elapsed := time.Since(start)

	assert.Nil(t, result, "waitForActivation on a never-completing lock must return nil")
	// Should return after ~WaiterWindow, not ~LockTTL.
	assert.Less(t, elapsed, 2*time.Second,
		"waitForActivation should return after WaiterWindow (~200ms), not LockTTL (5s); elapsed=%v", elapsed)
	assert.GreaterOrEqual(t, elapsed, 100*time.Millisecond,
		"waitForActivation should wait at least most of WaiterWindow; elapsed=%v", elapsed)
}

// TestResolutionRetriesOnLockDeletion verifies that when a waiter observes a
// lock's KeyValueDelete, the bounded loop re-enters (retries resolution) instead
// of returning nil immediately. We confirm this by deleting the lock during the
// watch, which should cause the loop to retry and eventually acquire the lock.
func TestResolutionRetriesOnLockDeletion(t *testing.T) {
	_, c, il := setupPlacementTestCluster(t, "test-retry-on-delete")
	ctx := context.Background()

	ci := testCI("TestKind", "retry-on-delete")
	key := kvKey(ci)

	// Publish topology so the member is valid.
	host, port, err := c.ActorSystem.GetHostPort()
	require.NoError(t, err)
	selfMember := &cluster.Member{
		Host:  host,
		Port:  int32(port),
		Id:    il.memberID,
		Kinds: []string{"TestKind"},
	}
	c.MemberList.UpdateClusterTopology(cluster.Members{selfMember})

	// Write an initial lock for a "foreign" holder.
	foreignRec := activationRecord{LockID: "foreign-lock", MemberID: "foreign-holder"}
	foreignData, err := json.Marshal(&foreignRec)
	require.NoError(t, err)
	_, err = il.identities.Create(ctx, key, foreignData)
	require.NoError(t, err)

	// Schedule: delete the lock after 150ms so the waiter sees KeyValueDelete
	// and resolveIdentity retries.
	il.config.WaiterWindow = 2 * time.Second
	go func() {
		time.Sleep(150 * time.Millisecond)
		_ = il.identities.Delete(context.Background(), key)
	}()

	// Get must succeed: after observing the delete, the retry loop re-acquires
	// the lock and activates locally.
	pid := il.Get(ci)
	require.NotNil(t, pid, "Get must succeed after lock deletion triggers retry")
	assert.Equal(t, c.ActorSystem.Address(), pid.Address,
		"activation should be on the live node after retry")
}
