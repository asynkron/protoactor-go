package remoting

import (
	"log"

	"github.com/AsynkronIT/protoactor-go/actor"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func newEndpointWriter(host string, config *remotingConfig) actor.Producer {
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
	stream Remoting_ReceiveClient
}

func (state *endpointWriter) initialize() {
	err := state.initializeInternal()
	if err != nil {
		log.Printf("[REMOTING] EndpointWriter failed to connect to %v, err: %v", state.host, err)
	}
}

func (state *endpointWriter) initializeInternal() error {
	log.Println("[REMOTING] Started EndpointWriter for host", state.host)
	log.Println("[REMOTING] Connecting to host", state.host)
	conn, err := grpc.Dial(state.host, state.config.dialOptions...)
	if err != nil {
		return err
	}
	log.Println("[REMOTING] Connected to host", state.host)
	state.conn = conn
	c := NewRemotingClient(conn)
	log.Println("[REMOTING] Getting stream from host", state.host)
	stream, err := c.Receive(context.Background(), state.config.callOptions...)
	if err != nil {
		return err
	}
	log.Println("[REMOTING] Got stream from host", state.host)
	state.stream = stream
	return nil
}

func (state *endpointWriter) sendEnvelopes(msg []interface{}, ctx actor.Context) {
	envelopes := make([]*MessageEnvelope, len(msg))

	for i, tmp := range msg {
		envelopes[i] = tmp.(*MessageEnvelope)
	}

	batch := &MessageBatch{
		Envelopes: envelopes,
	}
	err := state.stream.Send(batch)
	if err != nil {
		ctx.Stash()
		log.Println("[REMOTING] gRPC Failed to send to host", state.host)
		panic("restart")
		//log.Printf("[REMOTING] Endpoing writer %v failed to send, shutting down", ctx.Self())
		//ctx.Self().Stop()
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
