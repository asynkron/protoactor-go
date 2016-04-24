package remoting

import (
	"log"
	"net"

	"github.com/rogeralsing/gam"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type server struct{}

var endpoints = make(map[string]*gam.PID) 

func (s *server) Receive(ctx context.Context, in *gam.MessageEnvelope) (*gam.Unit, error) {
	pid := in.Target
	message := UnpackMessage(in)
	pid.Tell(message)
	log.Println("Got message ", message)
	return &gam.Unit{}, nil
}

func remoteHandler(pid *gam.PID) (gam.ActorRef,bool) {
	return nil,false
}

func StartServer(node string, host string) {
	gam.GlobalProcessRegistry.AddRemoteHandler(remoteHandler)
	gam.GlobalProcessRegistry.Node = node
	gam.GlobalProcessRegistry.Host = host
	lis, err := net.Listen("tcp", host)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	gam.RegisterRemotingServer(s, &server{})
	log.Printf("Starting GAM server on %v@%v.", node,host)
	go s.Serve(lis)
}

func Register(name string, pid *gam.PID) {
	endpoints[name] = pid
}

