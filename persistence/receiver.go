package persistence

import (
	"log"
	"reflect"

	"github.com/AsynkronIT/gam/actor"
	proto "github.com/golang/protobuf/proto"
)

func Using(provider Provider) actor.Receive {
	eventIndex := 0
	//snapshotInterval := provider.GetSnapshotInterval()
	return func(context actor.Context) {
		name := context.Self().Id
		switch context.Message().(type) {
		case *actor.Started:
			context.Next()
			context.Self().Tell(&Replay{})
		case *Replay:
			if p, ok := context.Actor().(persistent); ok {
				p.init(func(msg proto.Message) {
					provider.PersistEvent(name, eventIndex, msg)
					eventIndex++
					context.Receive(msg)
				})
			} else {
				log.Fatalf("Actor type %v is not persistent", reflect.TypeOf(context.Actor()))
			}
			eventIndex = 0

			context.Next()
			snapshot, ok := provider.GetSnapshot(name)
			if ok {
				//synchronously receive snapshot
				context.Receive(OfferSnapshot{Snapshot: snapshot})
			}
			provider.GetEvents(name, func(e interface{}) {
				context.Receive(e)
				eventIndex++
			})

			context.Receive(&ReplayComplete{})
		default:
			context.Next()
		}
	}
}
