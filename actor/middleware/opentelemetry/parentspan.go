package opentelemetry

import (
	"sync"

	"github.com/asynkron/protoactor-go/actor"
	"go.opentelemetry.io/otel/trace"
)

var parentSpans = sync.Map{}

func getAndClearParentSpan(pid *actor.PID) trace.Span {
	value, ok := parentSpans.Load(pid)
	if !ok {
		return nil
	}

	parentSpans.Delete(pid)

	span, _ := value.(trace.Span)

	return span
}

func setParentSpan(pid *actor.PID, span trace.Span) {
	parentSpans.Store(pid, span)
}
