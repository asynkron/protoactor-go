package remote

var (
	DefaultSerializerID int32
	serializers         []Serializer
)

func init() {
	RegisterSerializer(newProtoSerializer())
	RegisterSerializer(newJsonSerializer())
}

func RegisterSerializer(serializer Serializer) {
	serializers = append(serializers, serializer)
}

type Serializer interface {
	Serialize(msg interface{}) ([]byte, error)
	Deserialize(typeName string, bytes []byte) (interface{}, error)
	GetTypeName(msg interface{}) (string, error)
}

func Serialize(message interface{}, serializerID int32) ([]byte, string, error) {
	res, err := serializers[serializerID].Serialize(message)
	typeName, err := serializers[serializerID].GetTypeName(message)
	return res, typeName, err
}

func Deserialize(message []byte, typeName string, serializerID int32) (interface{}, error) {
	return serializers[serializerID].Deserialize(typeName, message)
}

// RootSerializable is the root level in-process representation of a message
type RootSerializable interface {
	// Serialize returns the on-the-wire representation of the message
	//   Message -> IRootSerialized -> ByteString
	Serialize() RootSerialized
}

// RootSerialized is the root level on-the-wire representation of a message
type RootSerialized interface {
	// Deserialize returns the in-process representation of a message
	//   ByteString -> IRootSerialized -> Message
	Deserialize() RootSerializable
}
