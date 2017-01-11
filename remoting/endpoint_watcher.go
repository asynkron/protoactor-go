package remoting

import (
	"log"

	"github.com/AsynkronIT/protoactor-go/actor"
)

type EndpointTerminated struct {
	address string
}

type remoteWatch struct {
	Watcher *actor.PID
	Watchee *actor.PID
}

type remoteUnwatch struct {
	Watcher *actor.PID
	Watchee *actor.PID
}

type remoteTerminate struct {
	Watcher *actor.PID
	Watchee *actor.PID
}

func newEndpointWatcher(address string) actor.Producer {
	return func() actor.Actor {
		return &endpointWatcher{
			address: address,
		}
	}
}

type endpointWatcher struct {
	address string
	watched map[string]*actor.PID //key is the watching PID string, value is the watched PID
	watcher map[string]*actor.PID //key is the watched PID string, value is the watching PID
}

func (state *endpointWatcher) initialize() {
	log.Printf("[REMOTING] Started EndpointWatcher for address %v", state.address)
	state.watched = make(map[string]*actor.PID)
	state.watcher = make(map[string]*actor.PID)
}

func (state *endpointWatcher) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		state.initialize()

	case *remoteTerminate:
		delete(state.watched, msg.Watcher.Id)
		delete(state.watcher, msg.Watchee.Id)

	case *EndpointTerminated:
		//The EndpointWatcher is notified that the given endpoint has closed
		//Now notify all watchers that any watched PID from this endpoint is terminated
		//also make Terminated carry information about AddressTerminated

	case *remoteWatch:

		state.watched[msg.Watcher.Id] = msg.Watchee
		state.watcher[msg.Watchee.Id] = msg.Watcher

		//recreate the Watch command
		w := &actor.Watch{
			Watcher: msg.Watcher,
		}

		//pass it off to the remote PID
		sendRemoteMessage(msg.Watchee, w, nil)

	case *remoteUnwatch:

		//delete the watch entries
		delete(state.watched, msg.Watcher.Id)
		delete(state.watcher, msg.Watchee.Id)

		//recreate the Unwatch command
		uw := &actor.Unwatch{
			Watcher: msg.Watcher,
		}

		//pass it off to the remote PID
		sendRemoteMessage(msg.Watchee, uw, nil)

	default:
		log.Printf("[REMOTING] EndpointWatcher for %v, Unknown message %v", state.address, msg)
	}
}
