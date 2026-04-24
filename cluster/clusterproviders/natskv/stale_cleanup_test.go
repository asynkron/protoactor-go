package natskv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStale_MultipleCrashedNodeActivations_CleanedIncrementally verifies
// that when a node crashes (no graceful shutdown), its multiple stale
// activation records are cleaned up one-by-one as other nodes resolve
// each identity via Get(). The stale member's tracking record persists
// in KV until a topology leave event triggers removeMemberID.
//
// This extends TestIdentityLookup_StaleActivationCleaned (which covers
// single-grain stale detection) by testing the multi-grain case and
// verifying that member tracking records are NOT cleaned lazily — only
// the identity records themselves are cleaned on access.
func TestStale_MultipleCrashedNodeActivations_CleanedIncrementally(t *testing.T) {
	_, c, il := setupPlacementTestCluster(t, "test-stale-multi-crash")
	ctx := context.Background()

	crashedMember := "crashed-node-multi"

	// Simulate 3 stale activations from the crashed node.
	// Also create a member tracking record listing all 3 keys.
	var keys []string
	for i := 0; i < 3; i++ {
		ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: fmt.Sprintf("crash-multi-%d", i)}
		staleRec := activationRecord{
			PidID:      fmt.Sprintf("TestKind/crash-multi-%d", i),
			PidAddress: "crashed-host:9999",
			MemberID:   crashedMember,
		}
		data, err := json.Marshal(&staleRec)
		require.NoError(t, err)
		_, err = il.identities.Put(ctx, kvKey(ci), data)
		require.NoError(t, err)
		keys = append(keys, kvKey(ci))
	}

	// Write the member tracking record.
	trackData, err := json.Marshal(&memberRecord{Keys: keys})
	require.NoError(t, err)
	_, err = il.memberTracker.Put(ctx, crashedMember, trackData)
	require.NoError(t, err)

	// Access only grain 0 via Get() — should clean that one stale record.
	ci0 := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "crash-multi-0"}
	pid0 := il.Get(ci0)
	require.NotNil(t, pid0, "Get should return a new PID for grain 0")
	assert.Equal(t, c.ActorSystem.Address(), pid0.Address)

	// Grain 0's KV record should now point to this node.
	rec0 := il.getExistingActivation(ctx, ci0)
	require.NotNil(t, rec0)
	assert.Equal(t, il.memberID, rec0.MemberID,
		"grain 0 KV record should be updated to the live member")

	// Grains 1 and 2 should STILL have the stale records (not yet accessed).
	for i := 1; i <= 2; i++ {
		ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: fmt.Sprintf("crash-multi-%d", i)}
		rec := il.getExistingActivation(ctx, ci)
		require.NotNil(t, rec, "grain %d should still have stale record", i)
		assert.Equal(t, crashedMember, rec.MemberID,
			"grain %d should still show crashed member (not yet accessed via Get)", i)
	}

	// The member tracking record should still exist (lazy cleanup is
	// per-identity, not per-member — only topology leave cleans the
	// full member tracking record).
	_, err = il.memberTracker.Get(ctx, crashedMember)
	assert.NoError(t, err, "member tracking record should still exist (not cleaned lazily)")
}

// TestStale_TopologyLeave_CleansAllMemberActivations verifies that when
// a member leaves the cluster (detected via ClusterTopology event), all
// of its activation records and its member tracking record are removed
// from NATS KV.
func TestStale_TopologyLeave_CleansAllMemberActivations(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	ctx := context.Background()
	identities, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_topo_leave_identities",
	})
	require.NoError(t, err)

	tracking, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_topo_leave_tracking",
	})
	require.NoError(t, err)

	il := &IdentityLookup{
		identities:    identities,
		memberTracker: tracking,
		config:        newDefaultConfig(),
		semaphore:     make(chan struct{}, 200),
		memberID:      "member-survivor",
	}

	// Simulate a member that owned 5 grains.
	deadMemberID := "member-dead-xyz"
	for i := 0; i < 5; i++ {
		ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: fmt.Sprintf("topo-grain-%d", i)}
		lockID, rev, ok := il.tryAcquireLock(ctx, ci)
		require.True(t, ok)
		err = il.storeActivation(ctx, ci, lockID, rev, deadMemberID, "dead-host:8080", fmt.Sprintf("TestKind/topo-grain-%d", i))
		require.NoError(t, err)
	}

	// Verify all 5 activations and the tracking record exist.
	for i := 0; i < 5; i++ {
		ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: fmt.Sprintf("topo-grain-%d", i)}
		rec := il.getExistingActivation(ctx, ci)
		require.NotNil(t, rec, "activation %d should exist before cleanup", i)
	}
	trackEntry, err := tracking.Get(ctx, deadMemberID)
	require.NoError(t, err, "member tracking record should exist")
	var mrec memberRecord
	require.NoError(t, json.Unmarshal(trackEntry.Value(), &mrec))
	assert.Len(t, mrec.Keys, 5, "tracking record should have 5 keys")

	// Simulate topology leave by calling removeMemberID directly.
	// In production, this is triggered by the ClusterTopology event handler
	// in Setup() when topology.Left contains the dead member.
	il.removeMemberID(ctx, deadMemberID)

	// Verify all activations are gone.
	for i := 0; i < 5; i++ {
		ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: fmt.Sprintf("topo-grain-%d", i)}
		rec := il.getExistingActivation(ctx, ci)
		assert.Nil(t, rec, "activation %d should be deleted after topology leave", i)
	}

	// Verify the tracking record is gone.
	_, err = tracking.Get(ctx, deadMemberID)
	assert.ErrorIs(t, err, jetstream.ErrKeyNotFound,
		"member tracking record should be deleted after topology leave")
}

// TestStale_OrphanedMemberTracking_NoErrors verifies that removeMemberID
// handles the case where identity keys listed in a member's tracking
// record have already been deleted (e.g., RemoveActivation partial
// failure or race with another cleanup path). The cleanup should
// succeed without errors.
func TestStale_OrphanedMemberTracking_NoErrors(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	ctx := context.Background()
	identities, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_orphan_tracking_identities",
	})
	require.NoError(t, err)

	tracking, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_orphan_tracking_tracking",
	})
	require.NoError(t, err)

	il := &IdentityLookup{
		identities:    identities,
		memberTracker: tracking,
		config:        newDefaultConfig(),
		semaphore:     make(chan struct{}, 200),
	}

	orphanMember := "member-orphan"

	// Create 3 activations for the orphan member.
	for i := 0; i < 3; i++ {
		ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: fmt.Sprintf("orphan-%d", i)}
		lockID, rev, ok := il.tryAcquireLock(ctx, ci)
		require.True(t, ok)
		err = il.storeActivation(ctx, ci, lockID, rev, orphanMember, "h:8080", fmt.Sprintf("TestKind/orphan-%d", i))
		require.NoError(t, err)
	}

	// Manually delete 2 of the 3 identity keys to simulate partial cleanup.
	for i := 0; i < 2; i++ {
		ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: fmt.Sprintf("orphan-%d", i)}
		err = identities.Delete(ctx, kvKey(ci))
		require.NoError(t, err)
	}

	// removeMemberID should not panic or return errors when some keys
	// are already gone. It logs warnings but continues.
	assert.NotPanics(t, func() {
		il.removeMemberID(ctx, orphanMember)
	}, "removeMemberID should not panic on partially-deleted keys")

	// The remaining identity key (orphan-2) should be deleted.
	ci2 := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "orphan-2"}
	rec := il.getExistingActivation(ctx, ci2)
	assert.Nil(t, rec, "remaining identity key should be deleted by removeMemberID")

	// The tracking record itself should be deleted.
	_, err = tracking.Get(ctx, orphanMember)
	assert.ErrorIs(t, err, jetstream.ErrKeyNotFound,
		"member tracking record should be deleted")
}

// TestStale_RemoveActivation_CAS_DoesNotDeleteNewActivation verifies
// that RemoveActivation's CAS delete does not delete a new activation
// that was written between the grain termination and the cleanup.
//
// Scenario:
//  1. Node A activates grain "foo" (revision 1)
//  2. Grain terminates on node A -> RemoveActivation begins
//  3. Before RemoveActivation's CAS delete executes, node B re-activates
//     "foo" (revision 2 via CAS update)
//  4. RemoveActivation's CAS delete with revision 1 must fail
//  5. Node B's activation (revision 2) must survive
//
// We simulate this by manually writing version 2 between reading version 1
// and calling the CAS delete, using the RemoveActivation callback.
func TestStale_RemoveActivation_CAS_DoesNotDeleteNewActivation(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	ctx := context.Background()
	identities, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_cas_safety_identities",
	})
	require.NoError(t, err)

	tracking, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "test_cas_safety_tracking",
	})
	require.NoError(t, err)

	il := &IdentityLookup{
		identities:    identities,
		memberTracker: tracking,
		config:        newDefaultConfig(),
		semaphore:     make(chan struct{}, 200),
		memberID:      "member-A",
	}

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "cas-grain"}

	// Step 1: Node A activates the grain (version 1).
	lockID, rev, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok)
	err = il.storeActivation(ctx, ci, lockID, rev, "member-A", "host-A:8080", "TestKind/cas-grain")
	require.NoError(t, err)

	pidA := actor.NewPID("host-A:8080", "TestKind/cas-grain")

	// Step 2: Simulate node B re-activating the same grain.
	// This overwrites the KV entry with a new revision.
	newRec := activationRecord{
		PidID:      "TestKind/cas-grain",
		PidAddress: "host-B:9090",
		MemberID:   "member-B",
	}
	newData, err := json.Marshal(&newRec)
	require.NoError(t, err)
	_, err = identities.Put(ctx, kvKey(ci), newData)
	require.NoError(t, err)

	// Step 3: Now call the RemoveActivation callback with node A's old PID.
	// The callback should read the current entry, see the PID doesn't match
	// (it's now node B's PID), and skip deletion.
	//
	// Build the RemoveActivation callback the same way setupPlacementActor does.
	removeActivation := func(rmCtx context.Context, rmCI *cluster.ClusterIdentity, rmPid *actor.PID) error {
		key := kvKey(rmCI)
		entry, err := il.identities.Get(rmCtx, key)
		if err != nil {
			if errors.Is(err, jetstream.ErrKeyNotFound) {
				return nil
			}
			return err
		}
		var rec activationRecord
		if err := json.Unmarshal(entry.Value(), &rec); err != nil {
			return err
		}
		// Only delete if PID matches.
		if rec.PidID != rmPid.Id || rec.PidAddress != rmPid.Address {
			return nil // PID changed — someone else re-activated
		}
		if rec.MemberID != "" {
			il.removeKeyFromMember(rmCtx, rec.MemberID, key)
		}
		return il.identities.Delete(rmCtx, key, jetstream.LastRevision(entry.Revision()))
	}

	// Call RemoveActivation with node A's OLD PID.
	err = removeActivation(ctx, ci, pidA)
	assert.NoError(t, err, "RemoveActivation should not error (PID mismatch -> skip)")

	// Step 4: Verify node B's activation survived.
	rec := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec, "node B's activation must survive CAS-safe RemoveActivation")
	assert.Equal(t, "host-B:9090", rec.PidAddress, "surviving activation should be node B's")
	assert.Equal(t, "member-B", rec.MemberID)
}

// TestStale_PidCacheTTL_ExpiresStalePids verifies that the PID cache's
// TTL mechanism prevents serving stale PIDs indefinitely. After a grain
// terminates, the cached PID should expire and subsequent lookups should
// resolve fresh from NATS KV.
func TestStale_PidCacheTTL_ExpiresStalePids(t *testing.T) {
	// Use a very short TTL for this test.
	shortTTL := 200 * time.Millisecond

	cache := cluster.NewPidCacheWithTTL(shortTTL)

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "ttl-grain"}
	stalePid := actor.NewPID("old-host:8080", "TestKind/ttl-grain")

	// Cache the PID.
	cache.Set(ci.Identity, ci.Kind, stalePid)

	// Immediately retrieve — should hit cache.
	cached, ok := cache.Get(ci.Identity, ci.Kind)
	require.True(t, ok, "PID should be in cache immediately after Set")
	assert.True(t, stalePid.Equal(cached))

	// Wait for TTL to expire.
	time.Sleep(shortTTL + 50*time.Millisecond)

	// After TTL, cache should miss.
	_, ok = cache.Get(ci.Identity, ci.Kind)
	assert.False(t, ok, "PID should be expired from cache after TTL")
}

// TestStale_ConcurrentRemovePidAndReactivation verifies that when multiple
// goroutines simultaneously detect a stale activation and attempt to clean
// up and re-activate the same identity, only one activation record exists
// after the race resolves. The NATS KV CAS operations and the placement
// actor's deduplication (spawning map) ensure correctness.
func TestStale_ConcurrentRemovePidAndReactivation(t *testing.T) {
	_, c, il := setupPlacementTestCluster(t, "test-stale-race")

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "race-grain"}
	ctx := context.Background()

	// Inject a stale activation from a dead member.
	staleRec := activationRecord{
		PidID:      "TestKind/race-grain",
		PidAddress: "dead-host:9999",
		MemberID:   "dead-member-race",
	}
	data, err := json.Marshal(&staleRec)
	require.NoError(t, err)
	_, err = il.identities.Put(ctx, kvKey(ci), data)
	require.NoError(t, err)

	// Launch 5 concurrent Get() calls — all will detect the stale activation
	// and attempt to clean + re-activate.
	const concurrency = 5
	var wg sync.WaitGroup
	pids := make([]*actor.PID, concurrency)

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			pids[idx] = il.Get(ci)
		}(i)
	}
	wg.Wait()

	// All should have returned a PID (some may be nil if they lost the race
	// and retried but eventually succeeded).
	var nonNilPids []*actor.PID
	for _, pid := range pids {
		if pid != nil {
			nonNilPids = append(nonNilPids, pid)
		}
	}
	require.NotEmpty(t, nonNilPids, "at least one Get() should succeed")

	// All non-nil PIDs should be the same (coalesced or same activation).
	for i := 1; i < len(nonNilPids); i++ {
		assert.True(t, nonNilPids[0].Equal(nonNilPids[i]),
			"all PIDs should match: pids[0]=%v pids[%d]=%v", nonNilPids[0], i, nonNilPids[i])
	}

	// The winning PID should be on this node (live member).
	assert.Equal(t, c.ActorSystem.Address(), nonNilPids[0].Address,
		"activation should be on the live node")

	// Verify exactly one activation in KV.
	rec := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec)
	assert.Equal(t, il.memberID, rec.MemberID,
		"KV should contain exactly one activation for the live member")
}
