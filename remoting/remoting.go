package remoting

import (
	"log"
	"net"

	"github.com/golang/protobuf/proto"
	"github.com/rogeralsing/gam/actor"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var _ actor.ActorRef = &RemoteActorRef{}

type server struct{}

func (s *server) Receive(stream Remoting_ReceiveServer) error {
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

func remoteHandler(pid *actor.PID) (actor.ActorRef, bool) {
	ref := NewRemoteActorRef(pid)
	return ref, true
}

var endpointManagerPID *actor.PID

func StartServer(host string) {
	actor.ProcessRegistry.RegisterHostResolver(remoteHandler)
	actor.ProcessRegistry.Host = host

	endpointManagerPID = actor.SpawnTemplate(&EndpointManager{})

	lis, err := net.Listen("tcp", host)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	RegisterRemotingServer(s, &server{})
	log.Printf("Starting GAM server on %v.", host)
	go s.Serve(lis)
}

type RemoteActorRef struct {
	pid *actor.PID
}

func NewRemoteActorRef(pid *actor.PID) actor.ActorRef {
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

func (ref *RemoteActorRef) SendSystemMessage(message actor.SystemMessage) {

}

func (ref *RemoteActorRef) Stop() {

}

func sendMessage(message proto.Message, target *actor.PID) {

}

type EndpointManager struct {
	connections map[string]*actor.PID
}

func (state *EndpointManager) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case actor.Started:
		state.connections = make(map[string]*actor.PID)
		log.Println("Started EndpointManagerActor")
	case *MessageEnvelope:
		pid := state.connections[msg.Target.Host]
		if pid == nil {
			pid = actor.SpawnTemplate(&EndpointWriter{host: msg.Target.Host})
			state.connections[msg.Target.Host] = pid
		}
		pid.Tell(msg)
	}
}

type EndpointWriter struct {
	host   string
	conn   *grpc.ClientConn
	stream Remoting_ReceiveClient
}

func (state *EndpointWriter) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case actor.Started:
		log.Println("Started EndpointSenderActor for host ", state.host)
		conn, err := grpc.Dial(state.host, grpc.WithInsecure())
		state.conn = conn
		if err != nil {
			log.Fatalf("did not connect: %v", err)
		}
		c := NewRemotingClient(conn)
		stream, err := c.Receive(context.Background())
		state.stream = stream
	case actor.Stopped:
		state.conn.Close()
	case *MessageEnvelope:
		state.stream.Send(msg)
	}
}
