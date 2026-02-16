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
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/asynkron/protoactor-go/persistence/protopg"
)

var testDSN string

func TestMain(m *testing.M) {
	if dsn := os.Getenv("PROTOPG_TEST_DSN"); dsn != "" {
		testDSN = dsn
		os.Exit(m.Run())
	}

	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:16-alpine",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_USER":     "postgres",
				"POSTGRES_PASSWORD": "postgres",
				"POSTGRES_DB":       "protoactor_test",
			},
			WaitingFor: wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start postgres container: %v\n", err)
		os.Exit(1)
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		fmt.Fprintf(os.Stderr, "failed to get container host: %v\n", err)
		os.Exit(1)
	}

	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		_ = container.Terminate(ctx)
		fmt.Fprintf(os.Stderr, "failed to get container port: %v\n", err)
		os.Exit(1)
	}

	testDSN = fmt.Sprintf("postgres://postgres:postgres@%s:%s/protoactor_test?sslmode=disable", host, port.Port())

	code := m.Run()

	_ = container.Terminate(ctx)
	os.Exit(code)
}

// newTestProvider creates a fresh provider with unique table names so tests
// don't interfere with each other.
func newTestProvider(t *testing.T, snapshotInterval int) *protopg.PostgresProvider {
	t.Helper()
	suffix := fmt.Sprintf("_%d_%d", time.Now().UnixNano(), os.Getpid())
	eventsTable := "events" + suffix
	snapshotsTable := "snapshots" + suffix

	p, err := protopg.New(
		protopg.WithConnectionString(testDSN),
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
	tests := []struct {
		input    int
		expected int
	}{
		{0, 1},   // 0 is replaced by the default of 1
		{1, 1},
		{5, 5},
		{100, 100},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("interval-%d", tc.input), func(t *testing.T) {
			p := newTestProvider(t, tc.input)
			assert.Equal(t, tc.expected, p.GetSnapshotInterval())
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
	p.GetEvents("actor-1", 0, 0, func(e any) {
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
		p.GetEvents("actor-1", 0, 0, func(e any) {
			collected = append(collected, e.(*wrapperspb.StringValue).GetValue())
		})
		assert.Equal(t, []string{"e0", "e1", "e2", "e3", "e4"}, collected)
	})

	t.Run("SpecificRange", func(t *testing.T) {
		var collected []string
		p.GetEvents("actor-1", 1, 3, func(e any) {
			collected = append(collected, e.(*wrapperspb.StringValue).GetValue())
		})
		assert.Equal(t, []string{"e1", "e2"}, collected)
	})

	t.Run("FromMiddleToEnd", func(t *testing.T) {
		var collected []string
		p.GetEvents("actor-1", 3, 0, func(e any) {
			collected = append(collected, e.(*wrapperspb.StringValue).GetValue())
		})
		assert.Equal(t, []string{"e3", "e4"}, collected)
	})

	t.Run("EmptyRange", func(t *testing.T) {
		var collected []string
		p.GetEvents("actor-1", 2, 2, func(e any) {
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
	p.GetEvents("actor-1", 0, 0, func(e any) {
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
	p.GetEvents("alice", 0, 0, func(e any) {
		aliceEvents = append(aliceEvents, e.(*wrapperspb.StringValue).GetValue())
	})
	assert.Equal(t, []string{"alice-e0", "alice-e1"}, aliceEvents)

	// Verify bob's events.
	var bobEvents []string
	p.GetEvents("bob", 0, 0, func(e any) {
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
	p.GetEvents("charlie", 0, 0, func(e any) {
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
	p.GetEvents("actor-1", 0, 0, func(e any) {
		events = append(events, e.(*wrapperspb.StringValue).GetValue())
	})
	assert.Equal(t, []string{"e0", "e1"}, events)

	snap, idx, ok := p.GetSnapshot("actor-1")
	require.True(t, ok)
	assert.Equal(t, 1, idx)
	assert.Equal(t, "snap-1", snap.(*wrapperspb.StringValue).GetValue())
}
