package remote

import (
	"log"

	"github.com/AsynkronIT/protoactor-go/actor"

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
		log.Printf("[REMOTING] EndpointWriter failed to connect to %v, err: %v", state.address, err)
	}
}

func (state *endpointWriter) initializeInternal() error {
	log.Printf("[REMOTING] Started EndpointWriter for address %v", state.address)
	log.Printf("[REMOTING] EndpointWriter connecting to address %v", state.address)
	conn, err := grpc.Dial(state.address, state.config.dialOptions...)
	if err != nil {
		return err
	}
	//	log.Printf("[REMOTING] Connected to address %v", state.address)
	state.conn = conn
	c := NewRemotingClient(conn)
	//	log.Printf("[REMOTING] Getting stream from address %v", state.address)
	stream, err := c.Receive(context.Background(), state.config.callOptions...)
	if err != nil {
		return err
	}
	go func() {
		_, err := stream.Recv()
		if err != nil {
			log.Printf("[REMOTING] EndpointWriter lost connection to address %v", state.address)

			//notify that the endpoint terminated
			terminated := &EndpointTerminatedEvent{
				Address: state.address,
			}
			actor.EventStream.Publish(terminated)
		}
	}()

	log.Printf("[REMOTING] EndpointWriter connected to address %v", state.address)
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
		log.Printf("[REMOTING] gRPC Failed to send to address %v", state.address)
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
