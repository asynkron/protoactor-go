package remoting

import (
	"log"

	"github.com/AsynkronIT/gam/actor"
	"github.com/AsynkronIT/gam/remoting/messages"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func newEndpointWriter(host string, config *remotingConfig) actor.ActorProducer {
	return func() actor.Actor {
		return &endpointWriter{
			host:   host,
			config: config,
		}
	}
}

type endpointWriter struct {
	config *remotingConfig
	host   string
	conn   *grpc.ClientConn
	stream messages.Remoting_ReceiveClient
}

func (state *endpointWriter) initialize() {
	log.Println("[REMOTING] Started EndpointWriter for host", state.host)
	log.Println("[REMOTING] Connecting to host", state.host)
	conn, err := grpc.Dial(state.host, state.config.dialOptions...)

	if err != nil {
		log.Fatalf("[REMOTING] Failed to connect to host %v: %v", state.host, err)
	}
	log.Println("[REMOTING] Connected to host", state.host)
	state.conn = conn
	c := messages.NewRemotingClient(conn)
	log.Println("[REMOTING] Getting stream from host", state.host)
	stream, err := c.Receive(context.Background(), state.config.callOptions...)
	if err != nil {
		log.Fatalf("[REMOTING] Failed to get stream from host %v: %v", state.host, err)
	}
	log.Println("[REMOTING] Got stream from host", state.host)
	state.stream = stream
}

func (state *endpointWriter) sendEnvelopes(msg []interface{}, ctx actor.Context) {
	envelopes := make([]*messages.MessageEnvelope, len(msg))

	for i, tmp := range msg {
		m := tmp.(actor.UserMessage)
		envelopes[i] = m.Message.(*messages.MessageEnvelope)
	}

	batch := &messages.MessageBatch{
		Envelopes: envelopes,
	}

	err := state.stream.Send(batch)
	if err != nil {
		ctx.Stash()
		log.Println("[REMOTING] Failed to send to host", state.host)
		panic("restart")
	}
}

func (state *endpointWriter) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		state.initialize()
	case *actor.Stopped:
		state.conn.Close()
	case *actor.Restarting:
		state.conn.Close()
	case []interface{}:
		state.sendEnvelopes(msg, ctx)
	default:
		log.Fatal("Unknown message", msg)
	}
}
