package remote

var DefaultSerializerID int32
var serializers []Serializer

func init() {
	RegisterSerializer(newProtoSerializer())
	RegisterSerializer(newJsonSerializer())
}

func RegisterSerializerAsDefault(serializer Serializer) {
	serializers = append(serializers, serializer)
	DefaultSerializerID = int32(len(serializers) - 1)
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
