package remote

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/eventstream"
	"github.com/AsynkronIT/protoactor-go/mailbox"
)

var (
	endpointManagerPID *actor.PID
	endpointSub        *eventstream.Subscription
)

func newEndpointManager(config *remoteConfig) actor.Producer {
	return func() actor.Actor {
		return &endpointManager{
			config: config,
		}
	}
}

func subscribeEndpointManager() {
	endpointSub = eventstream.
		Subscribe(endpointManagerPID.Tell).
		WithPredicate(func(m interface{}) bool {
			switch m.(type) {
			case *EndpointTerminatedEvent, *EndpointConnectedEvent:
				return true
			}
			return false
		})
}

func unsubEndpointManager() {
	eventstream.Unsubscribe(endpointSub)
}

func spawnEndpointManager(config *remoteConfig) {
	props := actor.
		FromProducer(newEndpointManager(config)).
		WithMailbox(mailbox.Bounded(config.endpointManagerQueueSize)).
		WithSupervisor(actor.RestartingSupervisorStrategy())

	endpointManagerPID = actor.Spawn(props)
}

func stopEndpointManager() {
	endpointManagerPID.Tell(&StopEndpointManager{})
}

type endpoint struct {
	writer  *actor.PID
	watcher *actor.PID
}

type endpointManager struct {
	connections map[string]*endpoint
	config      *remoteConfig
}

func (state *endpointManager) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		state.connections = make(map[string]*endpoint)
		plog.Debug("Started EndpointManager")
	case *StopEndpointManager:
		for _, edp := range state.connections {
			edp.watcher.GracefulStop()
			edp.writer.GracefulStop()
		}
		state.connections = make(map[string]*endpoint)
		ctx.SetBehavior(state.Terminated)
		plog.Debug("Stopped EndpointManager")
	case *EndpointTerminatedEvent:
		address := msg.Address
		if connected, endpoint := state.checkConnected(address); connected {
			endpoint.watcher.Tell(msg)
			state.removeEndpoint(address)
		}
	case *EndpointConnectedEvent:
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
	case *remoteDeliver:
		address := msg.target.Address
		endpoint := state.ensureConnected(address, ctx)
		endpoint.writer.Tell(msg)
	}
}

func (state *endpointManager) Terminated(ctx actor.Context) {}

func (state *endpointManager) checkConnected(address string) (bool, *endpoint) {
	e, ok := state.connections[address]
	return ok, e
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

func (state *endpointManager) removeEndpoint(address string) {
	if e, ok := state.connections[address]; ok {
		e.watcher.Stop()
		e.writer.Stop()
		delete(state.connections, address)
	}
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
