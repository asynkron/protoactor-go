package persistence

import (
	"log"

	"github.com/AsynkronIT/gam/actor"
	proto "github.com/golang/protobuf/proto"
)

func Using(provider Provider) actor.Receive {
	started := false
	eventIndex := 0
	snapshotInterval := provider.GetSnapshotInterval()
	return func(context actor.Context) {
		name := context.Self().Id
		switch msg := context.Message().(type) {
		case *actor.Started:
			context.Next()
			context.Self().Tell(&Replay{})
		case *Replay:
			started = false
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

			started = true //persistence is now started
			context.Receive(&ReplayComplete{})
		case *actor.Stopped:
			log.Printf("Stopped\n")
			context.Next()
		case proto.Message:
			if started {
				if _, ok := context.Message().(PersistentEvent); ok {
					provider.PersistEvent(name, eventIndex, msg)
					eventIndex++
					if snapshotInterval != 0 && eventIndex%snapshotInterval == 0 {
						persistSnapshot := provider.GetPersistSnapshot(name)
						context.Receive(RequestSnapshot{PersistSnapshot: persistSnapshot})
					}
				}
			}
			context.Next()
		default:
			context.Next()
		}
	}
}
