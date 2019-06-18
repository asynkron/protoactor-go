package opentracing

import (
	"sync"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/opentracing/opentracing-go"
)

var stoppingSpans = sync.Map{}

func getAndClearStoppingSpan(pid *actor.PID) opentracing.Span {
	value, ok := stoppingSpans.Load(pid)
	if !ok {
		return nil
	}
	stoppingSpans.Delete(pid)
	return value.(opentracing.Span)
}

func getStoppingSpan(pid *actor.PID) opentracing.Span {
	value, ok := stoppingSpans.Load(pid)
	if !ok {
		return nil
	}
	return value.(opentracing.Span)
}

func setStoppingSpan(pid *actor.PID, span opentracing.Span) {
	stoppingSpans.Store(pid, span)
}
