package remoting

import (
	"log"
	"net"

	"github.com/rogeralsing/gam/actor"
	"google.golang.org/grpc"
)

type server struct{}

func (s *server) Receive(stream Remoting_ReceiveServer) error {
	for {
		batch, err := stream.Recv()
		if err != nil {
			return err
		}
		for _, envelope := range batch.Envelopes {
			pid := envelope.Target
			message := UnpackMessage(envelope)
			pid.Tell(message)
		}
	}
}

func StartServer(host string) {

	lis, err := net.Listen("tcp", host)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	host = lis.Addr().String()
	log.Printf("Host is %v", host)
	actor.ProcessRegistry.RegisterHostResolver(remoteHandler)
	actor.ProcessRegistry.Host = host
	props := actor.
		FromProducer(newEndpointManager).
		WithMailbox(actor.NewBoundedMailbox(1000, 100000))

	endpointManagerPID = actor.Spawn(props)

	s := grpc.NewServer()
	RegisterRemotingServer(s, &server{})
	log.Printf("Starting GAM server on %v.", host)
	go s.Serve(lis)
}
