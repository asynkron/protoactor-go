package remote

import (
	"testing"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

func TestJsonSerializer_round_trip(t *testing.T) {
	m := &ActorPidRequest{
		Kind: "abc",
		Name: "def",
	}
	b, typeName, _ := Serialize(m, 1)
	res, err := Deserialize(b, typeName, 1)

	assert.Nil(t, err)

	typed := res.(*ActorPidRequest)
	assert.Equal(t, "remote.ActorPidRequest", typeName)
	assert.Equal(t, m.Kind, typed.Kind)
	assert.Equal(t, m.Name, typed.Name)
}

func TestJsonSerializer_Serialize_PID(t *testing.T) {
	system := actor.NewActorSystem()
	m := system.NewLocalPID("foo")
	b, typeName, _ := Serialize(m, 1)
	res, err := Deserialize(b, typeName, 1)

	assert.Nil(t, err)

	typed := res.(*actor.PID)
	assert.Equal(t, "actor.PID", typeName)
	assert.True(t, m.Equal(typed))
}

func TestJsonSerializer_Deserialize_UnknownType(t *testing.T) {
	b := []byte(`{"key":"value"}`)
	res, err := Deserialize(b, "unknown.Type", 1)

	assert.Nil(t, err)

	typed := res.(*JSONMessage)
	assert.Equal(t, "unknown.Type", typed.TypeName)
	assert.Equal(t, `{"key":"value"}`, typed.JSON)
}

func TestProtobufSerializer_Serialize_PID(t *testing.T) {
	system := actor.NewActorSystem()
	m := system.NewLocalPID("foo")
	b, typeName, _ := Serialize(m, 0)
	res, err := Deserialize(b, typeName, 0)

	assert.Nil(t, err)

	typed := res.(*actor.PID)
	assert.Equal(t, "actor.PID", typeName)
	assert.True(t, m.Equal(typed))
}

func TestSerialize_InvalidSerializerID(t *testing.T) {
	_, _, err := Serialize("msg", int32(len(serializers)))
	assert.Error(t, err)
}

func TestDeserialize_InvalidSerializerID(t *testing.T) {
	_, err := Deserialize([]byte("{}"), "", int32(len(serializers)))
	assert.Error(t, err)
}

// TestProtobufSerializer_Deserialize_InvalidType ensures an error is returned when the message type is unknown.
func TestProtobufSerializer_Deserialize_InvalidType(t *testing.T) {
	_, err := Deserialize([]byte{}, "unknown.Type", 0)
	assert.Error(t, err)
}

// TestProtobufSerializer_Serialize_NonProtoMessage verifies that attempting to
// serialize a non-protobuf type returns an error.
func TestProtobufSerializer_Serialize_NonProtoMessage(t *testing.T) {
	_, _, err := Serialize("plain string", 0)
	assert.Error(t, err, "non-proto.Message should fail serialization")
}

// TestProtobufSerializer_Serialize_NilMessage verifies behavior with nil.
func TestProtobufSerializer_Serialize_NilMessage(t *testing.T) {
	_, _, err := Serialize(nil, 0)
	assert.Error(t, err, "nil message should fail serialization")
}

// TestSerialize_NegativeSerializerID verifies bounds checking.
func TestSerialize_NegativeSerializerID(t *testing.T) {
	_, _, err := Serialize(&ActorPidRequest{}, -1)
	assert.Error(t, err, "negative serializer ID should fail")
}

// TestDeserialize_NegativeSerializerID verifies bounds checking.
func TestDeserialize_NegativeSerializerID(t *testing.T) {
	_, err := Deserialize([]byte{}, "actor.PID", -1)
	assert.Error(t, err, "negative serializer ID should fail")
}

// TestDeserialize_CorruptedData verifies that corrupted protobuf data returns error.
func TestDeserialize_CorruptedData(t *testing.T) {
	_, err := Deserialize([]byte{0xFF, 0xFE, 0xFD, 0xFC}, "actor.PID", 0)
	assert.Error(t, err, "corrupted protobuf data should fail deserialization")
}
