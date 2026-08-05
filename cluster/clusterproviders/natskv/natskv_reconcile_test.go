package natskv

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReconcile_PrunesStaleMemberAfterTwoMisses verifies that reconcileMembers
// removes a member that is present in the in-memory p.members map but has no
// corresponding key in the KV bucket (e.g. its TTL expired but the delete
// marker was never delivered to the watcher).
//
// Pruning is deliberately gated on TWO consecutive reconcile passes finding the
// member absent from the live KV key set. A member that is absent for only a
// single pass (which is what a member that joins concurrently with a reconcile
// snapshot looks like) is retained; only a member absent in two successive
// passes is a genuine ghost and is pruned. This eliminates the false-prune race
// that would otherwise permanently block a live node via the member block list.
func TestReconcile_PrunesStaleMemberAfterTwoMisses(t *testing.T) {
	srv := startEmbeddedNATS(t)
	// Disable the reconcile ticker so this test's manual reconcileMembers()
	// calls are the only reconcile activity (no concurrent goroutine touching
	// reconcileMisses).
	p, c := setupCluster(t, srv, "test-reconcile-prune", WithReconcileInterval(-1))

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	// Inject a stale member directly into p.members. This member has NO
	// corresponding KV key -- it was never written to the bucket -- so it
	// simulates a member whose key TTL-expired without a delivered delete.
	staleID := "test-reconcile-prune_stale-ghost"
	staleNode := NewNode(staleID, "10.0.0.99", 6666, []string{"ghost"})

	p.membersMu.Lock()
	p.members[staleID] = staleNode
	p.membersMu.Unlock()

	// First reconcile pass: the member is absent from KV for the FIRST time, so
	// the grace rule retains it.
	require.NoError(t, p.reconcileMembers())

	p.membersMu.RLock()
	_, afterFirst := p.members[staleID]
	p.membersMu.RUnlock()
	require.True(t, afterFirst,
		"stale member must be retained after a single missed pass (race grace)")

	// Second reconcile pass: absent for the second consecutive time -> pruned.
	require.NoError(t, p.reconcileMembers())

	p.membersMu.RLock()
	_, afterSecond := p.members[staleID]
	selfID := p.self.ID
	p.membersMu.RUnlock()

	assert.False(t, afterSecond,
		"stale member absent for two consecutive passes should be pruned")
	assert.NotEmpty(t, selfID, "self identity should be retained after reconcile")
}

// TestReconcile_RetainsLiveMemberWithKVKey verifies that reconcileMembers does
// NOT prune a member that has a live KV key, across repeated passes, while a
// ghost with no KV key is eventually pruned.
func TestReconcile_RetainsLiveMemberWithKVKey(t *testing.T) {
	srv := startEmbeddedNATS(t)

	// p1's reconcile ticker is disabled so its manual reconcileMembers() calls
	// are the only reconcile activity.
	p1, c1 := setupCluster(t, srv, "test-reconcile-retain", WithReconcileInterval(-1))
	require.NoError(t, p1.StartMember(c1))
	t.Cleanup(func() { _ = p1.Shutdown(true) })

	// Second live member sharing the same cluster/bucket.
	p2, c2 := setupCluster(t, srv, "test-reconcile-retain")
	require.NoError(t, p2.StartMember(c2))
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	// Wait until p1 discovers p2 (real KV key + watcher).
	require.Eventually(t, func() bool {
		p1.membersMu.RLock()
		_, ok := p1.members[p2.self.ID]
		p1.membersMu.RUnlock()
		return ok
	}, 10*time.Second, 100*time.Millisecond, "p1 should discover live member p2")

	// Inject a stale ghost that has no KV key.
	staleID := "test-reconcile-retain_stale-ghost"
	p1.membersMu.Lock()
	p1.members[staleID] = NewNode(staleID, "10.0.0.99", 6666, nil)
	p1.membersMu.Unlock()

	// Two passes so the ghost clears the grace window and is pruned.
	require.NoError(t, p1.reconcileMembers())
	require.NoError(t, p1.reconcileMembers())

	p1.membersMu.RLock()
	_, staleStillPresent := p1.members[staleID]
	_, liveRetained := p1.members[p2.self.ID]
	p1.membersMu.RUnlock()

	assert.False(t, staleStillPresent, "stale member should be pruned after two passes")
	assert.True(t, liveRetained, "live member with a real KV key must be retained")
}

// TestReconcile_UpsertsLiveMemberMissingFromMemory verifies that reconcile is
// bidirectional: a member whose KV key is present but which is absent from the
// in-memory map (e.g. the watcher died or missed the Put) is re-added by
// reconcile. This hedges the "dead watcher" failure mode where the watcher
// stops delivering both deletes AND adds.
func TestReconcile_UpsertsLiveMemberMissingFromMemory(t *testing.T) {
	srv := startEmbeddedNATS(t)

	// p1's reconcile ticker is disabled so its manual reconcileMembers() call is
	// the only reconcile activity.
	p1, c1 := setupCluster(t, srv, "test-reconcile-upsert", WithReconcileInterval(-1))
	require.NoError(t, p1.StartMember(c1))
	t.Cleanup(func() { _ = p1.Shutdown(true) })

	// p2 registers with a long TTL and a long refresh interval: its key stays
	// present in KV for the whole test, but it will NOT re-Put during the test,
	// so p1's watcher cannot re-add it after we remove it below. That isolates
	// the upsert to reconcile.
	p2, c2 := setupCluster(t, srv, "test-reconcile-upsert",
		WithMemberTTL(60*time.Second),
		WithRefreshInterval(60*time.Second))
	require.NoError(t, p2.StartMember(c2))
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	// Wait until p1 discovers p2 via the watcher's initial Put.
	require.Eventually(t, func() bool {
		p1.membersMu.RLock()
		_, ok := p1.members[p2.self.ID]
		p1.membersMu.RUnlock()
		return ok
	}, 10*time.Second, 100*time.Millisecond, "p1 should discover live member p2")

	// Simulate a missed Put / dead watcher: drop p2 from p1's in-memory map
	// while p2's KV key remains live.
	p1.membersMu.Lock()
	delete(p1.members, p2.self.ID)
	p1.membersMu.Unlock()

	// Reconcile must re-add p2 from its live KV key.
	require.NoError(t, p1.reconcileMembers())

	p1.membersMu.RLock()
	_, readded := p1.members[p2.self.ID]
	p1.membersMu.RUnlock()

	assert.True(t, readded,
		"reconcile should re-add a live member whose KV key exists but was missing from memory")
}

// TestReconcile_RetainsMemberThatJoinsBetweenPasses verifies the core grace
// invariant against the concurrent-join scenario the two-pass rule exists for:
// a member that is absent from one reconcile snapshot but whose KV key is
// present by the next must never be pruned. This is what a member joining
// concurrently with a reconcile pass looks like.
func TestReconcile_RetainsMemberThatJoinsBetweenPasses(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-reconcile-join", WithReconcileInterval(-1))
	require.NoError(t, p.StartMember(c))
	t.Cleanup(func() { _ = p.Shutdown(true) })

	// Inject a member that has NO KV key yet -- it is mid-join, absent from the
	// bucket at the first snapshot.
	joinID := "test-reconcile-join_late-joiner"
	joinNode := NewNode(joinID, "10.0.0.50", 7000, nil)
	p.membersMu.Lock()
	p.members[joinID] = joinNode
	p.membersMu.Unlock()

	// First pass: absent from KV -> recorded as a single miss, retained by grace.
	require.NoError(t, p.reconcileMembers())
	p.membersMu.RLock()
	_, afterFirst := p.members[joinID]
	p.membersMu.RUnlock()
	require.True(t, afterFirst, "member absent for one pass must be retained (grace)")

	// The member's registration lands (its Put becomes durable in the bucket).
	data, err := joinNode.Serialize()
	require.NoError(t, err)
	_, err = p.memberBucket.Put(p.ctx, p.memberKey(joinID), data)
	require.NoError(t, err)

	// Second pass: the key is now present, so the member is not a prune
	// candidate and must be retained -- no false prune of the concurrent joiner.
	require.NoError(t, p.reconcileMembers())
	p.membersMu.RLock()
	_, afterSecond := p.members[joinID]
	p.membersMu.RUnlock()
	assert.True(t, afterSecond,
		"member whose key appears by the second pass must never be pruned")
}

// TestReconcile_TickerPrunesStaleMemberEndToEnd verifies that the reconcile
// goroutine started by StartMember prunes a stale member end-to-end when driven
// by its ticker, using a short ReconcileInterval. This confirms the ticker path
// integrates without deadlock and that repeated passes clear the grace window.
func TestReconcile_TickerPrunesStaleMemberEndToEnd(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-reconcile-ticker",
		WithReconcileInterval(200*time.Millisecond))

	require.NoError(t, p.StartMember(c))
	t.Cleanup(func() { _ = p.Shutdown(true) })

	staleID := "test-reconcile-ticker_stale-ghost"
	p.membersMu.Lock()
	p.members[staleID] = NewNode(staleID, "10.0.0.99", 6666, nil)
	p.membersMu.Unlock()

	require.Eventually(t, func() bool {
		p.membersMu.RLock()
		_, present := p.members[staleID]
		p.membersMu.RUnlock()
		return !present
	}, 5*time.Second, 100*time.Millisecond,
		"reconcile ticker should prune the stale member end-to-end")
}
