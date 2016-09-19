package remoting

import (
	"log"
	"net"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/remoting/messages"
	"google.golang.org/grpc"
)

type server struct{}

func (s *server) Receive(stream messages.Remoting_ReceiveServer) error {
	for {
		batch, err := stream.Recv()
		if err != nil {
			return err
		}
		for _, envelope := range batch.Envelopes {
			pid := envelope.Target
			message := unpackMessage(envelope)
			pid.Tell(message)
		}
	}
}

func Start(host string, options ...RemotingOption) {

	lis, err := net.Listen("tcp", host)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	config := defaultRemoteConfig()
	for _, option := range options {
		option(config)
	}

	host = lis.Addr().String()
	log.Printf("Host is %v", host)
	actor.ProcessRegistry.RegisterHostResolver(remoteHandler)
	actor.ProcessRegistry.Host = host
	props := actor.
		FromProducer(newEndpointManager(config)).
		WithMailbox(actor.NewBoundedMailbox(1000, 100000))

	endpointManagerPID = actor.Spawn(props)

	s := grpc.NewServer(config.serverOptions...)
	messages.RegisterRemotingServer(s, &server{})
	log.Printf("Starting GAM server on %v.", host)
	go s.Serve(lis)
}
