package remoting

import (
	"log"

	"github.com/rogeralsing/gam/actor"
	"google.golang.org/grpc"
)

var endpointManagerPID *actor.PID

func newEndpointManager(dialOpts []grpc.DialOption, callOpts []grpc.CallOption) actor.ActorProducer {
	return func() actor.Actor {
		return &endpointManager{
			dialOpts: dialOpts,
			callOpts: callOpts,
		}
	}
}

type endpointManager struct {
	connections map[string]*actor.PID
	dialOpts    []grpc.DialOption
	callOpts    []grpc.CallOption
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
				FromProducer(newEndpointWriter(msg.Target.Host)).
				WithMailbox(actor.NewUnboundedBatchingMailbox(1000))
			pid = actor.Spawn(props)
			state.connections[msg.Target.Host] = pid
		}
		pid.Tell(msg)
	}
}
