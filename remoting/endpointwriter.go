package remoting

import (
	"log"

	"github.com/rogeralsing/gam/actor"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func newEndpointWriter(host string, config *RemotingConfig) actor.ActorProducer {
	return func() actor.Actor {
		return &endpointWriter{
			host:   host,
			config: config,
		}
	}
}

type endpointWriter struct {
	config *RemotingConfig
	host   string
	conn   *grpc.ClientConn
	stream Remoting_ReceiveClient
}

func (state *endpointWriter) initialize() {
	log.Println("Started EndpointWriter for host", state.host)
	log.Println("Connecting to host", state.host)
	conn, err := grpc.Dial(state.host, state.config.DialOptions...)

	if err != nil {
		log.Fatalf("Failed to connect to host %v: %v", state.host, err)
	}
	log.Println("Connected to host", state.host)
	state.conn = conn
	c := NewRemotingClient(conn)
	log.Println("Getting stream from host", state.host)
	stream, err := c.Receive(context.Background(), state.config.CallOptions...)
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
