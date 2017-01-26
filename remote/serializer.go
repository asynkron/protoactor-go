package remote

import (
	"reflect"

	"os"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/gogo/protobuf/proto"
)

func serialize(message proto.Message, target *actor.PID, sender *actor.PID) (*MessageEnvelope, error) {
	typeName := proto.MessageName(message)
	ensureGoGo(typeName)
	bytes, err := proto.Marshal(message)
	if err != nil {
		return nil, err
	}
	envelope := &MessageEnvelope{
		TypeName:    typeName,
		MessageData: bytes,
		Target:      target,
		Sender:      sender,
	}

	return envelope, nil
}

func deserialize(message *MessageEnvelope) proto.Message {

	ensureGoGo(message.TypeName)
	t1 := proto.MessageType(message.TypeName)
	if t1 == nil {
		plog.Error("Unknown message type", log.String("type", message.TypeName))
		os.Exit(1)
	}
	t := t1.Elem()

	intPtr := reflect.New(t)
	instance := intPtr.Interface().(proto.Message)
	proto.Unmarshal(message.MessageData, instance)

	return instance
}

func ensureGoGo(typeName string) {
	if typeName == "" {
		plog.Error("Message type name is empty string, make sure you have generated the Proto contacts with GOGO Proto: github.com/gogo/protobuf/proto")
		os.Exit(1)
	}
}
