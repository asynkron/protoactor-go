package opentelemetry

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/asynkron/protoactor-go/actor"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func ReceiverMiddleware() actor.ReceiverMiddleware {
	return func(next actor.ReceiverFunc) actor.ReceiverFunc {
		return func(c actor.ReceiverContext, envelope *actor.MessageEnvelope) {
			ctx := otel.GetTextMapPropagator().Extract(context.Background(), messageEnvelopeCarrier{envelope})
			sc := trace.SpanContextFromContext(ctx)
			if !sc.IsValid() {
				c.Logger().Debug("INBOUND No spanContext found", slog.Any("self", c.Self()))
			}
			var span trace.Span
			switch envelope.Message.(type) {
			case *actor.Started:
				parentSpan := getAndClearParentSpan(c.Self())
				if parentSpan != nil {
					ctx = trace.ContextWithSpan(ctx, parentSpan)
					c.Logger().Debug("INBOUND Found parent span", slog.Any("self", c.Self()), slog.Any("actor", c.Actor()), slog.Any("message", envelope.Message))
				} else {
					c.Logger().Debug("INBOUND No parent span", slog.Any("self", c.Self()), slog.Any("actor", c.Actor()), slog.Any("message", envelope.Message))
				}
			case *actor.Stopping:
				var parentSpan trace.Span
				if c.Parent() != nil {
					parentSpan = getStoppingSpan(c.Parent())
				}
				parentCtx := ctx
				if parentSpan != nil {
					parentCtx = trace.ContextWithSpan(parentCtx, parentSpan)
				}
				parentCtx, span = otel.Tracer("protoactor/middleware").Start(parentCtx, fmt.Sprintf("%T/stopping", c.Actor()))
				setStoppingSpan(c.Self(), span)
				span.SetAttributes(attribute.String("ActorPID", c.Self().String()), attribute.String("ActorType", fmt.Sprintf("%T", c.Actor())), attribute.String("MessageType", actor.MessageType(envelope.Message)))
				childCtx, stoppingHandlingSpan := otel.Tracer("protoactor/middleware").Start(parentCtx, "stopping-handling")
				_ = childCtx
				next(c, envelope)
				stoppingHandlingSpan.End()
				return
			case *actor.Stopped:
				span = getAndClearStoppingSpan(c.Self())
				next(c, envelope)
				if span != nil {
					span.End()
				}
				return
			}

			if span == nil {
				if !sc.IsValid() {
					c.Logger().Debug("INBOUND No spanContext. Starting new span", slog.Any("self", c.Self()), slog.Any("actor", c.Actor()), slog.Any("message", envelope.Message))
				} else {
					c.Logger().Debug("INBOUND Starting span from parent", slog.Any("self", c.Self()), slog.Any("actor", c.Actor()), slog.Any("message", envelope.Message))
				}
				_, span = otel.Tracer("protoactor/middleware").Start(ctx, fmt.Sprintf("%T/%s", c.Actor(), actor.MessageType(envelope.Message)))
			}

			setActiveSpan(c.Self(), span)
			span.SetAttributes(
				attribute.String("ActorPID", c.Self().String()),
				attribute.String("ActorType", fmt.Sprintf("%T", c.Actor())),
				attribute.String("MessageType", actor.MessageType(envelope.Message)),
			)

			defer func() {
				c.Logger().Debug("INBOUND Finishing span", slog.Any("self", c.Self()), slog.Any("actor", c.Actor()), slog.Any("message", envelope.Message))
				span.End()
				clearActiveSpan(c.Self())
			}()

			next(c, envelope)
		}
	}
}
