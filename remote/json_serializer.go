package remote

import (
	"bytes"
	"fmt"
	"reflect"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
)

type jsonSerializer struct {
	jsonpb.Marshaler
	jsonpb.Unmarshaler
}

func newJsonSerializer() Serializer {
	return &jsonSerializer{
		Marshaler: jsonpb.Marshaler{},
		Unmarshaler: jsonpb.Unmarshaler{
			AllowUnknownFields: true,
		},
	}
}

func (j *jsonSerializer) Serialize(msg interface{}) ([]byte, error) {
	if message, ok := msg.(*JsonMessage); ok {
		return []byte(message.Json), nil
	} else if message, ok := msg.(proto.Message); ok {

		str, err := j.Marshaler.MarshalToString(message)
		if err != nil {
			return nil, err
		}

		return []byte(str), nil
	}
	return nil, fmt.Errorf("msg must be proto.Message")
}

func (j *jsonSerializer) Deserialize(typeName string, b []byte) (interface{}, error) {
	protoType := proto.MessageType(typeName)
	if protoType == nil {
		m := &JsonMessage{
			TypeName: typeName,
			Json:     string(b),
		}
		return m, nil
	}
	t := protoType.Elem()

	intPtr := reflect.New(t)
	instance, ok := intPtr.Interface().(proto.Message)
	if ok {
		r := bytes.NewReader(b)
		j.Unmarshaler.Unmarshal(r, instance)

		return instance, nil
	}

	return nil, fmt.Errorf("msg must be proto.Message")
}

func (j *jsonSerializer) GetTypeName(msg interface{}) (string, error) {
	if message, ok := msg.(*JsonMessage); ok {
		return message.TypeName, nil
	} else if message, ok := msg.(proto.Message); ok {
		typeName := proto.MessageName(message)

		return typeName, nil
	}

	return "", fmt.Errorf("msg must be proto.Message")
}
