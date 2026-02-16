package remote

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

type jsonSerializer struct {
	marshaler   protojson.MarshalOptions
	unmarshaler protojson.UnmarshalOptions
}

func newJSONSerializer() Serializer {
	return &jsonSerializer{
		marshaler:   protojson.MarshalOptions{},
		unmarshaler: protojson.UnmarshalOptions{DiscardUnknown: true},
	}
}

func (j *jsonSerializer) Serialize(msg any) ([]byte, error) {
	if message, ok := msg.(*JSONMessage); ok {
		return []byte(message.JSON), nil
	} else if message, ok := msg.(proto.Message); ok {
		b, err := j.marshaler.Marshal(message)
		if err != nil {
			return nil, err
		}

		return b, nil
	}
	return nil, fmt.Errorf("msg must be proto.Message")
}

func (j *jsonSerializer) Deserialize(typeName string, b []byte) (any, error) {
	mt, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(typeName))
	if err != nil {
		// Type not found in the proto registry; wrap in a JSONMessage.
		m := &JSONMessage{
			TypeName: typeName,
			JSON:     string(b),
		}
		return m, nil
	}

	instance := mt.New().Interface()
	if err := j.unmarshaler.Unmarshal(b, instance); err != nil {
		return nil, err
	}

	return instance, nil
}

func (j *jsonSerializer) GetTypeName(msg any) (string, error) {
	if message, ok := msg.(*JSONMessage); ok {
		return message.TypeName, nil
	} else if message, ok := msg.(proto.Message); ok {
		typeName := proto.MessageName(message)

		return string(typeName), nil
	}

	return "", fmt.Errorf("msg must be proto.Message")
}
