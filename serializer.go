package gam

import proto "github.com/golang/protobuf/proto"

func NewMessageEnvelope(message proto.Message) (*MessageEnvelope, error) {
	typeName := proto.MessageName(message)
	bytes, err := proto.Marshal(message)
	if err != nil {
		return nil, err
	}
	envelope := &MessageEnvelope{
		TypeName:    typeName,
		MessageData: bytes,
	}

	return envelope, nil
}
