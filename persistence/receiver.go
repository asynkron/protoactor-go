package persistence

import (
	"log"
	"reflect"

	"github.com/AsynkronIT/protoactor-go/actor"
)

func Using(provider Provider) func(next actor.ReceiverFunc) actor.ReceiverFunc {
	return func(next actor.ReceiverFunc) actor.ReceiverFunc {
		fn := func(ctx actor.ReceiverContext, env *actor.MessageEnvelope) {
			switch env.Message.(type) {
			case *actor.Started:
				next(ctx, env)
				if p, ok := ctx.Actor().(persistent); ok {
					p.init(provider, ctx.(actor.Context))
				} else {
					log.Fatalf("Actor type %v is not persistent", reflect.TypeOf(ctx.Actor()))
				}
			default:
				next(ctx, env)
			}
		}
		return fn
	}
}
