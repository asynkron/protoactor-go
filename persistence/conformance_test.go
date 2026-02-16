package persistence

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ConformanceSuite validates any ProviderState implementation against the
// persistence contract. Create an instance, set NewProvider to a factory
// that returns a fresh provider for each sub-test, and call RunAll.
type ConformanceSuite struct {
	// NewProvider returns a fresh ProviderState for the given snapshotInterval.
	NewProvider func(snapshotInterval int) ProviderState

	// SupportsDelete indicates whether the provider actually removes data on
	// DeleteEvents / DeleteSnapshots. When false the delete tests still verify
	// no panic occurs, but skip assertions about data removal.
	SupportsDelete bool
}

// RunAll executes every conformance test as a subtest of t.
func (s *ConformanceSuite) RunAll(t *testing.T) {
	t.Helper()
	t.Run("PersistAndGetEvent", s.TestPersistAndGetEvent)
	t.Run("PersistAndGetSnapshot", s.TestPersistAndGetSnapshot)
	t.Run("GetEventsRange", s.TestGetEventsRange)
	t.Run("DeleteEvents", s.TestDeleteEvents)
	t.Run("DeleteSnapshots", s.TestDeleteSnapshots)
	t.Run("MultipleActors", s.TestMultipleActors)
	t.Run("SnapshotInterval", s.TestSnapshotInterval)
	t.Run("Restart", s.TestRestart)
}

// TestPersistAndGetEvent persists a single event and retrieves it.
func (s *ConformanceSuite) TestPersistAndGetEvent(t *testing.T) {
	provider := s.NewProvider(10)

	msg := newMessage("hello")
	provider.PersistEvent("actor-1", 0, msg)

	var collected []any
	provider.GetEvents("actor-1", 0, 0, func(e any) {
		collected = append(collected, e)
	})

	require.Len(t, collected, 1, "expected exactly one event")
	got, ok := collected[0].(*Message)
	require.True(t, ok, "expected event to be *Message, got %T", collected[0])
	assert.Equal(t, "hello", got.state)
}

// TestPersistAndGetSnapshot persists a snapshot and retrieves it.
func (s *ConformanceSuite) TestPersistAndGetSnapshot(t *testing.T) {
	provider := s.NewProvider(10)

	// No snapshot initially.
	_, _, ok := provider.GetSnapshot("actor-1")
	assert.False(t, ok, "expected no snapshot before any persist")

	snap := newSnapshot("state-at-5")
	provider.PersistSnapshot("actor-1", 5, snap)

	snapshot, eventIndex, ok := provider.GetSnapshot("actor-1")
	require.True(t, ok, "expected snapshot to exist")
	assert.Equal(t, 5, eventIndex)

	got, ok := snapshot.(*Snapshot)
	require.True(t, ok, "expected snapshot to be *Snapshot, got %T", snapshot)
	assert.Equal(t, "state-at-5", got.state)
}

// TestGetEventsRange persists multiple events and verifies range queries.
func (s *ConformanceSuite) TestGetEventsRange(t *testing.T) {
	provider := s.NewProvider(10)

	// Persist 5 events: e0..e4
	for i := 0; i < 5; i++ {
		provider.PersistEvent("actor-1", i, newMessage(fmt.Sprintf("e%d", i)))
	}

	// Sub-test: get all events (eventIndexEnd=0 means "all").
	t.Run("AllEvents", func(t *testing.T) {
		var collected []string
		provider.GetEvents("actor-1", 0, 0, func(e any) {
			collected = append(collected, e.(*Message).state)
		})
		assert.Equal(t, []string{"e0", "e1", "e2", "e3", "e4"}, collected)
	})

	// Sub-test: specific range [1, 3).
	t.Run("SpecificRange", func(t *testing.T) {
		var collected []string
		provider.GetEvents("actor-1", 1, 3, func(e any) {
			collected = append(collected, e.(*Message).state)
		})
		assert.Equal(t, []string{"e1", "e2"}, collected)
	})

	// Sub-test: range from middle to end.
	t.Run("FromMiddleToEnd", func(t *testing.T) {
		var collected []string
		provider.GetEvents("actor-1", 3, 0, func(e any) {
			collected = append(collected, e.(*Message).state)
		})
		assert.Equal(t, []string{"e3", "e4"}, collected)
	})

	// Sub-test: empty range (start == end).
	t.Run("EmptyRange", func(t *testing.T) {
		var collected []string
		provider.GetEvents("actor-1", 2, 2, func(e any) {
			collected = append(collected, e.(*Message).state)
		})
		assert.Empty(t, collected)
	})
}

// TestDeleteEvents verifies that DeleteEvents does not panic. If the provider
// supports deletion (SupportsDelete), it also verifies that deleted events
// are no longer returned.
func (s *ConformanceSuite) TestDeleteEvents(t *testing.T) {
	provider := s.NewProvider(10)

	for i := 0; i < 5; i++ {
		provider.PersistEvent("actor-1", i, newMessage(fmt.Sprintf("e%d", i)))
	}

	// DeleteEvents should not panic regardless of implementation.
	require.NotPanics(t, func() {
		provider.DeleteEvents("actor-1", 2) // delete events at indices <= 2
	})

	if !s.SupportsDelete {
		t.Skip("provider does not implement actual event deletion; verified no panic")
		return
	}

	// For providers that support delete, verify events 0-2 are gone.
	var collected []string
	provider.GetEvents("actor-1", 0, 0, func(e any) {
		collected = append(collected, e.(*Message).state)
	})
	assert.Equal(t, []string{"e3", "e4"}, collected,
		"after deleting events inclusive to index 2, only e3 and e4 should remain")
}

// TestDeleteSnapshots verifies that DeleteSnapshots does not panic. If the
// provider supports deletion (SupportsDelete), it also verifies removal.
func (s *ConformanceSuite) TestDeleteSnapshots(t *testing.T) {
	provider := s.NewProvider(10)

	provider.PersistSnapshot("actor-1", 3, newSnapshot("snap-3"))

	// DeleteSnapshots should not panic regardless of implementation.
	require.NotPanics(t, func() {
		provider.DeleteSnapshots("actor-1", 3)
	})

	if !s.SupportsDelete {
		t.Skip("provider does not implement actual snapshot deletion; verified no panic")
		return
	}

	// For providers that support delete, the snapshot should be gone.
	_, _, ok := provider.GetSnapshot("actor-1")
	assert.False(t, ok, "snapshot should have been deleted")
}

// TestMultipleActors verifies that events and snapshots for different actors
// are stored independently.
func (s *ConformanceSuite) TestMultipleActors(t *testing.T) {
	provider := s.NewProvider(10)

	// Persist events for two different actors.
	provider.PersistEvent("alice", 0, newMessage("alice-e0"))
	provider.PersistEvent("alice", 1, newMessage("alice-e1"))
	provider.PersistEvent("bob", 0, newMessage("bob-e0"))

	// Persist snapshots for two different actors.
	provider.PersistSnapshot("alice", 1, newSnapshot("alice-snap"))
	provider.PersistSnapshot("bob", 0, newSnapshot("bob-snap"))

	// Verify alice's events.
	var aliceEvents []string
	provider.GetEvents("alice", 0, 0, func(e any) {
		aliceEvents = append(aliceEvents, e.(*Message).state)
	})
	assert.Equal(t, []string{"alice-e0", "alice-e1"}, aliceEvents)

	// Verify bob's events.
	var bobEvents []string
	provider.GetEvents("bob", 0, 0, func(e any) {
		bobEvents = append(bobEvents, e.(*Message).state)
	})
	assert.Equal(t, []string{"bob-e0"}, bobEvents)

	// Verify alice's snapshot.
	snap, idx, ok := provider.GetSnapshot("alice")
	require.True(t, ok)
	assert.Equal(t, 1, idx)
	assert.Equal(t, "alice-snap", snap.(*Snapshot).state)

	// Verify bob's snapshot.
	snap, idx, ok = provider.GetSnapshot("bob")
	require.True(t, ok)
	assert.Equal(t, 0, idx)
	assert.Equal(t, "bob-snap", snap.(*Snapshot).state)

	// Verify a third actor has nothing.
	var charlieEvents []string
	provider.GetEvents("charlie", 0, 0, func(e any) {
		charlieEvents = append(charlieEvents, e.(*Message).state)
	})
	assert.Empty(t, charlieEvents)

	_, _, ok = provider.GetSnapshot("charlie")
	assert.False(t, ok)
}

// TestSnapshotInterval verifies that GetSnapshotInterval returns the value
// that was configured when the provider was created.
func (s *ConformanceSuite) TestSnapshotInterval(t *testing.T) {
	for _, interval := range []int{0, 1, 5, 100} {
		t.Run(fmt.Sprintf("interval-%d", interval), func(t *testing.T) {
			provider := s.NewProvider(interval)
			assert.Equal(t, interval, provider.GetSnapshotInterval())
		})
	}
}

// TestRestart verifies that data persists across Restart() calls.
func (s *ConformanceSuite) TestRestart(t *testing.T) {
	provider := s.NewProvider(10)

	// Persist some data.
	provider.PersistEvent("actor-1", 0, newMessage("e0"))
	provider.PersistEvent("actor-1", 1, newMessage("e1"))
	provider.PersistSnapshot("actor-1", 1, newSnapshot("snap-1"))

	// Call Restart.
	provider.Restart()

	// Verify events survive restart.
	var events []string
	provider.GetEvents("actor-1", 0, 0, func(e any) {
		events = append(events, e.(*Message).state)
	})
	assert.Equal(t, []string{"e0", "e1"}, events)

	// Verify snapshot survives restart.
	snap, idx, ok := provider.GetSnapshot("actor-1")
	require.True(t, ok)
	assert.Equal(t, 1, idx)
	assert.Equal(t, "snap-1", snap.(*Snapshot).state)
}

// ---------------------------------------------------------------------------
// Run the conformance suite against InMemoryProvider
// ---------------------------------------------------------------------------

func TestInMemoryConformance(t *testing.T) {
	suite := &ConformanceSuite{
		NewProvider: func(snapshotInterval int) ProviderState {
			return NewInMemoryProvider(snapshotInterval)
		},
		SupportsDelete: false, // InMemoryProvider has no-op deletes
	}
	suite.RunAll(t)
}
