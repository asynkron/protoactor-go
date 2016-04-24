package remoting

import (
	"log"
	"net"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"github.com/rogeralsing/gam"
)

type server struct{}

func (s *server) Receive(ctx context.Context, in *gam.MessageEnvelope) (*gam.Unit, error) {
	pid := in.Target
	message := UnpackMessage(in)
	pid.Tell(message)
	log.Println("Got message ", message)
	return &gam.Unit{}, nil
}

func StartServer(node string, address string) {
	gam.GlobalProcessRegistry.Node = node
	gam.GlobalProcessRegistry.Host = address
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	gam.RegisterRemotingServer(s, &server{})
	go s.Serve(lis)
}
