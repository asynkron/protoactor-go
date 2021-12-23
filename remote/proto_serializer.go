package remote

import (
	"fmt"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

type protoSerializer struct {
	typeLookup map[string]protoreflect.MessageDescriptor
}

func newProtoSerializer() *protoSerializer {
	return &protoSerializer{
		typeLookup: make(map[string]protoreflect.MessageDescriptor),
	}
}

func (p *protoSerializer) Serialize(msg interface{}) ([]byte, error) {
	if message, ok := msg.(proto.Message); ok {
		bytes, err := proto.Marshal(message)
		if err != nil {
			return nil, err
		}

		return bytes, nil
	}
	return nil, fmt.Errorf("msg must be proto.Message")
}

func (p *protoSerializer) Deserialize(typeName string, bytes []byte) (interface{}, error) {
	md := p.typeLookup[typeName]
	if md == nil {
		return nil, fmt.Errorf("unknown message type %v", typeName)
	}

	//Wrong
	//mt := dynamicpb.NewMessageType(md)
	//pm := mt.New().Interface()
	proto.Unmarshal(bytes, pm)
	return pm, nil
}

func (protoSerializer) GetTypeName(msg interface{}) (string, error) {
	if message, ok := msg.(proto.Message); ok {
		typeName := proto.MessageName(message)

		return string(typeName), nil
	}
	return "", fmt.Errorf("msg must be proto.Message")
}
