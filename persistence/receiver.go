package persistence

import (
	"log"
	"reflect"

	"github.com/AsynkronIT/protoactor-go/actor"
)

func Using(provider Provider) actor.ReceiveFunc {

	return func(context actor.Context) {
		switch context.Message().(type) {
		case *actor.Started:
			context.Next()
			context.Self().Tell(&Replay{}) //start async replay
		case *Replay:
			if p, ok := context.Actor().(persistent); ok {
				p.init(provider, context)
			} else {
				log.Fatalf("Actor type %v is not persistent", reflect.TypeOf(context.Actor()))
			}
		default:
			context.Next()
		}
	}
}
