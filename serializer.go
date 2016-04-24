package gam

import "reflect"
import proto "github.com/golang/protobuf/proto"

func PackMessage(message proto.Message,target *PID) (*MessageEnvelope, error) {
	typeName := proto.MessageName(message)
	bytes, err := proto.Marshal(message)
	if err != nil {
		return nil, err
	}
	envelope := &MessageEnvelope{
		TypeName:    typeName,
		MessageData: bytes,
		Target: target,
	}

	return envelope, nil
}

func UnpackMessage(message *MessageEnvelope) proto.Message {
	t := proto.MessageType(message.TypeName).Elem()
	intPtr := reflect.New(t)
	instance := intPtr.Interface().(proto.Message)
	proto.Unmarshal(message.MessageData, instance)
	return instance
}
