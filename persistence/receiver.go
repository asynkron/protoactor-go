package persistence

import (
	"log"
	"reflect"

	"github.com/asynkron/protoactor-go/actor"
)

func Using(provider Provider) func(next actor.ReceiverFunc) actor.ReceiverFunc {
	return func(next actor.ReceiverFunc) actor.ReceiverFunc {
		fn := func(ctx actor.ReceiverContext, env *actor.MessageEnvelope) {
			switch env.Message.(type) {

			// intercept the started event, handle it and then apply the persistence init logic
			case *actor.Started:
				next(ctx, env)

				// check if the actor is persistent
				if p, ok := ctx.Actor().(persistent); ok {
					// initialize it
					p.init(provider, ctx.(actor.Context))
				} else {
					// not an persistent actor, bail out
					log.Fatalf("Actor type %v is not persistent", reflect.TypeOf(ctx.Actor()))
				}
			default:
				next(ctx, env)
			}
		}
		return fn
	}
}
