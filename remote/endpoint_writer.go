package remote

import (
	"github.com/AsynkronIT/protoactor-go/actor"

	"github.com/AsynkronIT/protoactor-go/eventstream"
	"github.com/AsynkronIT/protoactor-go/log"
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
		plog.Error("EndpointWriter failed to connect", log.String("address", state.address), log.Error(err))
	}
}

func (state *endpointWriter) initializeInternal() error {
	plog.Info("Started EndpointWriter", log.String("address", state.address))
	plog.Info("EndpointWatcher connecting", log.String("address", state.address))
	conn, err := grpc.Dial(state.address, state.config.dialOptions...)
	if err != nil {
		return err
	}
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
			plog.Info("EndpointWriter lost connection to address", log.String("address", state.address))

			//notify that the endpoint terminated
			terminated := &EndpointTerminatedEvent{
				Address: state.address,
			}
			eventstream.Publish(terminated)
		}
	}()

	plog.Info("EndpointWriter connected", log.String("address", state.address))
	state.stream = stream
	return nil
}

func (state *endpointWriter) sendEnvelopes(msg []interface{}, ctx actor.Context) {
	envelopes := make([]*MessageEnvelope, len(msg))

	//type name uniqueness map name string to type index
	typeNames := make(map[string]int32)
	typeNamesArr := make([]string, 0)
	targetNames := make(map[string]int32)
	targetNamesArr := make([]string, 0)
	var typeID int32
	var targetID int32
	for i, tmp := range msg {
		rd := tmp.(*remoteDeliver)
		bytes, typeName, _ := serialize(rd.message)
		typeID, typeNamesArr = addToLookup(typeNames, typeName, typeNamesArr)
		targetID, targetNamesArr = addToLookup(targetNames, rd.target.Id, targetNamesArr)

		envelopes[i] = &MessageEnvelope{
			MessageData: bytes,
			Sender:      rd.sender,
			Target:      targetID,
			TypeId:      typeID,
		}
	}

	batch := &MessageBatch{
		TypeNames:   typeNamesArr,
		TargetNames: targetNamesArr,
		Envelopes:   envelopes,
	}
	err := state.stream.Send(batch)

	if err != nil {
		ctx.Stash()
		plog.Debug("gRPC Failed to send", log.String("address", state.address))
		panic("restart it")
	}
}

func addToLookup(m map[string]int32, name string, a []string) (int32, []string) {
	max := int32(len(m))
	id, ok := m[name]
	if !ok {
		m[name] = max
		id = max
		a = append(a, name)
	}
	return id, a
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
		plog.Error("Unknown message", log.Message(msg))
	}
}
