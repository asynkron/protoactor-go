package remote

import (
	"github.com/AsynkronIT/protoactor-go/actor"

	"github.com/AsynkronIT/protoactor-go/eventstream"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func newEndpointWriter(address string, config *remoteConfig) actor.Producer {
	return func() actor.Actor {
		return &endpointWriter{
			address: address,
			config:  config,
		}
	}
}

type endpointWriter struct {
	config  *remoteConfig
	address string
	conn    *grpc.ClientConn
	stream  Remoting_ReceiveClient
}

func (state *endpointWriter) initialize() {
	err := state.initializeInternal()
	if err != nil {
		logdbg.Printf("EndpointWriter failed to connect to %v, err: %v", state.address, err)
	}
}

func (state *endpointWriter) initializeInternal() error {
	logdbg.Printf("Started EndpointWriter for address %v", state.address)
	logdbg.Printf("EndpointWriter connecting to address %v", state.address)
	conn, err := grpc.Dial(state.address, state.config.dialOptions...)
	if err != nil {
		return err
	}
	//	log.Printf("Connected to address %v", state.address)
	state.conn = conn
	c := NewRemotingClient(conn)
	//	log.Printf("Getting stream from address %v", state.address)
	stream, err := c.Receive(context.Background(), state.config.callOptions...)
	if err != nil {
		return err
	}
	go func() {
		_, err := stream.Recv()
		if err != nil {
			logdbg.Printf("EndpointWriter lost connection to address %v", state.address)

			//notify that the endpoint terminated
			terminated := &EndpointTerminatedEvent{
				Address: state.address,
			}
			eventstream.Publish(terminated)
		}
	}()

	logdbg.Printf("EndpointWriter connected to address %v", state.address)
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
		logdbg.Printf("gRPC Failed to send to address %v", state.address)
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
		logerr.Println("Unknown message", msg)
	}
}
