package persistence

import (
	"log"
	"reflect"

	"github.com/AsynkronIT/gam/actor"
)

func Using(provider PersistenceProvider) actor.Receive {
	started := false
	eventIndex := 0
	snapshotInterval := provider.GetSnapshotInterval()
	return func(context actor.Context) {
		name := context.Self().Id
		switch msg := context.Message().(type) {
		case actor.Started:
			context.Next()
			context.Self().Tell(Replay{})
		case Replay:
			started = false
			log.Printf("Starting\n")
			eventIndex = 0

			context.Next()
			snapshot, ok := provider.GetSnapshot(name)
			if ok {
				//synchronously receive snapshot
				context.Receive(OfferSnapshot{Snapshot: snapshot})
			}
			messages := provider.GetEvents(name)
			for _, m := range messages {
				//synchronously receive events
				context.Receive(m)
			}
			started = true //persistence is now started
			context.Receive(ReplayComplete{})
		case actor.Stopped:
			log.Printf("Stopped\n")
			context.Next()
		case PersistentMessage:
			if started {
				log.Printf("got persistent message %v %+v\n", reflect.TypeOf(msg), msg)
				eventIndex++
				provider.PersistEvent(name, msg)
				if snapshotInterval != 0 && eventIndex%snapshotInterval == 0 {
					persistSnapshot := provider.GetPersistSnapshot(name)
					context.Receive(RequestSnapshot{PersistSnapshot: persistSnapshot})
				}
			}
			context.Next()
		default:
			context.Next()
		}
	}
}
