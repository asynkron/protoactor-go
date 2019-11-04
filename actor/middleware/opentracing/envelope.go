package opentracing

import (
	"github.com/otherview/protoactor-go/actor"
	"github.com/opentracing/opentracing-go"
)

type messageHeaderReader struct {
	ReadOnlyMessageHeader actor.ReadonlyMessageHeader
}

func (reader *messageHeaderReader) ForeachKey(handler func(key, val string) error) error {
	if reader.ReadOnlyMessageHeader == nil {
		return nil
	}
	for _, key := range reader.ReadOnlyMessageHeader.Keys() {
		err := handler(key, reader.ReadOnlyMessageHeader.Get(key))
		if err != nil {
			return err
		}
	}
	return nil
}

var _ opentracing.TextMapReader = &messageHeaderReader{}

type messageEnvelopeWriter struct {
	MessageEnvelope *actor.MessageEnvelope
}

func (writer *messageEnvelopeWriter) Set(key, val string) {
	writer.MessageEnvelope.SetHeader(key, val)
}

var _ opentracing.TextMapWriter = &messageEnvelopeWriter{}
