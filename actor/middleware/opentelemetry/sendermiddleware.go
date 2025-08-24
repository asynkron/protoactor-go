package opentelemetry

import (
	"context"
	"log/slog"

	"github.com/asynkron/protoactor-go/actor"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// SenderMiddleware injects the active span context into outbound messages.
func SenderMiddleware() actor.SenderMiddleware {
	return func(next actor.SenderFunc) actor.SenderFunc {
		return func(c actor.SenderContext, target *actor.PID, envelope *actor.MessageEnvelope) {
			span := getActiveSpan(c.Self())

			if span == nil {
				c.Logger().Debug("OUTBOUND No active span", slog.Any("self", c.Self()), slog.Any("actor", c.Actor()), slog.Any("message", envelope.Message))
				next(c, target, envelope)
				return
			}

			ctx := trace.ContextWithSpan(context.Background(), span)
			otel.GetTextMapPropagator().Inject(ctx, messageEnvelopeCarrier{envelope})

			c.Logger().Debug("OUTBOUND Successfully injected", slog.Any("self", c.Self()), slog.Any("actor", c.Actor()), slog.Any("message", envelope.Message))
			next(c, target, envelope)
		}
	}
}
