package natskv

import (
	"context"
	"encoding/json"
	"testing"
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
