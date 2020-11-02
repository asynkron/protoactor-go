package protocb

import (
	"encoding/json"
	"log"
	"reflect"

	"github.com/golang/protobuf/proto"
)

type envelope struct {
	Type       string          `json:"type"`       //reflected message type so we can deserialize back
	Message    json.RawMessage `json:"event"`      //this is still protobuf but the json form
	EventIndex int             `json:"eventIndex"` //event index in the event stream
	DocType    string          `json:"doctype"`    //type snapshot or event
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
		DocType:    doctype,
	}
	return envelope
}

func (envelope *envelope) message() proto.Message {
	t := proto.MessageType(envelope.Type).Elem()
	intPtr := reflect.New(t)
	instance := intPtr.Interface().(proto.Message)
	err := json.Unmarshal(envelope.Message, instance)
	if err != nil {
		log.Fatal(err)
	}
	return instance
}
