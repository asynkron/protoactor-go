package remote

import (
	"testing"

	"github.com/asynkron/protoactor-go/actor"
	"google.golang.org/protobuf/proto"
)

// BenchmarkProtoSerializer_Serialize benchmarks protobuf binary serialization
// of a realistic message (ActorPidResponse with nested PID).
func BenchmarkProtoSerializer_Serialize(b *testing.B) {
	msg := &ActorPidResponse{
		Pid:        &actor.PID{Address: "127.0.0.1:8080", Id: "actor/123"},
		StatusCode: 0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := Serialize(msg, 0)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkProtoSerializer_Deserialize benchmarks protobuf binary deserialization,
// which includes type lookup via the global proto registry.
func BenchmarkProtoSerializer_Deserialize(b *testing.B) {
	msg := &ActorPidResponse{
		Pid:        &actor.PID{Address: "127.0.0.1:8080", Id: "actor/123"},
		StatusCode: 0,
	}
	data, typeName, err := Serialize(msg, 0)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Deserialize(data, typeName, 0)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkProtoSerializer_RoundTrip benchmarks a full serialize-then-deserialize
// cycle, which is the critical path in remote message delivery.
func BenchmarkProtoSerializer_RoundTrip(b *testing.B) {
	msg := &ActorPidResponse{
		Pid:        &actor.PID{Address: "127.0.0.1:8080", Id: "actor/123"},
		StatusCode: 0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, typeName, err := Serialize(msg, 0)
		if err != nil {
			b.Fatal(err)
		}
		_, err = Deserialize(data, typeName, 0)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkJsonSerializer_Serialize benchmarks JSON serialization of a protobuf
// message (serializer ID 1).
func BenchmarkJsonSerializer_Serialize(b *testing.B) {
	msg := &ActorPidResponse{
		Pid:        &actor.PID{Address: "127.0.0.1:8080", Id: "actor/123"},
		StatusCode: 0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := Serialize(msg, 1)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkJsonSerializer_Deserialize benchmarks JSON deserialization of a protobuf
// message, including proto registry type lookup and JSON unmarshaling.
func BenchmarkJsonSerializer_Deserialize(b *testing.B) {
	msg := &ActorPidResponse{
		Pid:        &actor.PID{Address: "127.0.0.1:8080", Id: "actor/123"},
		StatusCode: 0,
	}
	data, typeName, err := Serialize(msg, 1)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Deserialize(data, typeName, 1)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMessageBatch_Serialization benchmarks protobuf serialization of a
// MessageBatch containing multiple envelopes, which is the wire format used
// by the endpoint writer to send batches of messages over gRPC.
func BenchmarkMessageBatch_Serialization(b *testing.B) {
	// Build a realistic batch with 100 envelopes
	envelopes := make([]*MessageEnvelope, 100)
	for i := 0; i < 100; i++ {
		innerMsg := &ActorPidRequest{Name: "test-actor", Kind: "testKind"}
		data, err := proto.Marshal(innerMsg)
		if err != nil {
			b.Fatal(err)
		}
		envelopes[i] = &MessageEnvelope{
			TypeId:      0,
			MessageData: data,
			Target:      int32(i % 10),
			Sender:      int32(i % 5),
		}
	}

	batch := &MessageBatch{
		TypeNames: []string{"remote.ActorPidRequest"},
		Targets:   []string{"target/0", "target/1", "target/2", "target/3", "target/4", "target/5", "target/6", "target/7", "target/8", "target/9"},
		Envelopes: envelopes,
		Senders: []*actor.PID{
			{Address: "127.0.0.1:8080", Id: "sender/0"},
			{Address: "127.0.0.1:8080", Id: "sender/1"},
			{Address: "127.0.0.1:8080", Id: "sender/2"},
			{Address: "127.0.0.1:8080", Id: "sender/3"},
			{Address: "127.0.0.1:8080", Id: "sender/4"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := proto.Marshal(batch)
		if err != nil {
			b.Fatal(err)
		}
		_ = data
	}
}

// BenchmarkMessageBatch_Deserialization benchmarks protobuf deserialization of
// a MessageBatch, the counterpart to BenchmarkMessageBatch_Serialization.
func BenchmarkMessageBatch_Deserialization(b *testing.B) {
	envelopes := make([]*MessageEnvelope, 100)
	for i := 0; i < 100; i++ {
		innerMsg := &ActorPidRequest{Name: "test-actor", Kind: "testKind"}
		data, err := proto.Marshal(innerMsg)
		if err != nil {
			b.Fatal(err)
		}
		envelopes[i] = &MessageEnvelope{
			TypeId:      0,
			MessageData: data,
			Target:      int32(i % 10),
			Sender:      int32(i % 5),
		}
	}

	batch := &MessageBatch{
		TypeNames: []string{"remote.ActorPidRequest"},
		Targets:   []string{"target/0", "target/1", "target/2", "target/3", "target/4", "target/5", "target/6", "target/7", "target/8", "target/9"},
		Envelopes: envelopes,
		Senders: []*actor.PID{
			{Address: "127.0.0.1:8080", Id: "sender/0"},
			{Address: "127.0.0.1:8080", Id: "sender/1"},
			{Address: "127.0.0.1:8080", Id: "sender/2"},
			{Address: "127.0.0.1:8080", Id: "sender/3"},
			{Address: "127.0.0.1:8080", Id: "sender/4"},
		},
	}

	serialized, err := proto.Marshal(batch)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out := &MessageBatch{}
		if err := proto.Unmarshal(serialized, out); err != nil {
			b.Fatal(err)
		}
	}
}
