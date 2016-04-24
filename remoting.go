package gam

import "fmt"

import "golang.org/x/net/context"

type server struct{}

func (s *server) Receive(ctx context.Context, in *MessageEnvelope) (*Unit, error) {
	pid := in.Target
	message := UnpackMessage(in)
	pid.Tell(message)
	fmt.Println("got request!!")
	return &Unit{}, nil
}
