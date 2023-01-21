package cluster

import (
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
)

type PubSubBatch struct {
	Envelopes []interface{}
}

// Serialize converts a PubSubBatch to a PubSubBatchTransport.
func (b *PubSubBatch) Serialize() remote.RootSerialized {
	batch := &PubSubBatchTransport{
		TypeNames: make([]string, 0),
		Envelopes: make([]*PubSubEnvelope, 0),
	}

	for _, envelope := range b.Envelopes {
		var serializerId int32
		messageData, typeName, err := remote.Serialize(envelope, serializerId)
		if err != nil {
			panic(err)
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
	return batch
}

// Deserialize converts a PubSubBatchTransport to a PubSubBatch.
func (t *PubSubBatchTransport) Deserialize() remote.RootSerializable {
	b := &PubSubBatch{
		Envelopes: make([]interface{}, 0),
	}

	for _, envelope := range t.Envelopes {
		message, err := remote.Deserialize(envelope.MessageData, t.TypeNames[envelope.TypeId], envelope.SerializerId)
		if err != nil {
			panic(err)
		}
		b.Envelopes = append(b.Envelopes, message)
	}
	return b
}

type DeliverBatchRequest struct {
	Subscribers *Subscribers
	PubSubBatch *PubSubBatch
	Topic       string
}

func (d *DeliverBatchRequest) Serialize() remote.RootSerialized {
	return &DeliverBatchRequestTransport{
		Subscribers: d.Subscribers,
		Batch:       d.PubSubBatch.Serialize().(*PubSubBatchTransport),
		Topic:       d.Topic,
	}
}

func (t *DeliverBatchRequestTransport) Deserialize() remote.RootSerializable {
	return &DeliverBatchRequest{
		Subscribers: t.Subscribers,
		PubSubBatch: t.Batch.Deserialize().(*PubSubBatch),
		Topic:       t.Topic,
	}
}

type PubSubAutoRespondBatch struct {
	Envelopes []interface{}
}

// Serialize converts a PubSubAutoRespondBatch to a PubSubAutoRespondBatchTransport.
func (b *PubSubAutoRespondBatch) Serialize() remote.RootSerialized {
	batch := &PubSubBatch{Envelopes: b.Envelopes}
	transport := batch.Serialize().(*PubSubBatchTransport)
	return &PubSubAutoRespondBatchTransport{
		TypeNames: transport.TypeNames,
		Envelopes: transport.Envelopes,
	}
}

// GetAutoResponse returns a PublishResponse.
func (b *PubSubAutoRespondBatch) GetAutoResponse(_ actor.Context) interface{} {
	return &PublishResponse{
		Status: PublishStatus_Ok,
	}
}

// GetMessages returns the message.
func (b *PubSubAutoRespondBatch) GetMessages() []interface{} {
	return b.Envelopes
}

// Deserialize converts a PubSubAutoRespondBatchTransport to a PubSubAutoRespondBatch.
func (t *PubSubAutoRespondBatchTransport) Deserialize() remote.RootSerializable {
	batch := &PubSubBatchTransport{
		TypeNames: t.TypeNames,
		Envelopes: t.Envelopes,
	}
	return &PubSubAutoRespondBatch{
		Envelopes: batch.Deserialize().(*PubSubBatch).Envelopes,
	}
}
