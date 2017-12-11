package protozip

import (
	"github.com/AsynkronIT/protoactor-go/actor"
)

func ZipkinTracer(next actor.SenderFunc) actor.SenderFunc {
	return func(ctx actor.Context, target *actor.PID, envelope *actor.MessageEnvelope) {

		envelope.SetHeader("trace-id", envelope.GetHeader("trace-id"))
		envelope.SetHeader("span-id", envelope.GetHeader("child-id"))
		envelope.SetHeader("child-id", "123random")

		next(ctx, target, envelope)
	}
}
