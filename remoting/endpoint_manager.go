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

func subscribeEndpointManager() {
	actor.EventStream.SubscribePID(endpointManagerPID,
		func(m interface{}) bool {
			_, ok := m.(*EndpointTerminated)
			return ok
		})
}

func spawnEndpointManager(config *remotingConfig) {
	props := actor.
		FromProducer(newEndpointManager(config)).
		WithMailbox(actor.NewBoundedMailbox(config.endpointManagerQueueSize))

	endpointManagerPID = actor.Spawn(props)
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
	case *EndpointTerminated:
		address := msg.Address
		endpoint := state.ensureConnected(address, ctx)
		endpoint.watcher.Tell(msg)
	case *remoteTerminate:
		address := msg.Watchee.Address
		endpoint := state.ensureConnected(address, ctx)
		endpoint.watcher.Tell(msg)
	case *remoteWatch:
		address := msg.Watchee.Address
		endpoint := state.ensureConnected(address, ctx)
		endpoint.watcher.Tell(msg)
	case *remoteUnwatch:
		address := msg.Watchee.Address
		endpoint := state.ensureConnected(address, ctx)
		endpoint.watcher.Tell(msg)
	case *MessageEnvelope:
		address := msg.Target.Address
		endpoint := state.ensureConnected(address, ctx)

		endpoint.writer.Tell(msg)
	}
}
func (state *endpointManager) ensureConnected(address string, ctx actor.Context) *endpoint {
	e, ok := state.connections[address]
	if !ok {
		e = &endpoint{
			writer:  state.spawnEndpointWriter(address, ctx),
			watcher: state.spawnEndpointWatcher(address, ctx),
		}
		state.connections[address] = e
	}
	return e
}

func (state *endpointManager) spawnEndpointWriter(address string, ctx actor.Context) *actor.PID {
	props := actor.
		FromProducer(newEndpointWriter(address, state.config)).
		WithMailbox(newEndpointWriterMailbox(state.config.endpointWriterBatchSize, state.config.endpointWriterQueueSize))
	pid := ctx.Spawn(props)
	return pid
}

func (state *endpointManager) spawnEndpointWatcher(address string, ctx actor.Context) *actor.PID {
	props := actor.
		FromProducer(newEndpointWatcher(address))
	pid := ctx.Spawn(props)
	return pid
}
