package protozip

import (
	"github.com/AsynkronIT/protoactor-go/actor"
)

func ZipkinTracer(next actor.SenderFunc) actor.SenderFunc {
	return func(ctx actor.SenderContext, target *actor.PID, envelope *actor.MessageEnvelope) {
		header := ctx.MessageHeader()

		envelope.SetHeader("trace-id", header.Get("trace-id"))
		envelope.SetHeader("span-id", header.Get("child-id"))
		envelope.SetHeader("child-id", "123random")

		next(ctx, target, envelope)
	}
}
