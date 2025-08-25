package remote

import "fmt"

var (
	// DefaultSerializerID is used when no specific serializer is requested.
	DefaultSerializerID int32
	serializers         []Serializer
)

func init() {
	RegisterSerializer(newProtoSerializer())
	RegisterSerializer(newJSONSerializer())
}

// RegisterSerializer registers a Serializer implementation.
func RegisterSerializer(serializer Serializer) {
	serializers = append(serializers, serializer)
}

// Serializer defines how messages are encoded and decoded.
type Serializer interface {
	Serialize(msg interface{}) ([]byte, error)
	Deserialize(typeName string, bytes []byte) (interface{}, error)
	GetTypeName(msg interface{}) (string, error)
}

// Serialize encodes a message using the specified serializer.
// An error is returned if the serializerID is out of range or serialization fails.
func Serialize(message interface{}, serializerID int32) ([]byte, string, error) {
	index := int(serializerID)
	if index < 0 || index >= len(serializers) {
		return nil, "", fmt.Errorf("serializerID %d out of range", serializerID)
	}

	res, err := serializers[index].Serialize(message)
	if err != nil {
		return nil, "", err
	}
	typeName, err := serializers[index].GetTypeName(message)
	if err != nil {
		return nil, "", err
	}
	return res, typeName, nil
}

// Deserialize decodes a message using the specified serializer.
// An error is returned if the serializerID is out of range or deserialization fails.
func Deserialize(message []byte, typeName string, serializerID int32) (interface{}, error) {
	index := int(serializerID)
	if index < 0 || index >= len(serializers) {
		return nil, fmt.Errorf("serializerID %d out of range", serializerID)
	}

	return serializers[index].Deserialize(typeName, message)
}

// RootSerializable is the root level in-process representation of a message
type RootSerializable interface {
	// Serialize returns the on-the-wire representation of the message
	//   Message -> IRootSerialized -> ByteString
	Serialize() (RootSerialized, error)
}

// RootSerialized is the root level on-the-wire representation of a message
type RootSerialized interface {
	// Deserialize returns the in-process representation of a message
	//   ByteString -> IRootSerialized -> Message
	Deserialize() (RootSerializable, error)
}
