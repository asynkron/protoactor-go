// Package opentelemetry provides OpenTelemetry tracing middleware for actors.
package opentelemetry

import (
	"context"
	"fmt"
	"sync"

	"github.com/asynkron/protoactor-go/actor"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

var activeSpan = sync.Map{}

// getActiveSpan retrieves the span associated with the actor PID, if any.
func getActiveSpan(pid *actor.PID) trace.Span {
	value, ok := activeSpan.Load(pid)
	if !ok {
		return nil
	}

	span, _ := value.(trace.Span)

	return span
}

func clearActiveSpan(pid *actor.PID) {
	activeSpan.Delete(pid)
}

func setActiveSpan(pid *actor.PID, span trace.Span) {
	activeSpan.Store(pid, span)
}

// GetActiveSpan exposes the active span for a given actor.Context.
// If no span is present a new span is started to avoid returning nil.
func GetActiveSpan(ctx actor.Context) trace.Span {
	span := getActiveSpan(ctx.Self())
	if span == nil {
		// TODO: Fix finding the real span always or handle no-span better on receiving side
		bctx, newSpan := otel.Tracer("protoactor/middleware").Start(context.Background(), fmt.Sprintf("%T/%s", ctx.Actor(), actor.MessageType(ctx.Message())))
		_ = bctx
		span = newSpan
	}

	return span
}
