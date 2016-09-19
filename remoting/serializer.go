package remoting

import (
	"reflect"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/remoting/messages"
	"github.com/golang/protobuf/proto"
)

func packMessage(message proto.Message, target *actor.PID) (*messages.MessageEnvelope, error) {
	typeName := proto.MessageName(message)
	bytes, err := proto.Marshal(message)
	if err != nil {
		return nil, err
	}
	envelope := &messages.MessageEnvelope{
		TypeName:    typeName,
		MessageData: bytes,
		Target:      target,
	}

	return envelope, nil
}

func unpackMessage(message *messages.MessageEnvelope) proto.Message {
	t := proto.MessageType(message.TypeName).Elem()
	intPtr := reflect.New(t)
	instance := intPtr.Interface().(proto.Message)
	proto.Unmarshal(message.MessageData, instance)
	return instance
}
