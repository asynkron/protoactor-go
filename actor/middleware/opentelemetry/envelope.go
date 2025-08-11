package opentelemetry

import (
	"github.com/asynkron/protoactor-go/actor"
	"go.opentelemetry.io/otel/propagation"
)

type messageEnvelopeCarrier struct {
	envelope *actor.MessageEnvelope
}

func (c messageEnvelopeCarrier) Get(key string) string {
	if c.envelope == nil || c.envelope.Header == nil {
		return ""
	}
	return c.envelope.Header.Get(key)
}

func (c messageEnvelopeCarrier) Set(key, val string) {
	if c.envelope != nil {
		c.envelope.SetHeader(key, val)
	}
}

func (c messageEnvelopeCarrier) Keys() []string {
	if c.envelope == nil || c.envelope.Header == nil {
		return nil
	}
	return c.envelope.Header.Keys()
}

var _ propagation.TextMapCarrier = (*messageEnvelopeCarrier)(nil)
