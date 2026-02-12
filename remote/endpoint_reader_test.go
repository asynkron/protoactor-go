package remote

import (
	"testing"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
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
