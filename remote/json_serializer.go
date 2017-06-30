package remote

import (
	"bytes"
	"fmt"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	"reflect"
)

type jsonSerializer struct {
	jsonpb.Marshaler
	jsonpb.Unmarshaler
}

func newJsonSerializer() Serializer {
	return &jsonSerializer{
		Marshaler:   jsonpb.Marshaler{},
		Unmarshaler: jsonpb.Unmarshaler{},
	}
}

func (json *jsonSerializer) Serialize(msg interface{}) ([]byte, error) {
	if message, ok := msg.(proto.Message); ok {

		str, err := json.MarshalToString(message)
		if err != nil {
			return nil, err
		}

		return []byte(str), nil
	}
	return nil, fmt.Errorf("msg must be proto.Message")
}

func (json *jsonSerializer) Deserialize(typeName string, b []byte) (interface{}, error) {
	protoType := proto.MessageType(typeName)
	if protoType == nil {
		return nil, fmt.Errorf("Unknown message type %v", typeName)
	}
	t := protoType.Elem()

	intPtr := reflect.New(t)
	instance := intPtr.Interface().(proto.Message)
	r := bytes.NewReader(b)
	json.Unmarshal(r, instance)

	return instance, nil
}

func (json *jsonSerializer) GetTypeName(msg interface{}) (string, error) {
	if message, ok := msg.(proto.Message); ok {
		typeName := proto.MessageName(message)

		return typeName, nil
	}

	return "", fmt.Errorf("msg must be proto.Message")
}
