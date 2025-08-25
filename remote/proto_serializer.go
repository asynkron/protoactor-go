package remote

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

type protoSerializer struct{}

func newProtoSerializer() *protoSerializer {
	return &protoSerializer{}
}

// Serialize converts a protobuf message into its binary form.
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

// Deserialize creates a protobuf message of the given type and fills it with the provided bytes.
func (p *protoSerializer) Deserialize(typeName string, bytes []byte) (interface{}, error) {
	mt, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(typeName))
	if err != nil {
		return nil, err
	}

	pm := mt.New().Interface()

	err = proto.Unmarshal(bytes, pm)
	return pm, err
}

// GetTypeName returns the fully qualified name of a protobuf message.
func (protoSerializer) GetTypeName(msg interface{}) (string, error) {
	if message, ok := msg.(proto.Message); ok {
		typeName := proto.MessageName(message)

		return string(typeName), nil
	}
	return "", fmt.Errorf("msg must be proto.Message")
}
