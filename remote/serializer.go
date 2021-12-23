package remote

import (
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Serialization struct {
	serializers []Serializer
	p           *protoSerializer
}

func NewSerialization() *Serialization {
	s := &Serialization{}

	s.p = newProtoSerializer()
	s.RegisterSerializer(s.p)
	s.RegisterSerializer(newJsonSerializer())

	return s
}

func (s *Serialization) RegisterSerializer(serializer Serializer) {
	s.serializers = append(s.serializers, serializer)
}

func (s *Serialization) RegisterFileDescriptor(desc protoreflect.FileDescriptor) {
	messages := desc.Messages()
	for i := 0; i < messages.Len(); i++ {
		message := messages.Get(i)

		s.p.typeLookup[string(message.FullName())] = message
	}
}

type Serializer interface {
	Serialize(msg interface{}) ([]byte, error)
	Deserialize(typeName string, bytes []byte) (interface{}, error)
	GetTypeName(msg interface{}) (string, error)
}

func (s *Serialization) Serialize(message interface{}, serializerID int32) ([]byte, string, error) {
	res, err := s.serializers[serializerID].Serialize(message)
	typeName, err := s.serializers[serializerID].GetTypeName(message)
	return res, typeName, err
}

func (s *Serialization) Deserialize(message []byte, typeName string, serializerID int32) (interface{}, error) {
	return s.serializers[serializerID].Deserialize(typeName, message)
}
