package opentracing

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/opentracing/opentracing-go"
	"sync"
)

var parentSpans = sync.Map{}

func getAndClearParentSpan(pid *actor.PID) opentracing.Span {
	value, ok := parentSpans.Load(pid)
	if !ok {
		return nil
	}
	parentSpans.Delete(pid)
	return value.(opentracing.Span)
}

func setParentSpan(pid *actor.PID, span opentracing.Span) {
	parentSpans.Store(pid, span)
}
