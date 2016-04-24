package remoting

import (
	"log"
	"net"

	"github.com/golang/protobuf/proto"
	"github.com/rogeralsing/gam"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type server struct{}

func (s *server) Receive(ctx context.Context, in *gam.MessageEnvelope) (*gam.Unit, error) {
	pid := in.Target
	message := UnpackMessage(in)
	pid.Tell(message)
	log.Println("Got message ", message)
	return &gam.Unit{}, nil
}

func remoteHandler(pid *gam.PID) (gam.ActorRef, bool) {
	log.Println("Resolving ", pid)
	ref := NewRemoteActorRef(pid)
	return ref,true
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
	log.Printf("Starting GAM server on %v@%v.", node, host)
	go s.Serve(lis)
}


type RemoteActorRef struct {
	pid	*gam.PID
}

func NewRemoteActorRef(pid *gam.PID) gam.ActorRef {
	return &RemoteActorRef {
		pid: pid,
	}
}

func (ref *RemoteActorRef) Tell(message interface{}) {
	switch msg := message.(type) {
		case proto.Message:
			sendMessage(ref.pid.Host,msg,ref.pid)
		default:
			log.Printf("failed, trying to send non Proto %v message to %v",msg,ref.pid)
	}
}

func (ref *RemoteActorRef) SendSystemMessage(message gam.SystemMessage) {
	
}

func (ref *RemoteActorRef) Stop() {
	
}

var _ gam.ActorRef = &RemoteActorRef {}

//TODO: this should be streaming
func sendMessage(address string, message proto.Message, target *gam.PID) {
	envelope, err := PackMessage(message, target)
	if err != nil {
		log.Fatalf("did not pack message: %v", err)
	}
	// Set up a connection to the server.
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := gam.NewRemotingClient(conn)
	_, err = c.Receive(context.Background(), envelope)
	if err != nil {
		log.Fatalf("did not send: %v", err)
	}
}
