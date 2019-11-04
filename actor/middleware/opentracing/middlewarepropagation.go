package opentracing

import (
	"github.com/otherview/protoactor-go/actor"
	"github.com/otherview/protoactor-go/actor/middleware/propagator"
)

func TracingMiddleware() actor.SpawnMiddleware {
	return propagator.New().
		WithItselfForwarded().
		WithSpawnMiddleware(SpawnMiddleware()).
		WithSenderMiddleware(SenderMiddleware()).
		WithReceiverMiddleware(ReceiverMiddleware()).
		SpawnMiddleware
}
