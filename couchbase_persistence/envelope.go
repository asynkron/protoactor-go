package couchbase_persistence

import (
	"encoding/json"
	"log"
	"reflect"

	proto "github.com/golang/protobuf/proto"
)

type envelope struct {
	Type       string          `json:"type"`
	Message    json.RawMessage `json:"event"`
	EventIndex int             `json:"eventIndex"`
	DocType    string          `json:"doctype"`
}

func newEnvelope(message proto.Message, doctype string, eventIndex int) *envelope {
	typeName := proto.MessageName(message)
	bytes, err := json.Marshal(message)
	if err != nil {
		log.Fatal(err)
	}
	envelope := &envelope{
		Type:       typeName,
		Message:    bytes,
		EventIndex: eventIndex,
		DocType:    "snapshot",
	}
	return envelope
}

func (envelope *envelope) message() proto.Message {
	t := proto.MessageType(envelope.Type).Elem()
	intPtr := reflect.New(t)
	instance := intPtr.Interface().(proto.Message)
	json.Unmarshal(envelope.Message, instance)
	return instance
}
