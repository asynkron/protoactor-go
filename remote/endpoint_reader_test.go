package remote

import (
	"context"
	"testing"
	"time"

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

func TestGetProcessDiagnostics_ExistingProcess(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)
	reader := newEndpointReader(r)

	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	pid, err := system.Root.SpawnNamed(props, "diag-actor")
	require.NoError(t, err)
	defer system.Root.Stop(pid)

	resp, err := reader.GetProcessDiagnostics(context.Background(), &GetProcessDiagnosticsRequest{
		Pid: pid,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.DiagnosticsString, "should return diagnostics for existing process")
}

func TestGetProcessDiagnostics_NonExistentProcess(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)
	reader := newEndpointReader(r)

	resp, err := reader.GetProcessDiagnostics(context.Background(), &GetProcessDiagnosticsRequest{
		Pid: actor.NewPID("nonhost", "nonexistent"),
	})
	assert.Error(t, err, "should return error for non-existent process")
	assert.Nil(t, resp)
}

func TestGetProcessDiagnostics_NilPid(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)
	reader := newEndpointReader(r)

	resp, err := reader.GetProcessDiagnostics(context.Background(), &GetProcessDiagnosticsRequest{
		Pid: nil,
	})
	assert.Error(t, err, "should return error for nil pid")
	assert.Nil(t, resp)
}

type mockReceiveServer struct {
	Remoting_ReceiveServer
	sentMessages []*RemoteMessage
}

func (m *mockReceiveServer) Send(msg *RemoteMessage) error {
	m.sentMessages = append(m.sentMessages, msg)
	return nil
}

func TestOnConnectRequest_ClientConnection(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)
	reader := newEndpointReader(r)

	stream := &mockReceiveServer{}
	_, err := reader.OnConnectRequest(stream, &ConnectRequest{
		ConnectionType: &ConnectRequest_ClientConnection{
			ClientConnection: &ClientConnection{
				MemberId: "client-1",
			},
		},
	})
	require.NoError(t, err)

	require.Len(t, stream.sentMessages, 1, "should send a ConnectResponse")
	connectResp := stream.sentMessages[0].GetConnectResponse()
	require.NotNil(t, connectResp, "response should be a ConnectResponse")
	assert.Equal(t, system.ID, connectResp.MemberId)
	assert.False(t, connectResp.Blocked)
}

func TestOnConnectRequest_ClientConnection_Blocked(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)
	reader := newEndpointReader(r)

	r.BlockList().Block("blocked-client")

	stream := &mockReceiveServer{}
	_, err := reader.OnConnectRequest(stream, &ConnectRequest{
		ConnectionType: &ConnectRequest_ClientConnection{
			ClientConnection: &ClientConnection{
				MemberId: "blocked-client",
			},
		},
	})
	require.NoError(t, err)

	require.Len(t, stream.sentMessages, 1)
	connectResp := stream.sentMessages[0].GetConnectResponse()
	require.NotNil(t, connectResp)
	assert.True(t, connectResp.Blocked)
	assert.Equal(t, system.ID, connectResp.MemberId)
}

// TestOnMessageBatch_DeserializationFailure_ContinuesProcessing verifies that
// a deserialization failure for one envelope doesn't kill the entire batch.
func TestOnMessageBatch_DeserializationFailure_ContinuesProcessing(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)
	reader := newEndpointReader(r)

	// Spawn a local actor that will receive the good message
	received := make(chan any, 1)
	props := actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *actor.Started, *actor.Stopping, *actor.Stopped:
			// ignore lifecycle
		default:
			received <- ctx.Message()
		}
	})
	pid, err := system.Root.SpawnNamed(props, "batch-test-target")
	require.NoError(t, err)
	defer system.Root.Stop(pid)

	batch := &MessageBatch{
		TypeNames: []string{
			"totally.bogus.NonExistentType", // index 0: will fail deserialization
			"actor.PID",                     // index 1: valid type
		},
		Targets: []string{pid.Id},
		Senders: []*actor.PID{},
		Envelopes: []*MessageEnvelope{
			{
				TypeId:      0, // bogus type
				Target:      0,
				Sender:      0,
				MessageData: []byte{0x01, 0x02}, // garbage data
			},
			{
				TypeId:      1, // valid type (actor.PID)
				Target:      0,
				Sender:      0,
				MessageData: []byte{}, // empty PID
			},
		},
	}

	err = reader.onMessageBatch(batch)
	assert.NoError(t, err, "batch should succeed even with one bad envelope")

	// The good message should have been delivered
	select {
	case <-received:
		// good — the second envelope was delivered
	case <-time.After(2 * time.Second):
		t.Fatal("good message in batch should still be delivered after bad envelope is skipped")
	}
}

// TestOnMessageBatch_DeserializationFailure_NotifiesSender verifies that when
// deserialization fails and the envelope has a sender, a DeadLetterResponse
// is sent back.
func TestOnMessageBatch_DeserializationFailure_NotifiesSender(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)
	reader := newEndpointReader(r)

	// Create a future to act as the sender
	fut := actor.NewFuture(system, 5*time.Second)

	// Spawn a dummy target
	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	pid, err := system.Root.SpawnNamed(props, "deser-notify-target")
	require.NoError(t, err)
	defer system.Root.Stop(pid)

	senderPID := fut.PID()

	batch := &MessageBatch{
		TypeNames: []string{"totally.bogus.NonExistentType"},
		Targets:   []string{pid.Id},
		Senders:   []*actor.PID{senderPID},
		Envelopes: []*MessageEnvelope{
			{
				TypeId:          0,
				Target:          0,
				Sender:          1, // 1-based index into Senders
				SenderRequestId: senderPID.RequestId,
				MessageData:     []byte{0x01, 0x02},
			},
		},
	}

	err = reader.onMessageBatch(batch)
	assert.NoError(t, err, "batch should not return error for deserialization failure")

	// The sender's future should resolve with ErrDeadLetter
	_, futErr := fut.Result()
	assert.ErrorIs(t, futErr, actor.ErrDeadLetter,
		"sender should receive ErrDeadLetter when deserialization fails")
}

// TestOnMessageBatch_EmptyBatch verifies that an empty batch succeeds.
func TestOnMessageBatch_EmptyBatch(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("localhost", 0)
	r := NewRemote(system, config)
	reader := newEndpointReader(r)

	batch := &MessageBatch{
		TypeNames: []string{},
		Targets:   []string{},
		Senders:   []*actor.PID{},
		Envelopes: []*MessageEnvelope{},
	}

	err := reader.onMessageBatch(batch)
	assert.NoError(t, err, "empty batch should succeed")
}

// TestOnMessageBatch_NegativeTypeId verifies negative type IDs are handled.
func TestOnMessageBatch_NegativeTypeId(t *testing.T) {
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
				TypeId: -1,
				Target: 0,
				Sender: 0,
			},
		},
	}

	err := reader.onMessageBatch(batch)
	assert.Error(t, err, "negative type ID should return error")
}

func TestDeserializeSender_NilPidInArray(t *testing.T) {
	arr := []*actor.PID{nil}

	// index=1 (1-based) with requestID > 0 triggers the Clone path
	assert.NotPanics(t, func() {
		result := deserializeSender(1, 42, arr)
		assert.Nil(t, result, "nil pid in array should return nil")
	})
}
