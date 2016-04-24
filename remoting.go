package gam

import (
	"log"
	"net"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type server struct{}

func (s *server) Receive(ctx context.Context, in *MessageEnvelope) (*Unit, error) {
	pid := in.Target
	message := UnpackMessage(in)
	pid.Tell(message)
	log.Println("Got message ", message)
	return &Unit{}, nil
}

func StartServer(address string) {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	RegisterRemotingServer(s, &server{})
	s.Serve(lis)
}
