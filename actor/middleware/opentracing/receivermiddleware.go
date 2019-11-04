package opentracing

import (
	"fmt"

	"github.com/otherview/protoactor-go/actor"
	"github.com/otherview/protoactor-go/log"
	"github.com/opentracing/opentracing-go"
)

func ReceiverMiddleware() actor.ReceiverMiddleware {
	return func(next actor.ReceiverFunc) actor.ReceiverFunc {
		return func(c actor.ReceiverContext, envelope *actor.MessageEnvelope) {
			spanContext, err := opentracing.GlobalTracer().Extract(opentracing.TextMap, opentracing.TextMapReader(&messageHeaderReader{ReadOnlyMessageHeader: envelope.Header}))
			if err == opentracing.ErrSpanContextNotFound {
				logger.Debug("INBOUND No spanContext found", log.Stringer("PID", c.Self()), log.Error(err))
				//next(c)
			} else if err != nil {
				logger.Debug("INBOUND Error", log.Stringer("PID", c.Self()), log.Error(err))
				next(c, envelope)
				return
			}
			var span opentracing.Span
			switch envelope.Message.(type) {
			case *actor.Started:
				parentSpan := getAndClearParentSpan(c.Self())
				if parentSpan != nil {
					span = opentracing.StartSpan(fmt.Sprintf("%T/%T", c.Actor(), envelope.Message), opentracing.ChildOf(parentSpan.Context()))
					logger.Debug("INBOUND Found parent span", log.Stringer("PID", c.Self()), log.TypeOf("ActorType", c.Actor()), log.TypeOf("MessageType", envelope.Message))
				} else {
					logger.Debug("INBOUND No parent span", log.Stringer("PID", c.Self()), log.TypeOf("ActorType", c.Actor()), log.TypeOf("MessageType", envelope.Message))
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
				logger.Debug("INBOUND No spanContext. Starting new span", log.Stringer("PID", c.Self()), log.TypeOf("ActorType", c.Actor()), log.TypeOf("MessageType", envelope.Message))
				span = opentracing.StartSpan(fmt.Sprintf("%T/%T", c.Actor(), envelope.Message))
			}
			if span == nil {
				logger.Debug("INBOUND Starting span from parent", log.Stringer("PID", c.Self()), log.TypeOf("ActorType", c.Actor()), log.TypeOf("MessageType", envelope.Message))
				span = opentracing.StartSpan(fmt.Sprintf("%T/%T", c.Actor(), envelope.Message), opentracing.ChildOf(spanContext))
			}

			setActiveSpan(c.Self(), span)
			span.SetTag("ActorPID", c.Self())
			span.SetTag("ActorType", fmt.Sprintf("%T", c.Actor()))
			span.SetTag("MessageType", fmt.Sprintf("%T", envelope.Message))

			defer func() {
				logger.Debug("INBOUND Finishing span", log.Stringer("PID", c.Self()), log.TypeOf("ActorType", c.Actor()), log.TypeOf("MessageType", envelope.Message))
				span.Finish()
				clearActiveSpan(c.Self())
			}()

			next(c, envelope)
		}
	}
}
