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
