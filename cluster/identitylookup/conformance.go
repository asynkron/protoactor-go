// Package identitylookup provides a conformance test suite for StorageLookup
// implementations, plus common utilities for identity lookup strategies.
package identitylookup

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// StorageConformanceSuite validates that a StorageLookup implementation
// conforms to the expected behavior. Backend implementations (e.g., Redis,
// MongoDB, in-memory) should run this suite in their own test files:
//
//	func TestConformance(t *testing.T) {
//	    suite := &identitylookup.StorageConformanceSuite{
//	        NewStorage: func() cluster.StorageLookup { return myimpl.New() },
//	        Cleanup:    func() { /* optional teardown */ },
//	    }
//	    suite.RunAll(t)
//	}
type StorageConformanceSuite struct {
	// NewStorage creates a fresh StorageLookup instance for each test.
	NewStorage func() cluster.StorageLookup

	// Cleanup is called after each test (may be nil).
	Cleanup func()
}

// RunAll runs every conformance test as a sub-test of t.
func (s *StorageConformanceSuite) RunAll(t *testing.T) {
	t.Helper()

	tests := []struct {
		name string
		fn   func(t *testing.T, storage cluster.StorageLookup)
	}{
		{"TryAcquireLock", s.testTryAcquireLock},
		{"DoubleAcquireFails", s.testDoubleAcquireFails},
		{"StoreAndGetActivation", s.testStoreAndGetActivation},
		{"RemoveActivation", s.testRemoveActivation},
		{"RemoveMemberId", s.testRemoveMemberId},
		{"WaitForActivation", s.testWaitForActivation},
		{"RemoveLock", s.testRemoveLock},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			storage := s.NewStorage()
			defer func() {
				if s.Cleanup != nil {
					s.Cleanup()
				}
			}()
			tc.fn(t, storage)
		})
	}
}

// --- individual test methods ---

func (s *StorageConformanceSuite) testTryAcquireLock(t *testing.T, storage cluster.StorageLookup) {
	ci := &cluster.ClusterIdentity{Identity: "actor-1", Kind: "kind-1"}

	lock := storage.TryAcquireLock(ci)
	require.NotNil(t, lock, "first TryAcquireLock should succeed")
	require.NotEmpty(t, lock.LockID, "LockID should be non-empty")
	require.Equal(t, ci.Identity, lock.ClusterIdentity.Identity)
	require.Equal(t, ci.Kind, lock.ClusterIdentity.Kind)
}

func (s *StorageConformanceSuite) testDoubleAcquireFails(t *testing.T, storage cluster.StorageLookup) {
	ci := &cluster.ClusterIdentity{Identity: "actor-2", Kind: "kind-1"}

	lock1 := storage.TryAcquireLock(ci)
	require.NotNil(t, lock1, "first TryAcquireLock should succeed")

	lock2 := storage.TryAcquireLock(ci)
	require.Nil(t, lock2, "second TryAcquireLock for same identity should return nil")
}

func (s *StorageConformanceSuite) testStoreAndGetActivation(t *testing.T, storage cluster.StorageLookup) {
	ci := &cluster.ClusterIdentity{Identity: "actor-3", Kind: "kind-1"}

	// Before any activation exists, lookup should return nil.
	existing := storage.TryGetExistingActivation(ci)
	require.Nil(t, existing, "no activation should exist yet")

	// Acquire lock, store activation.
	lock := storage.TryAcquireLock(ci)
	require.NotNil(t, lock)

	pid := actor.NewPID("127.0.0.1:8080", "actor-3")
	storage.StoreActivation("member-1", lock, pid)

	// Now we should find the stored activation.
	stored := storage.TryGetExistingActivation(ci)
	require.NotNil(t, stored, "activation should exist after StoreActivation")
	require.Equal(t, "member-1", stored.MemberID)
	require.NotEmpty(t, stored.Pid, "Pid field should be populated")
}

func (s *StorageConformanceSuite) testRemoveActivation(t *testing.T, storage cluster.StorageLookup) {
	ci := &cluster.ClusterIdentity{Identity: "actor-4", Kind: "kind-1"}

	lock := storage.TryAcquireLock(ci)
	require.NotNil(t, lock)

	pid := actor.NewPID("127.0.0.1:8080", "actor-4")
	storage.StoreActivation("member-1", lock, pid)

	// Verify it exists.
	stored := storage.TryGetExistingActivation(ci)
	require.NotNil(t, stored)

	// Remove using a SpawnLock that references the same identity.
	storage.RemoveActivation(&cluster.SpawnLock{
		LockID:          lock.LockID,
		ClusterIdentity: ci,
	})

	// Verify it's gone.
	gone := storage.TryGetExistingActivation(ci)
	require.Nil(t, gone, "activation should be removed after RemoveActivation")
}

func (s *StorageConformanceSuite) testRemoveMemberId(t *testing.T, storage cluster.StorageLookup) {
	// Store two activations for the same member.
	ci1 := &cluster.ClusterIdentity{Identity: "actor-5a", Kind: "kind-1"}
	ci2 := &cluster.ClusterIdentity{Identity: "actor-5b", Kind: "kind-1"}

	lock1 := storage.TryAcquireLock(ci1)
	require.NotNil(t, lock1)
	storage.StoreActivation("member-2", lock1, actor.NewPID("127.0.0.1:8080", "actor-5a"))

	lock2 := storage.TryAcquireLock(ci2)
	require.NotNil(t, lock2)
	storage.StoreActivation("member-2", lock2, actor.NewPID("127.0.0.1:8080", "actor-5b"))

	// Store one activation for a different member.
	ci3 := &cluster.ClusterIdentity{Identity: "actor-5c", Kind: "kind-1"}
	lock3 := storage.TryAcquireLock(ci3)
	require.NotNil(t, lock3)
	storage.StoreActivation("member-3", lock3, actor.NewPID("127.0.0.1:8081", "actor-5c"))

	// Remove all activations for member-2.
	storage.RemoveMemberId("member-2")

	// member-2 activations should be gone.
	require.Nil(t, storage.TryGetExistingActivation(ci1), "member-2 activation ci1 should be removed")
	require.Nil(t, storage.TryGetExistingActivation(ci2), "member-2 activation ci2 should be removed")

	// member-3 activation should remain.
	remaining := storage.TryGetExistingActivation(ci3)
	require.NotNil(t, remaining, "member-3 activation should remain")
	require.Equal(t, "member-3", remaining.MemberID)
}

func (s *StorageConformanceSuite) testWaitForActivation(t *testing.T, storage cluster.StorageLookup) {
	ci := &cluster.ClusterIdentity{Identity: "actor-6", Kind: "kind-1"}

	var wg sync.WaitGroup
	wg.Add(1)

	var result *cluster.StoredActivation

	// Start a goroutine that waits for the activation.
	go func() {
		defer wg.Done()
		result = storage.WaitForActivation(ci)
	}()

	// Give the goroutine a moment to start waiting.
	time.Sleep(50 * time.Millisecond)

	// Acquire lock and store activation from the main goroutine.
	lock := storage.TryAcquireLock(ci)
	require.NotNil(t, lock)
	storage.StoreActivation("member-4", lock, actor.NewPID("127.0.0.1:8080", "actor-6"))

	// Wait for the result with a timeout.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(5 * time.Second):
		t.Fatal("WaitForActivation did not return within 5 seconds")
	}

	require.NotNil(t, result, "WaitForActivation should return the stored activation")
	require.Equal(t, "member-4", result.MemberID)
}

func (s *StorageConformanceSuite) testRemoveLock(t *testing.T, storage cluster.StorageLookup) {
	ci := &cluster.ClusterIdentity{Identity: "actor-7", Kind: "kind-1"}

	// Acquire lock.
	lock := storage.TryAcquireLock(ci)
	require.NotNil(t, lock, "first TryAcquireLock should succeed")

	// Verify a second acquire fails.
	lock2 := storage.TryAcquireLock(ci)
	require.Nil(t, lock2, "second acquire should fail while lock is held")

	// Remove the lock.
	storage.RemoveLock(cluster.SpawnLock{
		LockID:          lock.LockID,
		ClusterIdentity: ci,
	})

	// Now acquiring the lock again should succeed.
	lock3 := storage.TryAcquireLock(ci)
	require.NotNil(t, lock3, "TryAcquireLock should succeed after RemoveLock")
}

// identityKey returns a deterministic map key for a ClusterIdentity.
func identityKey(ci *cluster.ClusterIdentity) string {
	return fmt.Sprintf("%s/%s", ci.Kind, ci.Identity)
}

// EnumerableStorage combines StorageLookup and StorageGrainEnumerator for
// conformance testing of enumerator implementations.
type EnumerableStorage interface {
	cluster.StorageLookup
	cluster.StorageGrainEnumerator
}

// EnumeratorConformanceSuite validates that a StorageGrainEnumerator
// implementation conforms to the expected behavior.
type EnumeratorConformanceSuite struct {
	// NewStorage creates a fresh EnumerableStorage instance for each test.
	NewStorage func() EnumerableStorage

	// Cleanup is called after each test (may be nil).
	Cleanup func()
}

// RunAll runs every enumerator conformance test as a sub-test of t.
func (s *EnumeratorConformanceSuite) RunAll(t *testing.T) {
	t.Helper()

	tests := []struct {
		name string
		fn   func(t *testing.T, storage EnumerableStorage)
	}{
		{"ListActivations", s.testListActivations},
		{"ListActivationsByMember", s.testListActivationsByMember},
		{"ExcludesLocked", s.testExcludesLocked},
		{"ReflectsRemovals", s.testReflectsRemovals},
		{"AfterRemoveMember", s.testAfterRemoveMember},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			storage := s.NewStorage()
			defer func() {
				if s.Cleanup != nil {
					s.Cleanup()
				}
			}()
			tc.fn(t, storage)
		})
	}
}

func (s *EnumeratorConformanceSuite) testListActivations(t *testing.T, storage EnumerableStorage) {
	ci1 := &cluster.ClusterIdentity{Identity: "enum-1a", Kind: "kind-1"}
	ci2 := &cluster.ClusterIdentity{Identity: "enum-1b", Kind: "kind-2"}

	lock1 := storage.TryAcquireLock(ci1)
	require.NotNil(t, lock1)
	storage.StoreActivation("member-1", lock1, actor.NewPID("127.0.0.1:8080", "enum-1a"))

	lock2 := storage.TryAcquireLock(ci2)
	require.NotNil(t, lock2)
	storage.StoreActivation("member-2", lock2, actor.NewPID("127.0.0.1:8081", "enum-1b"))

	activations, err := storage.ListActivations()
	require.NoError(t, err)
	assert.Len(t, activations, 2)

	// Build a map for order-independent assertions.
	byIdentity := make(map[string]*cluster.StoredActivationInfo)
	for _, a := range activations {
		byIdentity[a.Identity] = a
	}

	a1 := byIdentity["enum-1a"]
	require.NotNil(t, a1)
	assert.Equal(t, "kind-1", a1.Kind)
	assert.Equal(t, "member-1", a1.MemberID)
	assert.NotEmpty(t, a1.Pid)

	a2 := byIdentity["enum-1b"]
	require.NotNil(t, a2)
	assert.Equal(t, "kind-2", a2.Kind)
	assert.Equal(t, "member-2", a2.MemberID)
	assert.NotEmpty(t, a2.Pid)
}

func (s *EnumeratorConformanceSuite) testListActivationsByMember(t *testing.T, storage EnumerableStorage) {
	ci1 := &cluster.ClusterIdentity{Identity: "enum-2a", Kind: "kind-1"}
	ci2 := &cluster.ClusterIdentity{Identity: "enum-2b", Kind: "kind-1"}
	ci3 := &cluster.ClusterIdentity{Identity: "enum-2c", Kind: "kind-1"}

	lock1 := storage.TryAcquireLock(ci1)
	require.NotNil(t, lock1)
	storage.StoreActivation("member-A", lock1, actor.NewPID("127.0.0.1:8080", "enum-2a"))

	lock2 := storage.TryAcquireLock(ci2)
	require.NotNil(t, lock2)
	storage.StoreActivation("member-A", lock2, actor.NewPID("127.0.0.1:8080", "enum-2b"))

	lock3 := storage.TryAcquireLock(ci3)
	require.NotNil(t, lock3)
	storage.StoreActivation("member-B", lock3, actor.NewPID("127.0.0.1:8081", "enum-2c"))

	// Filter by member-A.
	memberA, err := storage.ListActivationsByMember("member-A")
	require.NoError(t, err)
	assert.Len(t, memberA, 2)
	for _, a := range memberA {
		assert.Equal(t, "member-A", a.MemberID)
	}

	// Filter by member-B.
	memberB, err := storage.ListActivationsByMember("member-B")
	require.NoError(t, err)
	assert.Len(t, memberB, 1)
	assert.Equal(t, "member-B", memberB[0].MemberID)
	assert.Equal(t, "enum-2c", memberB[0].Identity)

	// Non-existent member returns empty.
	none, err := storage.ListActivationsByMember("member-Z")
	require.NoError(t, err)
	assert.Empty(t, none)
}

func (s *EnumeratorConformanceSuite) testExcludesLocked(t *testing.T, storage EnumerableStorage) {
	ci := &cluster.ClusterIdentity{Identity: "enum-3a", Kind: "kind-1"}

	// Acquire lock but do NOT store an activation.
	lock := storage.TryAcquireLock(ci)
	require.NotNil(t, lock)

	activations, err := storage.ListActivations()
	require.NoError(t, err)
	assert.Empty(t, activations, "locked-only entries should not appear in ListActivations")

	byMember, err := storage.ListActivationsByMember("any-member")
	require.NoError(t, err)
	assert.Empty(t, byMember, "locked-only entries should not appear in ListActivationsByMember")
}

func (s *EnumeratorConformanceSuite) testReflectsRemovals(t *testing.T, storage EnumerableStorage) {
	ci := &cluster.ClusterIdentity{Identity: "enum-4a", Kind: "kind-1"}

	lock := storage.TryAcquireLock(ci)
	require.NotNil(t, lock)
	storage.StoreActivation("member-1", lock, actor.NewPID("127.0.0.1:8080", "enum-4a"))

	// Verify it appears.
	before, err := storage.ListActivations()
	require.NoError(t, err)
	assert.Len(t, before, 1)

	// Remove it.
	storage.RemoveActivation(&cluster.SpawnLock{
		LockID:          lock.LockID,
		ClusterIdentity: ci,
	})

	// Verify it's gone.
	after, err := storage.ListActivations()
	require.NoError(t, err)
	assert.Empty(t, after, "removed activation should not appear in ListActivations")
}

func (s *EnumeratorConformanceSuite) testAfterRemoveMember(t *testing.T, storage EnumerableStorage) {
	ci1 := &cluster.ClusterIdentity{Identity: "enum-5a", Kind: "kind-1"}
	ci2 := &cluster.ClusterIdentity{Identity: "enum-5b", Kind: "kind-1"}

	lock1 := storage.TryAcquireLock(ci1)
	require.NotNil(t, lock1)
	storage.StoreActivation("member-X", lock1, actor.NewPID("127.0.0.1:8080", "enum-5a"))

	lock2 := storage.TryAcquireLock(ci2)
	require.NotNil(t, lock2)
	storage.StoreActivation("member-Y", lock2, actor.NewPID("127.0.0.1:8081", "enum-5b"))

	// Remove all activations for member-X.
	storage.RemoveMemberId("member-X")

	// ListActivationsByMember should return empty for member-X.
	memberX, err := storage.ListActivationsByMember("member-X")
	require.NoError(t, err)
	assert.Empty(t, memberX, "member-X activations should be gone after RemoveMemberId")

	// member-Y should still be present.
	memberY, err := storage.ListActivationsByMember("member-Y")
	require.NoError(t, err)
	assert.Len(t, memberY, 1)
	assert.Equal(t, "enum-5b", memberY[0].Identity)

	// ListActivations should only have member-Y's activation.
	all, err := storage.ListActivations()
	require.NoError(t, err)
	assert.Len(t, all, 1)
}
