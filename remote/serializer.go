package remote

import (
	"reflect"

	"os"

	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/gogo/protobuf/proto"
)

func serialize(message proto.Message) ([]byte, string, error) {
	typeName := proto.MessageName(message)
	ensureGoGo(typeName)
	bytes, err := proto.Marshal(message)
	if err != nil {
		return nil, "", err
	}

	return bytes, typeName, nil
}

func deserialize(message *MessageEnvelope, typeName string) proto.Message {

	ensureGoGo(typeName)
	t1 := proto.MessageType(typeName)
	if t1 == nil {
		plog.Error("Unknown message type", log.String("type", typeName))
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
