package remoting

import (
	"log"
	"net"

	"github.com/golang/protobuf/proto"
	"github.com/rogeralsing/gam"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var _ gam.ActorRef = &RemoteActorRef{}

type server struct{}

func (s *server) Receive(stream gam.Remoting_ReceiveServer) error {
	for {
		envelope, err := stream.Recv()
		if err != nil {
			return err
		}
		pid := envelope.Target
		message := UnpackMessage(envelope)
		pid.Tell(message)
	}
}

func remoteHandler(pid *gam.PID) (gam.ActorRef, bool) {
	ref := NewRemoteActorRef(pid)
	return ref, true
}

var endpointManagerPID *gam.PID

func StartServer(host string) {
	gam.GlobalProcessRegistry.AddRemoteHandler(remoteHandler)
	gam.GlobalProcessRegistry.Host = host

	endpointManagerPID = gam.SpawnTemplate(&EndpointManagerActor{})

	lis, err := net.Listen("tcp", host)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	gam.RegisterRemotingServer(s, &server{})
	log.Printf("Starting GAM server on %v.", host)
	go s.Serve(lis)
}

type RemoteActorRef struct {
	pid *gam.PID
}

func NewRemoteActorRef(pid *gam.PID) gam.ActorRef {
	return &RemoteActorRef{
		pid: pid,
	}
}

func (ref *RemoteActorRef) Tell(message interface{}) {
	switch msg := message.(type) {
	case proto.Message:
		envelope, _ := PackMessage(msg, ref.pid)
		endpointManagerPID.Tell(envelope)
	default:
		log.Printf("failed, trying to send non Proto %v message to %v", msg, ref.pid)
	}
}

func (ref *RemoteActorRef) SendSystemMessage(message gam.SystemMessage) {

}

func (ref *RemoteActorRef) Stop() {

}

func sendMessage(message proto.Message, target *gam.PID) {

}

type EndpointManagerActor struct {
	connections map[string]*gam.PID
}

func (state *EndpointManagerActor) Receive(ctx gam.Context) {
	switch msg := ctx.Message().(type) {
	case gam.Started:
		state.connections = make(map[string]*gam.PID)
		log.Println("Started EndpointManagerActor")
	case *gam.MessageEnvelope:
		pid := state.connections[msg.Target.Host]
		if pid == nil {
			pid = gam.SpawnTemplate(&EndpointSenderActor{host: msg.Target.Host})
			state.connections[msg.Target.Host] = pid
		}
		pid.Tell(msg)
	}
}

type EndpointSenderActor struct {
	host   string
	conn   *grpc.ClientConn
	stream gam.Remoting_ReceiveClient
}

func (state *EndpointSenderActor) Receive(ctx gam.Context) {
	switch msg := ctx.Message().(type) {
	case gam.Started:
		log.Println("Started EndpointSenderActor for host ", state.host)
		conn, err := grpc.Dial(state.host, grpc.WithInsecure())
		state.conn = conn
		if err != nil {
			log.Fatalf("did not connect: %v", err)
		}
		c := gam.NewRemotingClient(conn)
		stream, err := c.Receive(context.Background())
		state.stream = stream
	case gam.Stopped:
		state.conn.Close()
	case *gam.MessageEnvelope:
		state.stream.Send(msg)
	}
}
