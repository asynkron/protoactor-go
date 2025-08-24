package opentracing

import (
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/actor/middleware/propagator"
)

// TracingMiddleware sets up spawn, sender, and receiver middlewares that propagate tracing spans.
func TracingMiddleware() actor.SpawnMiddleware {
	return propagator.New().
		WithItselfForwarded().
		WithSpawnMiddleware(SpawnMiddleware()).
		WithSenderMiddleware(SenderMiddleware()).
		WithReceiverMiddleware(ReceiverMiddleware()).
		SpawnMiddleware
}
