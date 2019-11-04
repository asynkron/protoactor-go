package opentracing

import (
	"fmt"
	"sync"

	"github.com/otherview/protoactor-go/actor"
	"github.com/opentracing/opentracing-go"
)

var activeSpan = sync.Map{}

func getActiveSpan(pid *actor.PID) opentracing.Span {
	value, ok := activeSpan.Load(pid)
	if !ok {
		return nil
	}
	return value.(opentracing.Span)
}

func clearActiveSpan(pid *actor.PID) {
	activeSpan.Delete(pid)
}

func setActiveSpan(pid *actor.PID, span opentracing.Span) {
	activeSpan.Store(pid, span)
}

func GetActiveSpan(context actor.Context) opentracing.Span {
	span := getActiveSpan(context.Self())
	if span == nil {
		// TODO: Fix finding the real span always or handle no-span better on receiving side
		span = opentracing.StartSpan(fmt.Sprintf("%T/%T", context.Actor(), context.Message()))
	}
	return span
}
