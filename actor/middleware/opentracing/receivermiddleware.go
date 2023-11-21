package opentracing

import (
	"fmt"
	"log/slog"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/opentracing/opentracing-go"
)

func ReceiverMiddleware() actor.ReceiverMiddleware {
	return func(next actor.ReceiverFunc) actor.ReceiverFunc {
		return func(c actor.ReceiverContext, envelope *actor.MessageEnvelope) {
			spanContext, err := opentracing.GlobalTracer().Extract(opentracing.TextMap, opentracing.TextMapReader(&messageHeaderReader{ReadOnlyMessageHeader: envelope.Header}))
			if err == opentracing.ErrSpanContextNotFound {
				c.Logger().Debug("INBOUND No spanContext found", slog.Any("self", c.Self()), slog.Any("error", err))
				// next(c)
			} else if err != nil {
				c.Logger().Debug("INBOUND Error", slog.Any("self", c.Self()), slog.Any("error", err))
				next(c, envelope)
				return
			}
			var span opentracing.Span
			switch envelope.Message.(type) {
			case *actor.Started:
				parentSpan := getAndClearParentSpan(c.Self())
				if parentSpan != nil {
					span = opentracing.StartSpan(fmt.Sprintf("%T/%T", c.Actor(), envelope.Message), opentracing.ChildOf(parentSpan.Context()))
					c.Logger().Debug("INBOUND Found parent span", slog.Any("self", c.Self()), slog.Any("actor", c.Actor()), slog.Any("message", envelope.Message))
				} else {
					c.Logger().Debug("INBOUND No parent span", slog.Any("self", c.Self()), slog.Any("actor", c.Actor()), slog.Any("message", envelope.Message))
				}
			case *actor.Stopping:
				var parentSpan opentracing.Span
				if c.Parent() != nil {
					parentSpan = getStoppingSpan(c.Parent())
				}
				if parentSpan != nil {
					span = opentracing.StartSpan(fmt.Sprintf("%T/stopping", c.Actor()), opentracing.ChildOf(parentSpan.Context()))
				} else {
					span = opentracing.StartSpan(fmt.Sprintf("%T/stopping", c.Actor()))
				}
				setStoppingSpan(c.Self(), span)
				span.SetTag("ActorPID", c.Self())
				span.SetTag("ActorType", fmt.Sprintf("%T", c.Actor()))
				span.SetTag("MessageType", fmt.Sprintf("%T", envelope.Message))
				stoppingHandlingSpan := opentracing.StartSpan("stopping-handling", opentracing.ChildOf(span.Context()))
				next(c, envelope)
				stoppingHandlingSpan.Finish()
				return
			case *actor.Stopped:
				span = getAndClearStoppingSpan(c.Self())
				next(c, envelope)
				if span != nil {
					span.Finish()
				}
				return
			}
			if span == nil && spanContext == nil {
				c.Logger().Debug("INBOUND No spanContext. Starting new span", slog.Any("self", c.Self()), slog.Any("actor", c.Actor()), slog.Any("message", envelope.Message))
				span = opentracing.StartSpan(fmt.Sprintf("%T/%T", c.Actor(), envelope.Message))
			}
			if span == nil {
				c.Logger().Debug("INBOUND Starting span from parent", slog.Any("self", c.Self()), slog.Any("actor", c.Actor()), slog.Any("message", envelope.Message))
				span = opentracing.StartSpan(fmt.Sprintf("%T/%T", c.Actor(), envelope.Message), opentracing.ChildOf(spanContext))
			}

			setActiveSpan(c.Self(), span)
			span.SetTag("ActorPID", c.Self())
			span.SetTag("ActorType", fmt.Sprintf("%T", c.Actor()))
			span.SetTag("MessageType", fmt.Sprintf("%T", envelope.Message))

			defer func() {
				c.Logger().Debug("INBOUND Finishing span", slog.Any("self", c.Self()), slog.Any("actor", c.Actor()), slog.Any("message", envelope.Message))
				span.Finish()
				clearActiveSpan(c.Self())
			}()

			next(c, envelope)
		}
	}
}
