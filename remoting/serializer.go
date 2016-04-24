package remoting

import "reflect"
import proto "github.com/golang/protobuf/proto"
import "github.com/rogeralsing/gam"

func PackMessage(message proto.Message,target *gam.PID) (*gam.MessageEnvelope, error) {
	typeName := proto.MessageName(message)
	bytes, err := proto.Marshal(message)
	if err != nil {
		return nil, err
	}
	envelope := &gam.MessageEnvelope{
		TypeName:    typeName,
		MessageData: bytes,
		Target: target,
	}

	return envelope, nil
}

func UnpackMessage(message *gam.MessageEnvelope) proto.Message {
	t := proto.MessageType(message.TypeName).Elem()
	intPtr := reflect.New(t)
	instance := intPtr.Interface().(proto.Message)
	proto.Unmarshal(message.MessageData, instance)
	return instance
}
