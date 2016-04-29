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

func remoteHandler(pid *actor.PID) (actor.ActorRef, bool) {
	ref := newRemoteActorRef(pid)
	return ref, true
}

func newEndpointManager() actor.Actor {
	return &endpointManager{}
}

func newEndpointWriter(host string) actor.ActorProducer {
	return func() actor.Actor {
		return &endpointWriter{host: host}
	}
}

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

type RemoteActorRef struct {
	pid *actor.PID
}

func newRemoteActorRef(pid *actor.PID) actor.ActorRef {
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

func (ref *RemoteActorRef) SendSystemMessage(message actor.SystemMessage) {}

func (ref *RemoteActorRef) Stop() {}

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

type endpointWriter struct {
	host   string
	conn   *grpc.ClientConn
	stream Remoting_ReceiveClient
}

func (state *endpointWriter) initialize() {
	log.Println("Started EndpointWriter for host", state.host)
	log.Println("Connecting to host", state.host)
	conn, err := grpc.Dial(state.host, grpc.WithInsecure())

	if err != nil {
		log.Fatalf("Failed to connect to host %v: %v", state.host, err)
	}
	log.Println("Connected to host", state.host)
	state.conn = conn
	c := NewRemotingClient(conn)
	log.Println("Getting stream from host", state.host)
	stream, err := c.Receive(context.Background())
	if err != nil {
		log.Fatalf("Failed to get stream from host %v: %v", state.host, err)
	}
	log.Println("Got stream from host", state.host)
	state.stream = stream
}

func (state *endpointWriter) sendEnvelopes(messages []interface{}, ctx actor.Context) {
	envelopes := make([]*MessageEnvelope, len(messages))

	for i, tmp := range messages {
		envelopes[i] = tmp.(*MessageEnvelope)
	}

	batch := &MessageBatch{
		Envelopes: envelopes,
	}

	err := state.stream.Send(batch)
	if err != nil {
		ctx.Stash()
		log.Println("Failed to send to host", state.host)
		panic("restart")
	}
}

func (state *endpointWriter) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case actor.Started:
		state.initialize()
	case actor.Stopped:
		state.conn.Close()
	case actor.Restarting:
		state.conn.Close()
	case []interface{}:
		state.sendEnvelopes(msg, ctx)
	default:
		log.Fatal("Unknown message", msg)
	}
}
