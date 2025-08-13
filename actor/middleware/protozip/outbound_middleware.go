package protozip

import (
	"github.com/asynkron/protoactor-go/actor"
)

// ZipkinTracer is an outbound middleware that copies Zipkin tracing headers
// from the sender context into the outgoing message envelope.
func ZipkinTracer(next actor.SenderFunc) actor.SenderFunc {
	return func(ctx actor.SenderContext, target *actor.PID, envelope *actor.MessageEnvelope) {
		header := ctx.MessageHeader()

		envelope.SetHeader("trace-id", header.Get("trace-id"))
		envelope.SetHeader("span-id", header.Get("child-id"))
		envelope.SetHeader("child-id", "123random")

		next(ctx, target, envelope)
	}
}
