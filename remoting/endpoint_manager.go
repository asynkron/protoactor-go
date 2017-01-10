package remoting

import (
	"log"

	"github.com/AsynkronIT/protoactor-go/actor"
)

var endpointManagerPID *actor.PID

func newEndpointManager(config *remotingConfig) actor.Producer {
	return func() actor.Actor {
		return &endpointManager{
			config: config,
		}
	}
}

type endpoint struct {
	writer  *actor.PID
	watcher *actor.PID
}

type endpointManager struct {
	connections map[string]*endpoint
	config      *remotingConfig
}

func (state *endpointManager) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		state.connections = make(map[string]*endpoint)

		log.Println("[REMOTING] Started EndpointManager")
	case *remoteWatch:
		host := msg.Watchee.Host
		endpoint := state.ensureConnected(host, ctx)
		endpoint.watcher.Tell(msg)
	case *remoteUnwatch:
		host := msg.Watchee.Host
		endpoint := state.ensureConnected(host, ctx)
		endpoint.watcher.Tell(msg)
	case *MessageEnvelope:
		host := msg.Target.Host
		endpoint := state.ensureConnected(host, ctx)

		if endpoint == nil {
			log.Println("endpoint is nil!!!")
		}

		endpoint.writer.Tell(msg)
	}
}
func (state *endpointManager) ensureConnected(host string, ctx actor.Context) *endpoint {
	e, ok := state.connections[host]
	if !ok {
		props := actor.
			FromProducer(newEndpointWriter(host, state.config)).
			WithMailbox(newEndpointWriterMailbox(state.config.endpointWriterBatchSize, state.config.endpointWriterQueueSize))
		pid := ctx.Spawn(props)
		e = &endpoint{
			writer:  pid,
			watcher: nil, //TODO: spawn watcher
		}
		state.connections[host] = e
	}
	return e
}
