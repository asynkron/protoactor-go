package opentracing

import (
	"github.com/asynkron/protoactor-go/actor"
	"github.com/opentracing/opentracing-go"
	"log/slog"
)

func SenderMiddleware() actor.SenderMiddleware {
	return func(next actor.SenderFunc) actor.SenderFunc {
		return func(c actor.SenderContext, target *actor.PID, envelope *actor.MessageEnvelope) {
			span := getActiveSpan(c.Self())

			if span == nil {
				c.Logger().Debug("OUTBOUND No active span", slog.Any("self", c.Self()), slog.Any("actor", c.Actor()), slog.Any("message", envelope.Message))
				next(c, target, envelope)
				return
			}

			err := opentracing.GlobalTracer().Inject(span.Context(), opentracing.TextMap, opentracing.TextMapWriter(&messageEnvelopeWriter{MessageEnvelope: envelope}))
			if err != nil {
				c.Logger().Debug("OUTBOUND Error injecting", slog.Any("self", c.Self()), slog.Any("actor", c.Actor()), slog.Any("message", envelope.Message))
				next(c, target, envelope)
				return
			}

			c.Logger().Debug("OUTBOUND Successfully injected", slog.Any("self", c.Self()), slog.Any("actor", c.Actor()), slog.Any("message", envelope.Message))
			next(c, target, envelope)
		}
	}
}
