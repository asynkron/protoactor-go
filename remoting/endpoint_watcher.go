package remoting

import (
	"log"

	"github.com/AsynkronIT/protoactor-go/actor"
)

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
	watched map[string]*actor.PID
}

func (state *endpointWatcher) initialize() {
	log.Printf("[REMOTING] Started EndpointWatcher for host %v", state.host)
	state.watched = make(map[string]*actor.PID)
}

func (state *endpointWatcher) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		state.initialize()
	case *remoteWatch:
		w := &actor.Watch{
			Watcher: msg.Watcher,
		}
		sendRemoteMessage(msg.Watchee, w, nil)
	case *remoteUnwatch:
		uw := &actor.Unwatch{
			Watcher: msg.Watcher,
		}
		sendRemoteMessage(msg.Watchee, uw, nil)
	default:
		log.Printf("[REMOTING] EndpointWatcher for %v, Unknown message %v", state.host, msg)
	}
}
