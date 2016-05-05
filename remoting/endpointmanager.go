package remoting

import (
	"log"

	"github.com/rogeralsing/gam/actor"
)

var endpointManagerPID *actor.PID

func newEndpointManager(config *RemotingConfig) actor.ActorProducer {
	return func() actor.Actor {
		return &endpointManager{
			config: config,
		}
	}
}

type endpointManager struct {
	connections map[string]*actor.PID
	config      *RemotingConfig
}

func (state *endpointManager) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case actor.Started:
		state.connections = make(map[string]*actor.PID)
		log.Println("Started EndpointManager")
	case *MessageEnvelope:
		pid, ok := state.connections[msg.Target.Host]
		if !ok {
			props := actor.
				FromProducer(newEndpointWriter(msg.Target.Host, state.config)).
				WithMailbox(actor.NewUnboundedBatchingMailbox(state.config.batchSize))
			pid = actor.Spawn(props)
			state.connections[msg.Target.Host] = pid
		}
		pid.Tell(msg)
	}
}
