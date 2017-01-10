package remoting

import (
	"log"

	"github.com/AsynkronIT/protoactor-go/actor"
)

type addressTerminated struct {
	host string
}

type remoteWatch struct {
	Watcher *actor.PID
	Watchee *actor.PID
}

type remoteUnwatch struct {
	Watcher *actor.PID
	Watchee *actor.PID
}

func newEndpointWatcher(host string) actor.Producer {
	return func() actor.Actor {
		return &endpointWatcher{
			host: host,
		}
	}
}

type endpointWatcher struct {
	host    string
	watched map[string]*actor.PID //key is the watching PID string, value is the watched PID
	watcher map[string]*actor.PID //key is the watched PID string, value is the watching PID
}

func (state *endpointWatcher) initialize() {
	log.Printf("[REMOTING] Started EndpointWatcher for host %v", state.host)
	state.watched = make(map[string]*actor.PID)
	state.watcher = make(map[string]*actor.PID)
}

func (state *endpointWatcher) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		state.initialize()

	case *addressTerminated:
		//The EndpointWatcher is notified that the given endpoint has closed
		//Now notify all watchers that the watched PID has Terminated
		//also make Terminated carry information about AddressTerminated

	case *remoteWatch:

		state.watched[msg.Watcher.String()] = msg.Watchee
		state.watcher[msg.Watchee.String()] = msg.Watcher

		//recreate the Watch command
		w := &actor.Watch{
			Watcher: msg.Watcher,
		}

		//pass it off to the remote PID
		sendRemoteMessage(msg.Watchee, w, nil)

	case *remoteUnwatch:

		//delete the watch entries
		delete(state.watched, msg.Watcher.String())
		delete(state.watcher, msg.Watchee.String())

		//recreate the Unwatch command
		uw := &actor.Unwatch{
			Watcher: msg.Watcher,
		}

		//pass it off to the remote PID
		sendRemoteMessage(msg.Watchee, uw, nil)
	default:
		log.Printf("[REMOTING] EndpointWatcher for %v, Unknown message %v", state.host, msg)
	}
}
