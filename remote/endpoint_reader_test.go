package remote

import (
	"context"
	"testing"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeserializeSender_ValidIndex(t *testing.T) {
	pid1 := actor.NewPID("127.0.0.1:8080", "actor1")
	pid2 := actor.NewPID("127.0.0.1:8080", "actor2")
	arr := []*actor.PID{pid1, pid2}

	// index is 1-based; index=1 should return pid1 (arr[0])
	result := deserializeSender(1, 0, arr)
	assert.Equal(t, pid1, result)

	// index=2 should return pid2 (arr[1])
	result = deserializeSender(2, 0, arr)
	assert.Equal(t, pid2, result)
}

func TestDeserializeSender_ZeroIndex(t *testing.T) {
	arr := []*actor.PID{actor.NewPID("127.0.0.1:8080", "actor1")}

	// index=0 means no sender
	result := deserializeSender(0, 0, arr)
	assert.Nil(t, result)
}

func TestDeserializeSender_OutOfBoundsIndex(t *testing.T) {
	arr := []*actor.PID{actor.NewPID("127.0.0.1:8080", "actor1")}

	// index=5 is out of bounds (arr has 1 element, valid 1-based indices are 1)
	result := deserializeSender(5, 0, arr)
	assert.Nil(t, result, "out-of-bounds sender index should return nil, not panic")
}

func TestDeserializeSender_NegativeIndex(t *testing.T) {
	arr := []*actor.PID{actor.NewPID("127.0.0.1:8080", "actor1")}

	// negative index should not panic
	result := deserializeSender(-1, 0, arr)
	assert.Nil(t, result, "negative sender index should return nil, not panic")
}

func TestDeserializeSender_EmptyArray(t *testing.T) {
	var arr []*actor.PID

	// non-zero index with empty array should not panic
	result := deserializeSender(1, 0, arr)
	assert.Nil(t, result, "sender index with empty array should return nil, not panic")
}

func TestDeserializeTarget_ValidIndex(t *testing.T) {
	arr := []string{"actor1", "actor2"}
	address := "127.0.0.1:8080"

	result := deserializeTarget(0, 0, arr, address)
	assert.NotNil(t, result)
	assert.Equal(t, "actor1", result.Id)

	result = deserializeTarget(1, 0, arr, address)
	assert.NotNil(t, result)
	assert.Equal(t, "actor2", result.Id)
}

func TestDeserializeTarget_OutOfBoundsIndex(t *testing.T) {
	arr := []string{"actor1"}
	address := "127.0.0.1:8080"

	// index=5 is out of bounds
	result := deserializeTarget(5, 0, arr, address)
	assert.Nil(t, result, "out-of-bounds target index should return nil, not panic")
}

func TestDeserializeTarget_NegativeIndex(t *testing.T) {
	arr := []string{"actor1"}
	address := "127.0.0.1:8080"

	// negative index should not panic
	result := deserializeTarget(-1, 0, arr, address)
	assert.Nil(t, result, "negative target index should return nil, not panic")
}

func TestDeserializeTarget_EmptyArray(t *testing.T) {
	var arr []string
	address := "127.0.0.1:8080"

	// index=0 with empty array should not panic
	result := deserializeTarget(0, 0, arr, address)
	assert.Nil(t, result, "target index with empty array should return nil, not panic")
}

func TestOnMessageBatch_TypeIdOutOfBounds(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)

	reader := newEndpointReader(r)

	batch := &MessageBatch{
		TypeNames: []string{"some.Type"},
		Targets:   []string{"someActor"},
		Senders:   []*actor.PID{},
		Envelopes: []*MessageEnvelope{
			{
				TypeId: 99, // out of bounds -- TypeNames only has 1 element
				Target: 0,
				Sender: 0,
			},
		},
	}

	err := reader.onMessageBatch(batch)
	assert.Error(t, err, "out-of-bounds TypeId should return error, not panic")
}

func TestOnMessageBatch_SenderOutOfBounds(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)

	reader := newEndpointReader(r)

	batch := &MessageBatch{
		TypeNames: []string{"actor.PID"},
		Targets:   []string{"someActor"},
		Senders:   []*actor.PID{}, // empty senders array
		Envelopes: []*MessageEnvelope{
			{
				TypeId: 0,
				Target: 0,
				Sender: 5, // out of bounds -- Senders is empty
			},
		},
	}

	// This should not panic; sender out of bounds should be handled gracefully
	err := reader.onMessageBatch(batch)
	// The function should either return an error or skip the bad sender.
	// Either way, it must not panic.
	_ = err
}

func TestOnMessageBatch_TargetOutOfBounds(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)

	reader := newEndpointReader(r)

	batch := &MessageBatch{
		TypeNames: []string{"actor.PID"},
		Targets:   []string{"someActor"},
		Senders:   []*actor.PID{},
		Envelopes: []*MessageEnvelope{
			{
				TypeId: 0,
				Target: 99, // out of bounds -- Targets has 1 element
				Sender: 0,
			},
		},
	}

	err := reader.onMessageBatch(batch)
	assert.Error(t, err, "out-of-bounds target index should return error, not panic")
}

func TestEndpointReader_MustEmbedDoesNotPanic(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)
	reader := newEndpointReader(r)

	assert.NotPanics(t, func() {
		reader.mustEmbedUnimplementedRemotingServer()
	})
}

func TestListProcesses_ReturnsSpawnedActors(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)
	reader := newEndpointReader(r)

	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	pid, err := system.Root.SpawnNamed(props, "test-list-actor")
	require.NoError(t, err)
	defer system.Root.Stop(pid)

	resp, err := reader.ListProcesses(context.Background(), &ListProcessesRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)

	found := false
	for _, p := range resp.Pids {
		if p.Id == "test-list-actor" {
			found = true
			break
		}
	}
	assert.True(t, found, "ListProcesses should return the spawned actor")
}

func TestListProcesses_FilterByPattern(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)
	reader := newEndpointReader(r)

	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	pid1, _ := system.Root.SpawnNamed(props, "alpha-actor")
	pid2, _ := system.Root.SpawnNamed(props, "beta-actor")
	defer system.Root.Stop(pid1)
	defer system.Root.Stop(pid2)

	resp, err := reader.ListProcesses(context.Background(), &ListProcessesRequest{
		Pattern: "alpha",
		Type:    ListProcessesMatchType_MatchPartOfString,
	})
	require.NoError(t, err)

	foundAlpha := false
	foundBeta := false
	for _, p := range resp.Pids {
		if p.Id == "alpha-actor" {
			foundAlpha = true
		}
		if p.Id == "beta-actor" {
			foundBeta = true
		}
	}
	assert.True(t, foundAlpha, "should find alpha-actor with partial match")
	assert.False(t, foundBeta, "should not find beta-actor with alpha filter")
}

func TestListProcesses_FilterByExactMatch(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)
	reader := newEndpointReader(r)

	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	pid1, _ := system.Root.SpawnNamed(props, "exact-match-actor")
	defer system.Root.Stop(pid1)

	resp, err := reader.ListProcesses(context.Background(), &ListProcessesRequest{
		Pattern: "exact-match-actor",
		Type:    ListProcessesMatchType_MatchExactString,
	})
	require.NoError(t, err)

	found := false
	for _, p := range resp.Pids {
		if p.Id == "exact-match-actor" {
			found = true
		}
	}
	assert.True(t, found, "exact match should find the actor")

	resp, err = reader.ListProcesses(context.Background(), &ListProcessesRequest{
		Pattern: "exact-match",
		Type:    ListProcessesMatchType_MatchExactString,
	})
	require.NoError(t, err)

	found = false
	for _, p := range resp.Pids {
		if p.Id == "exact-match-actor" {
			found = true
		}
	}
	assert.False(t, found, "partial string should not match with exact match type")
}

func TestListProcesses_FilterByRegex(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)
	reader := newEndpointReader(r)

	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	pid1, _ := system.Root.SpawnNamed(props, "regex-test-123")
	pid2, _ := system.Root.SpawnNamed(props, "regex-test-abc")
	defer system.Root.Stop(pid1)
	defer system.Root.Stop(pid2)

	resp, err := reader.ListProcesses(context.Background(), &ListProcessesRequest{
		Pattern: `regex-test-\d+`,
		Type:    ListProcessesMatchType_MatchRegex,
	})
	require.NoError(t, err)

	found123 := false
	foundAbc := false
	for _, p := range resp.Pids {
		if p.Id == "regex-test-123" {
			found123 = true
		}
		if p.Id == "regex-test-abc" {
			foundAbc = true
		}
	}
	assert.True(t, found123, "regex should match digits suffix")
	assert.False(t, foundAbc, "regex should not match alpha suffix")
}

func TestListProcesses_InvalidRegex(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)
	reader := newEndpointReader(r)

	resp, err := reader.ListProcesses(context.Background(), &ListProcessesRequest{
		Pattern: "[invalid",
		Type:    ListProcessesMatchType_MatchRegex,
	})
	assert.Error(t, err, "invalid regex should return error")
	assert.Nil(t, resp)
}
