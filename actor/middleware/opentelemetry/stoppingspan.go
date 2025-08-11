package opentelemetry

import (
	"sync"

	"github.com/asynkron/protoactor-go/actor"
	"go.opentelemetry.io/otel/trace"
)

var stoppingSpans = sync.Map{}

func getAndClearStoppingSpan(pid *actor.PID) trace.Span {
	value, ok := stoppingSpans.Load(pid)
	if !ok {
		return nil
	}
	stoppingSpans.Delete(pid)
	return value.(trace.Span)
}

func getStoppingSpan(pid *actor.PID) trace.Span {
	value, ok := stoppingSpans.Load(pid)
	if !ok {
		return nil
	}
	return value.(trace.Span)
}

func setStoppingSpan(pid *actor.PID, span trace.Span) {
	stoppingSpans.Store(pid, span)
}
