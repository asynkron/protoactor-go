package remoting

import (
	"log"

	"github.com/rogeralsing/gam/actor"
)

var endpointManagerPID *actor.PID

func newEndpointManager() actor.Actor {
	return &endpointManager{}
}

type endpointManager struct {
	connections map[string]*actor.PID
}

func (state *endpointManager) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case actor.Started:
		state.connections = make(map[string]*actor.PID)
		log.Println("Started EndpointManager")
	case *MessageEnvelope:
		pid, ok := state.connections[msg.Target.Host]
		if !ok {
			pid = actor.Spawn(actor.Props(newEndpointWriter(msg.Target.Host)).WithMailbox(actor.NewUnboundedBatchingMailbox(1000)))
			state.connections[msg.Target.Host] = pid
		}
		pid.Tell(msg)
	}
}
