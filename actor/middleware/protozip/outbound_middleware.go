package protozip

import (
	"github.com/AsynkronIT/protoactor-go/actor"
)

func ZipkinTracer(next actor.SenderFunc) actor.SenderFunc {
	return func(ctx actor.Context, target *actor.PID, envelope *actor.MessageEnvelope) {
		header := ctx.MessageHeader()

		envelope.Header.Set("trace-id", header.Get("trace-id"))
		envelope.Header.Set("span-id", header.Get("child-id"))
		envelope.Header.Set("child-id", "123random")

		next(ctx, target, envelope)
	}
}
