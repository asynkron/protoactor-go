package remote

import (
	"time"

	"github.com/otherview/protoactor-go/actor"
	"github.com/otherview/protoactor-go/eventstream"
	"github.com/otherview/protoactor-go/log"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func endpointWriterProducer(address string, config *remoteConfig) actor.Producer {
	return func() actor.Actor {
		return &endpointWriter{
			address: address,
			config:  config,
		}
	}
}

type endpointWriter struct {
	config              *remoteConfig
	address             string
	conn                *grpc.ClientConn
	stream              Remoting_ReceiveClient
	defaultSerializerId int32
}

func (state *endpointWriter) initialize() {
	err := state.initializeInternal()
	if err != nil {
		plog.Error("EndpointWriter failed to connect", log.String("address", state.address), log.Error(err))
		// Wait 2 seconds to restart and retry
		// Replace with Exponential Backoff
		time.Sleep(2 * time.Second)
		panic(err)
	}
}

func (state *endpointWriter) initializeInternal() error {
	plog.Info("Started EndpointWriter", log.String("address", state.address))
	plog.Info("EndpointWriter connecting", log.String("address", state.address))
	conn, err := grpc.Dial(state.address, state.config.dialOptions...)
	if err != nil {
		return err
	}
	state.conn = conn
	c := NewRemotingClient(conn)
	resp, err := c.Connect(context.Background(), &ConnectRequest{})
	if err != nil {
		return err
	}
	state.defaultSerializerId = resp.DefaultSerializerId

	//	log.Printf("Getting stream from address %v", state.address)
	stream, err := c.Receive(context.Background(), state.config.callOptions...)
	if err != nil {
		return err
	}
	go func() {
		_, err := stream.Recv()
		if err != nil {
			plog.Info("EndpointWriter lost connection to address", log.String("address", state.address), log.Error(err))

			// notify that the endpoint terminated
			terminated := &EndpointTerminatedEvent{
				Address: state.address,
			}
			eventstream.Publish(terminated)
		}
	}()

	plog.Info("EndpointWriter connected", log.String("address", state.address))
	connected := &EndpointConnectedEvent{Address: state.address}
	eventstream.Publish(connected)
	state.stream = stream
	return nil
}

func (state *endpointWriter) sendEnvelopes(msg []interface{}, ctx actor.Context) {
	envelopes := make([]*MessageEnvelope, len(msg))

	// type name uniqueness map name string to type index
	typeNames := make(map[string]int32)
	typeNamesArr := make([]string, 0)
	targetNames := make(map[string]int32)
	targetNamesArr := make([]string, 0)
	var header *MessageHeader
	var typeID int32
	var targetID int32
	var serializerID int32
	for i, tmp := range msg {

		switch unwrapped := tmp.(type) {
		case *EndpointTerminatedEvent, EndpointTerminatedEvent:
			plog.Debug("Handling array wrapped terminate event", log.String("address", state.address), log.Object("msg", unwrapped))
			ctx.Stop(ctx.Self())
			return
		}
		rd := tmp.(*remoteDeliver)

		if rd.serializerID == -1 {
			serializerID = state.defaultSerializerId
		} else {
			serializerID = rd.serializerID
		}

		if rd.header == nil || rd.header.Length() == 0 {
			header = nil
		} else {
			header = &MessageHeader{rd.header.ToMap()}
		}

		bytes, typeName, err := Serialize(rd.message, serializerID)
		if err != nil {
			panic(err)
		}
		typeID, typeNamesArr = addToLookup(typeNames, typeName, typeNamesArr)
		targetID, targetNamesArr = addToLookup(targetNames, rd.target.Id, targetNamesArr)

		envelopes[i] = &MessageEnvelope{
			MessageHeader: header,
			MessageData:   bytes,
			Sender:        rd.sender,
			Target:        targetID,
			TypeId:        typeID,
			SerializerId:  serializerID,
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
		plog.Debug("gRPC Failed to send", log.String("address", state.address), log.Error(err))
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
	case *EndpointTerminatedEvent:
		ctx.Stop(ctx.Self())
	case []interface{}:
		state.sendEnvelopes(msg, ctx)
	case actor.SystemMessage, actor.AutoReceiveMessage:
		// ignore
	default:
		plog.Error("EndpointWriter received unknown message", log.String("address", state.address), log.TypeOf("type", msg), log.Message(msg))
	}
}
