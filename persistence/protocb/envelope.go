package protocb

import (
	"encoding/json"
	"log"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

type envelope struct {
	Type       string          `json:"type"`       // reflected message type so we can deserialize back
	Message    json.RawMessage `json:"event"`      // this is still protobuf but the json form
	EventIndex int             `json:"eventIndex"` // event index in the event stream
	DocType    string          `json:"doctype"`    // type snapshot or event
}

func newEnvelope(message proto.Message, doctype string, eventIndex int) *envelope {
	typeName := proto.MessageName(message)
	bytes, err := json.Marshal(message)
	if err != nil {
		log.Fatal(err)
	}
	envelope := &envelope{
		Type:       string(typeName),
		Message:    bytes,
		EventIndex: eventIndex,
		DocType:    doctype,
	}
	return envelope
}

func (envelope *envelope) message() proto.Message {
	mt, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(envelope.Type))
	if err != nil {
		log.Fatal(err)
	}

	pm := mt.New().Interface()
	err = json.Unmarshal(envelope.Message, pm)
	if err != nil {
		log.Fatal(err)
	}
	return pm
}
