//go:build integration

package protopg_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/asynkron/protoactor-go/persistence/protopg"
)

// getTestConnectionString returns a PostgreSQL connection string for
// integration tests. It reads from the PROTOPG_TEST_DSN environment variable
// or falls back to a sensible default for local Docker-based testing.
func getTestConnectionString(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("PROTOPG_TEST_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/protoactor_test?sslmode=disable"
	}
	return dsn
}

// newTestProvider creates a fresh provider with unique table names so tests
// don't interfere with each other.
func newTestProvider(t *testing.T, snapshotInterval int) *protopg.PostgresProvider {
	t.Helper()
	suffix := fmt.Sprintf("_%d_%d", time.Now().UnixNano(), os.Getpid())
	eventsTable := "events" + suffix
	snapshotsTable := "snapshots" + suffix

	p, err := protopg.New(
		protopg.WithConnectionString(getTestConnectionString(t)),
		protopg.WithSnapshotInterval(snapshotInterval),
		protopg.WithEventsTable(eventsTable),
		protopg.WithSnapshotsTable(snapshotsTable),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = p.CreateSchemaIfNotExists(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		p.Close()
	})

	return p
}

func TestGetSnapshotInterval(t *testing.T) {
	for _, interval := range []int{0, 1, 5, 100} {
		t.Run(fmt.Sprintf("interval-%d", interval), func(t *testing.T) {
			p := newTestProvider(t, interval)
			assert.Equal(t, interval, p.GetSnapshotInterval())
		})
	}
}

func TestRestart(t *testing.T) {
	p := newTestProvider(t, 10)

	// Restart should not panic and is a no-op.
	require.NotPanics(t, func() {
		p.Restart()
	})
}

func TestPersistAndGetEvent(t *testing.T) {
	p := newTestProvider(t, 10)

	msg := wrapperspb.String("hello")
	p.PersistEvent("actor-1", 0, msg)

	var collected []proto.Message
	p.GetEvents("actor-1", 0, 0, func(e interface{}) {
		collected = append(collected, e.(proto.Message))
	})

	require.Len(t, collected, 1)
	got, ok := collected[0].(*wrapperspb.StringValue)
	require.True(t, ok, "expected *wrapperspb.StringValue, got %T", collected[0])
	assert.Equal(t, "hello", got.GetValue())
}

func TestPersistAndGetSnapshot(t *testing.T) {
	p := newTestProvider(t, 10)

	// No snapshot initially.
	_, _, ok := p.GetSnapshot("actor-1")
	assert.False(t, ok)

	snap := wrapperspb.String("state-at-5")
	p.PersistSnapshot("actor-1", 5, snap)

	snapshot, eventIndex, ok := p.GetSnapshot("actor-1")
	require.True(t, ok)
	assert.Equal(t, 5, eventIndex)

	got, ok := snapshot.(*wrapperspb.StringValue)
	require.True(t, ok, "expected *wrapperspb.StringValue, got %T", snapshot)
	assert.Equal(t, "state-at-5", got.GetValue())
}

func TestGetEventsRange(t *testing.T) {
	p := newTestProvider(t, 10)

	// Persist 5 events.
	for i := 0; i < 5; i++ {
		p.PersistEvent("actor-1", i, wrapperspb.String(fmt.Sprintf("e%d", i)))
	}

	t.Run("AllEvents", func(t *testing.T) {
		var collected []string
		p.GetEvents("actor-1", 0, 0, func(e interface{}) {
			collected = append(collected, e.(*wrapperspb.StringValue).GetValue())
		})
		assert.Equal(t, []string{"e0", "e1", "e2", "e3", "e4"}, collected)
	})

	t.Run("SpecificRange", func(t *testing.T) {
		var collected []string
		p.GetEvents("actor-1", 1, 3, func(e interface{}) {
			collected = append(collected, e.(*wrapperspb.StringValue).GetValue())
		})
		assert.Equal(t, []string{"e1", "e2"}, collected)
	})

	t.Run("FromMiddleToEnd", func(t *testing.T) {
		var collected []string
		p.GetEvents("actor-1", 3, 0, func(e interface{}) {
			collected = append(collected, e.(*wrapperspb.StringValue).GetValue())
		})
		assert.Equal(t, []string{"e3", "e4"}, collected)
	})

	t.Run("EmptyRange", func(t *testing.T) {
		var collected []string
		p.GetEvents("actor-1", 2, 2, func(e interface{}) {
			collected = append(collected, e.(*wrapperspb.StringValue).GetValue())
		})
		assert.Empty(t, collected)
	})
}

func TestDeleteEvents(t *testing.T) {
	p := newTestProvider(t, 10)

	for i := 0; i < 5; i++ {
		p.PersistEvent("actor-1", i, wrapperspb.String(fmt.Sprintf("e%d", i)))
	}

	p.DeleteEvents("actor-1", 2)

	var collected []string
	p.GetEvents("actor-1", 0, 0, func(e interface{}) {
		collected = append(collected, e.(*wrapperspb.StringValue).GetValue())
	})
	assert.Equal(t, []string{"e3", "e4"}, collected)
}

func TestDeleteSnapshots(t *testing.T) {
	p := newTestProvider(t, 10)

	p.PersistSnapshot("actor-1", 3, wrapperspb.String("snap-3"))

	p.DeleteSnapshots("actor-1", 3)

	_, _, ok := p.GetSnapshot("actor-1")
	assert.False(t, ok, "snapshot should have been deleted")
}

func TestMultipleActors(t *testing.T) {
	p := newTestProvider(t, 10)

	p.PersistEvent("alice", 0, wrapperspb.String("alice-e0"))
	p.PersistEvent("alice", 1, wrapperspb.String("alice-e1"))
	p.PersistEvent("bob", 0, wrapperspb.String("bob-e0"))

	p.PersistSnapshot("alice", 1, wrapperspb.String("alice-snap"))
	p.PersistSnapshot("bob", 0, wrapperspb.String("bob-snap"))

	// Verify alice's events.
	var aliceEvents []string
	p.GetEvents("alice", 0, 0, func(e interface{}) {
		aliceEvents = append(aliceEvents, e.(*wrapperspb.StringValue).GetValue())
	})
	assert.Equal(t, []string{"alice-e0", "alice-e1"}, aliceEvents)

	// Verify bob's events.
	var bobEvents []string
	p.GetEvents("bob", 0, 0, func(e interface{}) {
		bobEvents = append(bobEvents, e.(*wrapperspb.StringValue).GetValue())
	})
	assert.Equal(t, []string{"bob-e0"}, bobEvents)

	// Verify alice's snapshot.
	snap, idx, ok := p.GetSnapshot("alice")
	require.True(t, ok)
	assert.Equal(t, 1, idx)
	assert.Equal(t, "alice-snap", snap.(*wrapperspb.StringValue).GetValue())

	// Verify bob's snapshot.
	snap, idx, ok = p.GetSnapshot("bob")
	require.True(t, ok)
	assert.Equal(t, 0, idx)
	assert.Equal(t, "bob-snap", snap.(*wrapperspb.StringValue).GetValue())

	// Verify a third actor has nothing.
	var charlieEvents []string
	p.GetEvents("charlie", 0, 0, func(e interface{}) {
		charlieEvents = append(charlieEvents, e.(*wrapperspb.StringValue).GetValue())
	})
	assert.Empty(t, charlieEvents)

	_, _, ok = p.GetSnapshot("charlie")
	assert.False(t, ok)
}

func TestSnapshotOverwrite(t *testing.T) {
	p := newTestProvider(t, 10)

	// Persist a snapshot, then overwrite it at the same index.
	p.PersistSnapshot("actor-1", 5, wrapperspb.String("first"))
	p.PersistSnapshot("actor-1", 5, wrapperspb.String("second"))

	snap, idx, ok := p.GetSnapshot("actor-1")
	require.True(t, ok)
	assert.Equal(t, 5, idx)
	assert.Equal(t, "second", snap.(*wrapperspb.StringValue).GetValue())
}

func TestGetSnapshotReturnsLatest(t *testing.T) {
	p := newTestProvider(t, 10)

	p.PersistSnapshot("actor-1", 2, wrapperspb.String("snap-2"))
	p.PersistSnapshot("actor-1", 5, wrapperspb.String("snap-5"))
	p.PersistSnapshot("actor-1", 3, wrapperspb.String("snap-3"))

	// Should return the highest snapshot_index.
	snap, idx, ok := p.GetSnapshot("actor-1")
	require.True(t, ok)
	assert.Equal(t, 5, idx)
	assert.Equal(t, "snap-5", snap.(*wrapperspb.StringValue).GetValue())
}

func TestDataSurvivesRestart(t *testing.T) {
	p := newTestProvider(t, 10)

	p.PersistEvent("actor-1", 0, wrapperspb.String("e0"))
	p.PersistEvent("actor-1", 1, wrapperspb.String("e1"))
	p.PersistSnapshot("actor-1", 1, wrapperspb.String("snap-1"))

	p.Restart()

	var events []string
	p.GetEvents("actor-1", 0, 0, func(e interface{}) {
		events = append(events, e.(*wrapperspb.StringValue).GetValue())
	})
	assert.Equal(t, []string{"e0", "e1"}, events)

	snap, idx, ok := p.GetSnapshot("actor-1")
	require.True(t, ok)
	assert.Equal(t, 1, idx)
	assert.Equal(t, "snap-1", snap.(*wrapperspb.StringValue).GetValue())
}
