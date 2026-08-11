package natskv

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/cluster"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestActivationGraceWindow verifies the A2.3 grace-window behaviour for
// activations whose owning member is absent.
//
// Timeline (via il.now):
//
//	t0        -- stale record injected; first Get() observes absent owner
//	t0+59s    -- second Get() still within grace (< 60s) -> returns PID uncached
//	t0+61s    -- third Get() grace has elapsed -> CAS-deletes + re-activates
func TestActivationGraceWindow(t *testing.T) {
	_, c, il := setupPlacementTestCluster(t, "test-grace-window")
	ctx := context.Background()

	// Freeze time at t0.
	t0 := time.Now()
	il.now = func() time.Time { return t0 }

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "grace-1"}
	stalePid := "TestKind/grace-1"
	staleAddr := "dead-host:9999"

	// Inject stale activation from a dead member.
	staleRec := activationRecord{
		PidID:      stalePid,
		PidAddress: staleAddr,
		MemberID:   "dead-member-grace",
	}
	data, err := json.Marshal(&staleRec)
	require.NoError(t, err)
	_, err = il.identities.Put(ctx, kvKey(ci), data)
	require.NoError(t, err)

	// t0: first Get() — within grace (first observation records t0).
	pid0 := il.Get(ci)
	require.NotNil(t, pid0, "first Get() must return the stale PID (within grace)")
	assert.Equal(t, staleAddr, pid0.Address, "first Get() should return stale PID address")

	// Confirm the KV record is still present (no cleanup yet).
	rec0 := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec0, "activation record must still exist within grace")
	assert.Equal(t, "dead-member-grace", rec0.MemberID)

	// Confirm stale PID was NOT put into PidCache.
	_, inCache := il.cluster.PidCache.Get(ci.Identity, ci.Kind)
	assert.False(t, inCache, "stale PID must NOT be added to PidCache during grace")

	// t0+59s: still within grace.
	il.now = func() time.Time { return t0.Add(59 * time.Second) }
	pid1 := il.Get(ci)
	require.NotNil(t, pid1, "second Get() at t0+59s must still return stale PID")
	assert.Equal(t, staleAddr, pid1.Address, "second Get() should still return stale PID address")

	rec1 := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec1, "activation record must still exist at t0+59s")

	_, inCache = il.cluster.PidCache.Get(ci.Identity, ci.Kind)
	assert.False(t, inCache, "stale PID must NOT be in PidCache at t0+59s")

	// t0+61s: grace elapsed — cleanup + re-activate.
	il.now = func() time.Time { return t0.Add(61 * time.Second) }
	pid2 := il.Get(ci)
	require.NotNil(t, pid2, "Get() at t0+61s must return a new PID after cleanup")
	assert.Equal(t, c.ActorSystem.Address(), pid2.Address,
		"new activation must be on the live node, not the dead member")

	// Confirm the old stale record is gone and a new one written.
	rec2 := il.getExistingActivation(ctx, ci)
	require.NotNil(t, rec2)
	assert.Equal(t, il.memberID, rec2.MemberID,
		"KV record after grace-elapsed cleanup must belong to the live member")
}

// TestAbsenceClockResetsOnPresence verifies that when a member re-appears
// (validated present) the absence clock entry is cleared, so a subsequent
// absence starts a fresh grace window rather than inheriting the old one.
func TestAbsenceClockResetsOnPresence(t *testing.T) {
	_, _, il := setupPlacementTestCluster(t, "test-grace-reset")
	ctx := context.Background()

	t0 := time.Now()
	il.now = func() time.Time { return t0 }

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "reset-grain"}
	// Use the live member's own memberID so ValidateActivationMember returns true
	// when we write the "present" record.
	liveMember := il.memberID

	// Phase 1: inject stale record (absent owner at t0).
	staleRec := activationRecord{
		PidID:      "TestKind/reset-grain",
		PidAddress: "dead-host:1234",
		MemberID:   "dead-member-reset",
	}
	data, _ := json.Marshal(&staleRec)
	_, err := il.identities.Put(ctx, kvKey(ci), data)
	require.NoError(t, err)

	// First Get() at t0 — records first-absent = t0.
	pid0 := il.Get(ci)
	require.NotNil(t, pid0)
	assert.Equal(t, "dead-host:1234", pid0.Address, "should return stale PID within grace")

	// Verify absence was recorded.
	il.absence.mu.Lock()
	_, recorded := il.absence.obs[kvKey(ci)]
	il.absence.mu.Unlock()
	assert.True(t, recorded, "absence must be recorded after first stale Get()")

	// Phase 2: overwrite with a live record (member re-appears).
	// We write directly because the live member IS in the member list.
	liveRec := activationRecord{
		PidID:      "TestKind/reset-grain",
		PidAddress: il.cluster.ActorSystem.Address(),
		MemberID:   liveMember,
	}
	liveData, _ := json.Marshal(&liveRec)
	_, err = il.identities.Put(ctx, kvKey(ci), liveData)
	require.NoError(t, err)

	// Get() at t0+30s — owner is now present; absence must clear.
	// First evict any stale PidCache entry so Get() reaches resolveIdentity.
	il.cluster.PidCache.Remove(ci.Identity, ci.Kind)
	il.now = func() time.Time { return t0.Add(30 * time.Second) }
	pidLive := il.Get(ci)
	require.NotNil(t, pidLive)
	assert.Equal(t, il.cluster.ActorSystem.Address(), pidLive.Address,
		"should return live PID when owner is present")

	il.absence.mu.Lock()
	_, stillRecorded := il.absence.obs[kvKey(ci)]
	il.absence.mu.Unlock()
	assert.False(t, stillRecorded, "absence must be cleared when owner is present")

	// Phase 3: inject second stale record. The grace window must restart from t0+30s.
	// Clear the PidCache so Get() does not hit the cached live PID from phase 2.
	il.cluster.PidCache.Remove(ci.Identity, ci.Kind)
	staleRec2 := activationRecord{
		PidID:      "TestKind/reset-grain",
		PidAddress: "dead-host:5678",
		MemberID:   "dead-member-reset-2",
	}
	data2, _ := json.Marshal(&staleRec2)
	_, err = il.identities.Put(ctx, kvKey(ci), data2)
	require.NoError(t, err)

	// Get() at t0+89s: if absence were NOT reset, elapsed = 89s > 60s -> cleanup.
	// Since it WAS reset at t0+30s, elapsed from first-absent-again (also at t0+89s since
	// this is the first observation of the second absence) = 0s < 60s -> still grace.
	// NOTE: t0+89s is the first-absent observation for the second absence, so firstSeen = t0+89s,
	// elapsed = 0 < 60s -> grace applies, stale PID returned.
	il.now = func() time.Time { return t0.Add(89 * time.Second) }
	pid2 := il.Get(ci)
	require.NotNil(t, pid2)
	assert.Equal(t, "dead-host:5678", pid2.Address,
		"second absence must NOT trigger cleanup before new grace elapses")
}

// TestAbsenceMapEviction verifies the three eviction conditions for the absence map.
func TestAbsenceMapEviction(t *testing.T) {
	_, _, il := setupPlacementTestCluster(t, "test-grace-eviction")
	ctx := context.Background()

	t0 := time.Now()
	il.now = func() time.Time { return t0 }

	// --- Case 1: evicted when member becomes present ---
	ci1 := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "evict-present"}

	stale1 := activationRecord{
		PidID: "TestKind/evict-present", PidAddress: "dead:9999", MemberID: "dead-ev1",
	}
	d1, _ := json.Marshal(&stale1)
	_, err := il.identities.Put(ctx, kvKey(ci1), d1)
	require.NoError(t, err)
	il.Get(ci1) // records absence

	il.absence.mu.Lock()
	_, ok1 := il.absence.obs[kvKey(ci1)]
	il.absence.mu.Unlock()
	assert.True(t, ok1, "case 1: absence must be recorded")

	// Overwrite with live record.
	liveRec1 := activationRecord{
		PidID:      "TestKind/evict-present",
		PidAddress: il.cluster.ActorSystem.Address(),
		MemberID:   il.memberID,
	}
	ld1, _ := json.Marshal(&liveRec1)
	_, err = il.identities.Put(ctx, kvKey(ci1), ld1)
	require.NoError(t, err)
	il.Get(ci1)

	il.absence.mu.Lock()
	_, ok1after := il.absence.obs[kvKey(ci1)]
	il.absence.mu.Unlock()
	assert.False(t, ok1after, "case 1: absence must be cleared when member present")

	// --- Case 2: evicted when record is entirely absent ---
	ci2 := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "evict-absent-record"}

	stale2 := activationRecord{
		PidID: "TestKind/evict-absent-record", PidAddress: "dead:9999", MemberID: "dead-ev2",
	}
	d2, _ := json.Marshal(&stale2)
	_, err = il.identities.Put(ctx, kvKey(ci2), d2)
	require.NoError(t, err)
	il.Get(ci2) // records absence

	il.absence.mu.Lock()
	_, ok2 := il.absence.obs[kvKey(ci2)]
	il.absence.mu.Unlock()
	assert.True(t, ok2, "case 2: absence must be recorded")

	// Delete the KV record entirely.
	err = il.identities.Delete(ctx, kvKey(ci2))
	require.NoError(t, err)
	il.Get(ci2) // record-absent path -> should clear absence

	il.absence.mu.Lock()
	_, ok2after := il.absence.obs[kvKey(ci2)]
	il.absence.mu.Unlock()
	assert.False(t, ok2after, "case 2: absence must be cleared when KV record is gone")

	// --- Case 3: evicted when grace-based cleanup fires ---
	ci3 := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "evict-cleanup-fired"}

	stale3 := activationRecord{
		PidID: "TestKind/evict-cleanup-fired", PidAddress: "dead:9999", MemberID: "dead-ev3",
	}
	d3, _ := json.Marshal(&stale3)
	_, err = il.identities.Put(ctx, kvKey(ci3), d3)
	require.NoError(t, err)

	il.Get(ci3) // records absence at t0

	// Advance past grace.
	il.now = func() time.Time { return t0.Add(61 * time.Second) }
	il.Get(ci3) // grace elapsed -> cleanup + re-activate -> clears absence

	il.absence.mu.Lock()
	_, ok3after := il.absence.obs[kvKey(ci3)]
	il.absence.mu.Unlock()
	assert.False(t, ok3after, "case 3: absence must be cleared after grace-elapsed cleanup")
}
