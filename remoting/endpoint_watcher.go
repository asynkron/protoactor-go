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
	host string
}

func (state *endpointWatcher) initialize() {
	log.Printf("[REMOTING] Started EndpointWatcher for host %v", state.host)
}
func (state *endpointWatcher) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		state.initialize()
	case *remoteWatch:
	case *remoteUnwatch:
	default:
		log.Fatal("Unknown message", msg)
	}
}
