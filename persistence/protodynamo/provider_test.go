//go:build integration

package protodynamo_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/awevoke/protoactor-go/persistence/protodynamo"
)

// startDynamoDBLocal launches a DynamoDB-local container via testcontainers
// and returns a configured *dynamodb.Client pointing at it.
func startDynamoDBLocal(t *testing.T) *dynamodb.Client {
	t.Helper()
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "amazon/dynamodb-local:latest",
			ExposedPorts: []string{"8000/tcp"},
			WaitingFor:   wait.ForListeningPort("8000/tcp"),
		},
		Started: true,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = container.Terminate(ctx)
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "8000")
	require.NoError(t, err)

	endpoint := fmt.Sprintf("http://%s:%s", host, port.Port())

	client := dynamodb.New(dynamodb.Options{
		Region:       "us-east-1",
		BaseEndpoint: aws.String(endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider("fakeKey", "fakeSecret", "fakeToken"),
	})

	return client
}

func newProvider(t *testing.T, client *dynamodb.Client, snapshotInterval int, suffix string) *protodynamo.DynamoDBProvider {
	t.Helper()

	provider, err := protodynamo.New(
		protodynamo.WithClient(client),
		protodynamo.WithEventsTable("events_"+suffix),
		protodynamo.WithSnapshotsTable("snapshots_"+suffix),
		protodynamo.WithSnapshotInterval(snapshotInterval),
	)
	require.NoError(t, err)

	err = provider.CreateTablesIfNotExist(context.Background())
	require.NoError(t, err)

	return provider
}

func TestPersistAndGetEvent(t *testing.T) {
	client := startDynamoDBLocal(t)
	p := newProvider(t, client, 10, "events1")

	msg := wrapperspb.String("hello")
	p.PersistEvent("actor-1", 0, msg)

	var collected []any
	p.GetEvents("actor-1", 0, 0, func(e any) {
		collected = append(collected, e)
	})

	require.Len(t, collected, 1)
	got, ok := collected[0].(proto.Message)
	require.True(t, ok)
	assert.Equal(t, "hello", got.(*wrapperspb.StringValue).GetValue())
}

func TestPersistAndGetSnapshot(t *testing.T) {
	client := startDynamoDBLocal(t)
	p := newProvider(t, client, 10, "snap1")

	// No snapshot initially.
	_, _, ok := p.GetSnapshot("actor-1")
	assert.False(t, ok)

	snap := wrapperspb.String("state-at-5")
	p.PersistSnapshot("actor-1", 5, snap)

	snapshot, eventIndex, ok := p.GetSnapshot("actor-1")
	require.True(t, ok)
	assert.Equal(t, 5, eventIndex)

	got, ok := snapshot.(*wrapperspb.StringValue)
	require.True(t, ok)
	assert.Equal(t, "state-at-5", got.GetValue())
}

func TestGetEventsRange(t *testing.T) {
	client := startDynamoDBLocal(t)
	p := newProvider(t, client, 10, "range1")

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
	client := startDynamoDBLocal(t)
	p := newProvider(t, client, 10, "del_events1")

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
	client := startDynamoDBLocal(t)
	p := newProvider(t, client, 10, "del_snap1")

	p.PersistSnapshot("actor-1", 3, wrapperspb.String("snap-3"))

	p.DeleteSnapshots("actor-1", 3)

	_, _, ok := p.GetSnapshot("actor-1")
	assert.False(t, ok)
}

func TestMultipleActors(t *testing.T) {
	client := startDynamoDBLocal(t)
	p := newProvider(t, client, 10, "multi1")

	p.PersistEvent("alice", 0, wrapperspb.String("alice-e0"))
	p.PersistEvent("alice", 1, wrapperspb.String("alice-e1"))
	p.PersistEvent("bob", 0, wrapperspb.String("bob-e0"))

	p.PersistSnapshot("alice", 1, wrapperspb.String("alice-snap"))
	p.PersistSnapshot("bob", 0, wrapperspb.String("bob-snap"))

	var aliceEvents []string
	p.GetEvents("alice", 0, 0, func(e any) {
		aliceEvents = append(aliceEvents, e.(*wrapperspb.StringValue).GetValue())
	})
	assert.Equal(t, []string{"alice-e0", "alice-e1"}, aliceEvents)

	var bobEvents []string
	p.GetEvents("bob", 0, 0, func(e any) {
		bobEvents = append(bobEvents, e.(*wrapperspb.StringValue).GetValue())
	})
	assert.Equal(t, []string{"bob-e0"}, bobEvents)

	snap, idx, ok := p.GetSnapshot("alice")
	require.True(t, ok)
	assert.Equal(t, 1, idx)
	assert.Equal(t, "alice-snap", snap.(*wrapperspb.StringValue).GetValue())

	snap, idx, ok = p.GetSnapshot("bob")
	require.True(t, ok)
	assert.Equal(t, 0, idx)
	assert.Equal(t, "bob-snap", snap.(*wrapperspb.StringValue).GetValue())

	var charlieEvents []string
	p.GetEvents("charlie", 0, 0, func(e any) {
		charlieEvents = append(charlieEvents, e.(*wrapperspb.StringValue).GetValue())
	})
	assert.Empty(t, charlieEvents)

	_, _, ok = p.GetSnapshot("charlie")
	assert.False(t, ok)
}

func TestSnapshotInterval(t *testing.T) {
	client := startDynamoDBLocal(t)

	for _, interval := range []int{0, 1, 5, 100} {
		t.Run(fmt.Sprintf("interval-%d", interval), func(t *testing.T) {
			p := newProvider(t, client, interval, fmt.Sprintf("interval_%d", interval))
			assert.Equal(t, interval, p.GetSnapshotInterval())
		})
	}
}

func TestRestart(t *testing.T) {
	client := startDynamoDBLocal(t)
	p := newProvider(t, client, 10, "restart1")

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

func TestCreateTablesIdempotent(t *testing.T) {
	client := startDynamoDBLocal(t)
	p := newProvider(t, client, 1, "idempotent1")

	// Calling CreateTablesIfNotExist a second time should not error.
	err := p.CreateTablesIfNotExist(context.Background())
	require.NoError(t, err)
}
