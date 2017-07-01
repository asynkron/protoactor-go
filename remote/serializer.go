package remote

var DefaultSerializerID int32 = 0
var serializers []Serializer

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

func serialize(message interface{}, serializerID int32) ([]byte, string, error) {
	res, err := serializers[serializerID].Serialize(message)
	typeName, err := serializers[serializerID].GetTypeName(message)
	return res, typeName, err
}

func deserialize(message []byte, typeName string, serializerID int32) interface{} {
	res, _ := serializers[serializerID].Deserialize(typeName, message)
	return res
}
