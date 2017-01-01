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

type endpointManager struct {
	connections map[string]*actor.PID
	config      *remotingConfig
}

func (state *endpointManager) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		state.connections = make(map[string]*actor.PID)
		log.Println("[REMOTING] Started EndpointManager")
	case *MessageEnvelope:
		pid, ok := state.connections[msg.Target.Host]
		if !ok {
			props := actor.
				FromProducer(newEndpointWriter(msg.Target.Host, state.config)).
				WithMailbox(newEndpointWriterMailbox(state.config.endpointWriterBatchSize, state.config.endpointWriterQueueSize))
			pid = ctx.Spawn(props)
			state.connections[msg.Target.Host] = pid
		}
		pid.Tell(msg)
	}
}
