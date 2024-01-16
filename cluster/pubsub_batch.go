package cluster

import (
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"google.golang.org/protobuf/proto"
)

var (
	_ remote.RootSerializable = (*PubSubBatch)(nil)
	_ remote.RootSerializable = (*DeliverBatchRequest)(nil)
	_ remote.RootSerializable = (*PubSubAutoRespondBatch)(nil)

	_ remote.RootSerialized = (*PubSubBatchTransport)(nil)
	_ remote.RootSerialized = (*DeliverBatchRequestTransport)(nil)
	_ remote.RootSerialized = (*PubSubAutoRespondBatchTransport)(nil)
)

type PubSubBatch struct {
	Envelopes []proto.Message
}

// Serialize converts a PubSubBatch to a PubSubBatchTransport.
func (b *PubSubBatch) Serialize() (remote.RootSerialized, error) {
	batch := &PubSubBatchTransport{
		TypeNames: make([]string, 0),
		Envelopes: make([]*PubSubEnvelope, 0),
	}

	for _, envelope := range b.Envelopes {
		var serializerId int32
		messageData, typeName, err := remote.Serialize(envelope, serializerId)
		if err != nil {
			return nil, err
		}
		// batch.TypeNames.IndexOf(typeName)
		typeIndex := -1
		for i, t := range batch.TypeNames {
			if t == typeName {
				typeIndex = i
				break
			}
		}
		if typeIndex == -1 {
			batch.TypeNames = append(batch.TypeNames, typeName)
			typeIndex = len(batch.TypeNames) - 1
		}
		batch.Envelopes = append(batch.Envelopes, &PubSubEnvelope{
			MessageData:  messageData,
			TypeId:       int32(typeIndex),
			SerializerId: serializerId,
		})
	}
	return batch, nil
}

// Deserialize converts a PubSubBatchTransport to a PubSubBatch.
func (t *PubSubBatchTransport) Deserialize() (remote.RootSerializable, error) {
	b := &PubSubBatch{
		Envelopes: make([]proto.Message, 0),
	}

	for _, envelope := range t.Envelopes {
		message, err := remote.Deserialize(envelope.MessageData, t.TypeNames[envelope.TypeId], envelope.SerializerId)
		if err != nil {
			return nil, err
		}
		protoMessage, ok := message.(proto.Message)
		if !ok {
			panic("message is not proto.Message")
		}

		b.Envelopes = append(b.Envelopes, protoMessage)
	}
	return b, nil
}

type DeliverBatchRequest struct {
	Subscribers *Subscribers
	PubSubBatch *PubSubBatch
	Topic       string
}

func (d *DeliverBatchRequest) Serialize() (remote.RootSerialized, error) {
	rs, err := d.PubSubBatch.Serialize()
	if err != nil {
		return nil, err
	}

	return &DeliverBatchRequestTransport{
		Subscribers: d.Subscribers,
		Batch:       rs.(*PubSubBatchTransport),
		Topic:       d.Topic,
	}, nil
}

func (t *DeliverBatchRequestTransport) Deserialize() (remote.RootSerializable, error) {
	rs, err := t.Batch.Deserialize()
	if err != nil {
		return nil, err
	}

	return &DeliverBatchRequest{
		Subscribers: t.Subscribers,
		PubSubBatch: rs.(*PubSubBatch),
		Topic:       t.Topic,
	}, nil
}

var _ actor.MessageBatch = (*PubSubAutoRespondBatch)(nil)

type PubSubAutoRespondBatch struct {
	Envelopes []proto.Message
}

// Serialize converts a PubSubAutoRespondBatch to a PubSubAutoRespondBatchTransport.
func (b *PubSubAutoRespondBatch) Serialize() (remote.RootSerialized, error) {
	batch := &PubSubBatch{Envelopes: b.Envelopes}

	rs, err := batch.Serialize()
	if err != nil {
		return nil, err
	}

	return &PubSubAutoRespondBatchTransport{
		TypeNames: rs.(*PubSubBatchTransport).TypeNames,
		Envelopes: rs.(*PubSubBatchTransport).Envelopes,
	}, nil
}

// GetAutoResponse returns a PublishResponse.
func (b *PubSubAutoRespondBatch) GetAutoResponse(_ actor.Context) interface{} {
	return &PublishResponse{
		Status: PublishStatus_Ok,
	}
}

// GetMessages returns the message.
func (b *PubSubAutoRespondBatch) GetMessages() []interface{} {
	var messages []interface{}
	for _, envelope := range b.Envelopes {
		messages = append(messages, envelope)
	}
	return messages
}

// Deserialize converts a PubSubAutoRespondBatchTransport to a PubSubAutoRespondBatch.
func (t *PubSubAutoRespondBatchTransport) Deserialize() (remote.RootSerializable, error) {
	batch := &PubSubBatchTransport{
		TypeNames: t.TypeNames,
		Envelopes: t.Envelopes,
	}
	rs, err := batch.Deserialize()
	if err != nil {
		return nil, err
	}

	return &PubSubAutoRespondBatch{
		Envelopes: rs.(*PubSubBatch).Envelopes,
	}, nil
}
