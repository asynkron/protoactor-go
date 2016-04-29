package remoting

import (
	"log"
	"net"

	"google.golang.org/grpc"

	"github.com/rogeralsing/gam/actor"
)

var endpointManagerPID *actor.PID

func StartServer(host string) {
	actor.ProcessRegistry.RegisterHostResolver(remoteHandler)
	actor.ProcessRegistry.Host = host

	endpointManagerPID = actor.Spawn(actor.Props(newEndpointManager).WithMailbox(actor.NewUnboundedMailbox(1000)))

	lis, err := net.Listen("tcp", host)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	RegisterRemotingServer(s, &server{})
	log.Printf("Starting GAM server on %v.", host)
	go s.Serve(lis)
}

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
