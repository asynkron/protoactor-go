package opentracing

import (
	"sync"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/opentracing/opentracing-go"
)

var parentSpans = sync.Map{}

func getAndClearParentSpan(pid *actor.PID) opentracing.Span {
	value, ok := parentSpans.Load(pid)
	if !ok {
		return nil
	}

	parentSpans.Delete(pid)

	span, _ := value.(opentracing.Span)

	return span
}

func setParentSpan(pid *actor.PID, span opentracing.Span) {
	parentSpans.Store(pid, span)
}
