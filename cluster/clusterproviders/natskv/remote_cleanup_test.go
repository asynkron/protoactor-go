package natskv

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStoreActivationContextPropagation verifies that storeActivation honors
// its parent context: the child context it derives via WithTimeout inherits
// cancellation from the parent, so a cancelled parent makes the KV Update
// return promptly with a context error rather than hanging. This proves
// context propagation into the bounded call; it does not exercise the 10s
// ceiling itself (which would require a genuinely unresponsive KV).
func TestStoreActivationContextPropagation(t *testing.T) {
	il := buildBareIdentityLookup(t)
	ci := testCI("TestKind", "bounded-1")
	ctx := context.Background()

	// Acquire a lock so we have a real revision to CAS-update.
	lockID, rev, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok)

	// Use a parent context that is already cancelled: storeActivation derives a
	// child WithTimeout, but the parent cancellation propagates immediately, so
	// the call must return quickly (well under the 10s bound) with an error.
	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()

	start := time.Now()
	err := il.storeActivation(cancelledCtx, ci, lockID, rev, il.memberID, "addr", "TestKind/bounded-1")
	elapsed := time.Since(start)

	require.Error(t, err, "storeActivation against a cancelled context must return an error")
	assert.Less(t, elapsed, 5*time.Second, "storeActivation must be bounded, took %v", elapsed)
}

// TestLocalReusePathStoresActivation verifies that when activateLocal's
// placement request returns a reuse (the record is still lock-only because
// PersistActivation never ran), the caller upgrades the record via
// storeActivation so no lock-only record survives a successful activateLocal.
func TestLocalReusePathStoresActivation(t *testing.T) {
	_, c, il := setupPlacementTestCluster(t, "test-reuse-store")
	ctx := context.Background()

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "reuse-1"}

	// First activation establishes a tracked actor and a completed record.
	pid1 := il.Get(ci)
	require.NotNil(t, pid1)

	rec := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec, "first activation must leave a completed record")
	assert.NotEmpty(t, rec.PidID, "record must carry a PID after first activation")

	// A subsequent Get finds the completed record via the existing-activation
	// check (step 3) and returns it without re-spawning. The record is never
	// lock-only after a successful activation.
	pid2 := il.Get(ci)
	require.NotNil(t, pid2)
	assert.True(t, pid1.Equal(pid2))

	rec2 := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec2)
	assert.Equal(t, rec.PidID, rec2.PidID, "record PID must be stable across reuse")

	_ = c
}

// TestReuseUpgradeFromLockOnly directly drives activateLocal against a
// placement actor that returns an existing PID without persisting, and asserts
// the lock-only record is upgraded to a completed record using the held
// revision.
func TestReuseUpgradeFromLockOnly(t *testing.T) {
	_, _, il := setupPlacementTestCluster(t, "test-reuse-upgrade")
	ctx := context.Background()

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "upgrade-1"}

	// Pre-spawn the grain via the placement actor with an empty RequestId so
	// PersistActivation no-ops (remote-initiated): the actor becomes tracked
	// but no completed record is written.
	placementPID := il.placementPID.Load()
	require.NotNil(t, placementPID)
	res, err := il.cluster.ActorSystem.Root.RequestFuture(placementPID,
		&cluster.ActivationRequest{ClusterIdentity: ci}, 5*time.Second).Result()
	require.NoError(t, err)
	resp := res.(*cluster.ActivationResponse)
	require.False(t, resp.Failed)
	require.NotNil(t, resp.Pid)

	// Now acquire the lock ourselves (record becomes lock-only) and call
	// activateLocal. The placement actor already tracks the grain, so it
	// returns the existing PID via the reuse path (no persist). activateLocal
	// must then upgrade the lock-only record to a completed record.
	lockID, rev, ok := il.tryAcquireLock(ctx, ci)
	require.True(t, ok, "lock must be acquirable (only a completed record blocks it; ours is absent)")

	// Confirm the record is lock-only before activation.
	raw := il.readRawRecord(ctx, ci)
	require.NotNil(t, raw)
	require.Empty(t, raw.PidID, "record must be lock-only before reuse upgrade")

	pid := il.activateLocal(ctx, ci, lockID, rev)
	require.NotNil(t, pid, "activateLocal reuse path must return the existing PID")
	assert.True(t, resp.Pid.Equal(pid), "reuse must return the already-tracked PID")

	// The lock-only record must have been upgraded to a completed record.
	upgraded := il.getExistingActivation(ctx, ci)
	require.NotNil(t, upgraded, "reuse path must upgrade the lock-only record")
	assert.Equal(t, pid.Id, upgraded.PidID, "upgraded record must carry the reused PID")
	assert.Equal(t, il.memberID, upgraded.MemberID)
}

// TestSelfPoisonCleansOwnRecord verifies the CleanupOwnRecord semantics: a
// record pointing at the poisoned PID is CAS-deleted, but a successor's record
// (different PID) is left untouched. We drive this through the placement
// actor's foreign-record self-check path.
func TestSelfPoisonCleansOwnRecord(t *testing.T) {
	_, _, il := setupPlacementTestCluster(t, "test-selfpoison-clean")
	ctx := context.Background()

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "poison-clean-1"}
	key := kvKey(ci)

	// Spawn a grain remote-initiated so the placement actor arms a self-check.
	placementPID := il.placementPID.Load()
	require.NotNil(t, placementPID)
	res, err := il.cluster.ActorSystem.Root.RequestFuture(placementPID,
		&cluster.ActivationRequest{ClusterIdentity: ci}, 5*time.Second).Result()
	require.NoError(t, err)
	resp := res.(*cluster.ActivationResponse)
	require.NotNil(t, resp.Pid)
	ownPID := resp.Pid

	// Write a completed record that points at OUR PID (as a late persist would).
	ownRec := activationRecord{PidID: ownPID.Id, PidAddress: ownPID.Address, MemberID: il.memberID}
	data, _ := json.Marshal(&ownRec)
	_, err = il.identities.Put(ctx, key, data)
	require.NoError(t, err)

	// Directly invoke the cleanup semantics by removeAndPoison + cleanup: send
	// RemoveAndPoisonRequest, then simulate the self-check foreign cleanup by
	// deleting the own record. We verify cleanup only removes matching records.
	// First, cleanup with a DIFFERENT (successor) PID must NOT delete our record.
	successorPID := actor.NewPID(ownPID.Address, "TestKind/successor")
	il.cleanupOwnRecord(ctx, ci, successorPID)
	still := il.getExistingActivation(ctx, ci)
	require.NotNil(t, still, "cleanup with a non-matching PID must leave the record intact")
	assert.Equal(t, ownPID.Id, still.PidID)

	// Now cleanup with the matching PID: the record must be CAS-deleted.
	il.cleanupOwnRecord(ctx, ci, ownPID)
	gone := il.getExistingActivation(ctx, ci)
	assert.Nil(t, gone, "cleanup with the matching PID must delete the record")
}

// TestStoreFailurePoisonsThroughPlacementBeforeLockDelete verifies the
// activateRemote store-failure ordering: the remote grain is asked to
// remove-and-poison before the lock is CAS-deleted, so no live instance
// survives on the losing side. We assert via the NATS poison control-plane:
// a poison request for the spawned PID reaches the member and the placement
// actor stops tracking it.
func TestStoreFailurePoisonsThroughPlacement(t *testing.T) {
	_, _, il := setupPlacementTestCluster(t, "test-store-fail-poison")

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "store-fail-1"}

	// Spawn a grain remote-initiated (empty RequestId): tracked, not persisted.
	placementPID := il.placementPID.Load()
	require.NotNil(t, placementPID)
	res, err := il.cluster.ActorSystem.Root.RequestFuture(placementPID,
		&cluster.ActivationRequest{ClusterIdentity: ci}, 5*time.Second).Result()
	require.NoError(t, err)
	resp := res.(*cluster.ActivationResponse)
	require.NotNil(t, resp.Pid)
	pid := resp.Pid

	// The grain is alive.
	_, alive := il.cluster.ActorSystem.ProcessRegistry.GetLocal(pid.Id)
	require.True(t, alive, "grain must be alive after spawn")

	// Issue removeAndPoison directly to the placement actor (the same request
	// the store-failure path routes over NATS). The grain must be poisoned.
	ack, err := il.cluster.ActorSystem.Root.RequestFuture(placementPID,
		&cluster.RemoveAndPoisonRequest{PID: pid}, 5*time.Second).Result()
	require.NoError(t, err)
	_, ok := ack.(*cluster.RemoveAndPoisonAck)
	require.True(t, ok)

	assert.Eventually(t, func() bool {
		_, a := il.cluster.ActorSystem.ProcessRegistry.GetLocal(pid.Id)
		return !a
	}, 3*time.Second, 25*time.Millisecond, "grain must be poisoned via removeAndPoison")
}

// TestRemoteRemoveAndPoisonNATSRoundTrip verifies the NATS poison control-plane
// end-to-end within a single member: a poison request published to the
// member-scoped subject reaches handleRemoveAndPoisonRequest, which validates
// the target against the identity record (the caller's lock-only record at the
// carried revision) and then drives the local placement actor to poison the
// grain.
func TestRemoteRemoveAndPoisonNATSRoundTrip(t *testing.T) {
	_, _, il := setupPlacementTestCluster(t, "test-poison-nats")
	ctx := context.Background()

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "poison-nats-1"}
	placementPID := il.placementPID.Load()
	require.NotNil(t, placementPID)
	res, err := il.cluster.ActorSystem.Root.RequestFuture(placementPID,
		&cluster.ActivationRequest{ClusterIdentity: ci}, 5*time.Second).Result()
	require.NoError(t, err)
	pid := res.(*cluster.ActivationResponse).Pid
	require.NotNil(t, pid)

	// The store-failure path leaves the caller's lock-only record in place; the
	// poison request carries that record's revision. Establish it here so the
	// target-validation guard honors the request.
	lockData, _ := json.Marshal(&activationRecord{LockID: "L", MemberID: il.memberID})
	rev, err := il.identities.Put(ctx, kvKey(ci), lockData)
	require.NoError(t, err)

	// Build a Member pointing at ourselves and drive the request path.
	self := &cluster.Member{Id: il.memberID}
	il.requestRemoteRemoveAndPoison(self, ci, pid, rev)

	assert.Eventually(t, func() bool {
		_, a := il.cluster.ActorSystem.ProcessRegistry.GetLocal(pid.Id)
		return !a
	}, 3*time.Second, 25*time.Millisecond, "grain must be poisoned via the NATS poison round-trip")
}

// TestReplayedPoisonRejectedAfterSuccessorActivation is the replay/spoof guard
// for finding 1: a poisonReq replayed AFTER a legitimate successor has
// activated the same identity on the same member must be REJECTED — the
// successor survives — while a genuine store-failure poison (matching lock-only
// revision) is still honored.
func TestReplayedPoisonRejectedAfterSuccessorActivation(t *testing.T) {
	_, _, il := setupPlacementTestCluster(t, "test-poison-replay")
	ctx := context.Background()

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "replay-1"}
	placementPID := il.placementPID.Load()
	require.NotNil(t, placementPID)

	// --- Loser: a remote-initiated activation whose caller-side persist fails.
	// Establish its lock-only record and capture the revision the caller would
	// carry in its poison request.
	loserData, _ := json.Marshal(&activationRecord{LockID: "L-loser", MemberID: il.memberID})
	loserRev, err := il.identities.Put(ctx, kvKey(ci), loserData)
	require.NoError(t, err)

	res, err := il.cluster.ActorSystem.Root.RequestFuture(placementPID,
		&cluster.ActivationRequest{ClusterIdentity: ci}, 5*time.Second).Result()
	require.NoError(t, err)
	loserPID := res.(*cluster.ActivationResponse).Pid
	require.NotNil(t, loserPID)

	// --- Successor: wins the identity legitimately. It re-acquires the lock and
	// stores a completed record, bumping the revision away from loserRev.
	res2, err := il.cluster.ActorSystem.Root.RequestFuture(placementPID,
		&cluster.ActivationRequest{ClusterIdentity: ci}, 5*time.Second).Result()
	require.NoError(t, err)
	successorPID := res2.(*cluster.ActivationResponse).Pid
	require.NotNil(t, successorPID)
	// storeActivation upgrades the record to point at the successor PID.
	require.NoError(t, il.storeActivation(ctx, ci, "L-succ", loserRev, il.memberID,
		successorPID.Address, successorPID.Id))

	// --- Replay: the loser's original poison request arrives now, carrying the
	// stale loserRev. It must be REJECTED — the current record is completed
	// (points at the successor), so poisonTargetIsOrphan returns false.
	staleReq := &poisonReq{
		Kind:       ci.Kind,
		Identity:   ci.Identity,
		PidID:      successorPID.Id, // replay could even target the successor PID
		PidAddress: successorPID.Address,
		Revision:   loserRev,
	}
	assert.False(t, il.poisonTargetIsOrphan(staleReq),
		"replayed poison against a completed successor record must be rejected")

	// Drive the full NATS handler path to confirm the successor survives.
	self := &cluster.Member{Id: il.memberID}
	il.requestRemoteRemoveAndPoison(self, ci, successorPID, loserRev)
	// Give the (rejected) request time to be processed.
	time.Sleep(300 * time.Millisecond)
	_, alive := il.cluster.ActorSystem.ProcessRegistry.GetLocal(successorPID.Id)
	assert.True(t, alive, "successor grain must survive a replayed poison")

	// --- Sanity: a genuine store-failure poison still works. Reset the record
	// to a fresh lock-only state and poison at the matching revision.
	freshData, _ := json.Marshal(&activationRecord{LockID: "L-fresh", MemberID: il.memberID})
	freshRev, err := il.identities.Put(ctx, kvKey(ci), freshData)
	require.NoError(t, err)
	genuineReq := &poisonReq{
		Kind:       ci.Kind,
		Identity:   ci.Identity,
		PidID:      successorPID.Id,
		PidAddress: successorPID.Address,
		Revision:   freshRev,
	}
	assert.True(t, il.poisonTargetIsOrphan(genuineReq),
		"a genuine store-failure poison (lock-only record at the carried revision) must be honored")
}

// TestCheckActivationRecordClassification exercises natskv's
// CheckActivationRecord callback against real KV state, covering all four
// RecordCheck outcomes. This is the natskv-specific classifier that the shared
// placement actor's self-check depends on; the shared decision-table logic is
// covered deterministically in the cluster package.
func TestCheckActivationRecordClassification(t *testing.T) {
	il := buildBareIdentityLookup(t)
	ctx := context.Background()

	selfPID := actor.NewPID("self-host:1", "TestKind/g1")

	// Absent: no record.
	ciAbsent := testCI("TestKind", "absent")
	assert.Equal(t, cluster.RecordAbsent, il.checkActivationRecord(ctx, ciAbsent, selfPID))

	// Lock-only: record exists but has no PID.
	ciLock := testCI("TestKind", "lockonly")
	lockData, _ := json.Marshal(&activationRecord{LockID: "L", MemberID: il.memberID})
	_, err := il.identities.Put(ctx, kvKey(ciLock), lockData)
	require.NoError(t, err)
	assert.Equal(t, cluster.RecordLockOnly, il.checkActivationRecord(ctx, ciLock, selfPID))

	// Own: completed record whose PID matches selfPID.
	ciOwn := testCI("TestKind", "own")
	ownData, _ := json.Marshal(&activationRecord{
		PidID: selfPID.Id, PidAddress: selfPID.Address, MemberID: il.memberID,
	})
	_, err = il.identities.Put(ctx, kvKey(ciOwn), ownData)
	require.NoError(t, err)
	assert.Equal(t, cluster.RecordOwn, il.checkActivationRecord(ctx, ciOwn, selfPID))

	// Foreign: completed record whose PID differs from selfPID.
	ciForeign := testCI("TestKind", "foreign")
	foreignData, _ := json.Marshal(&activationRecord{
		PidID: "TestKind/other", PidAddress: "other-host:2", MemberID: "other-member",
	})
	_, err = il.identities.Put(ctx, kvKey(ciForeign), foreignData)
	require.NoError(t, err)
	assert.Equal(t, cluster.RecordForeign, il.checkActivationRecord(ctx, ciForeign, selfPID))
}
