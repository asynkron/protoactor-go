package remote

import (
	io "io"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/log"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func endpointWriterProducer(remote *Remote, address string, config *Config) actor.Producer {
	return func() actor.Actor {
		return &endpointWriter{
			address: address,
			config:  config,
			remote:  remote,
		}
	}
}

type endpointWriter struct {
	config              *Config
	address             string
	conn                *grpc.ClientConn
	stream              Remoting_ReceiveClient
	defaultSerializerId int32
	remote              *Remote
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
	plog.Info("Started EndpointWriter. connecting", log.String("address", state.address))
	conn, err := grpc.Dial(state.address, state.config.DialOptions...)
	if err != nil {
		plog.Info("EndpointWriter connect failed", log.String("address", state.address), log.Error(err))
		return err
	}
	state.conn = conn
	c := NewRemotingClient(conn)
	resp, err := c.Connect(context.Background(), &ConnectRequest{})
	if err != nil {
		plog.Info("EndpointWriter connect failed", log.String("address", state.address), log.Error(err))
		return err
	}
	state.defaultSerializerId = resp.DefaultSerializerId

	//	log.Printf("Getting stream from address %v", state.address)
	stream, err := c.Receive(context.Background(), state.config.CallOptions...)
	if err != nil {
		plog.Info("EndpointWriter connect failed", log.String("address", state.address), log.Error(err))
		return err
	}
	go func() {
		for {
			_, err := stream.Recv()
			if err == io.EOF {
				plog.Debug("EndpointWriter stream completed", log.String("address", state.address))
				break
			} else if err != nil {
				plog.Error("EndpointWriter lost connection", log.String("address", state.address), log.Error(err))

				// notify that the endpoint terminated
				terminated := &EndpointTerminatedEvent{
					Address: state.address,
				}
				state.remote.actorSystem.EventStream.Publish(terminated)
				break
			} else {
				plog.Info("EndpointWriter remote disconnected", log.String("address", state.address))
				// notify that the endpoint terminated
				terminated := &EndpointTerminatedEvent{
					Address: state.address,
				}
				state.remote.actorSystem.EventStream.Publish(terminated)
			}
		}
	}()

	plog.Info("EndpointWriter connected", log.String("address", state.address))
	connected := &EndpointConnectedEvent{Address: state.address}
	state.remote.actorSystem.EventStream.Publish(connected)
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
		if state.stream != nil {
			err := state.stream.CloseSend()
			if err != nil {
				plog.Error("EndpointWriter error when closing the stream", log.Error(err))
			}
		}
	case *actor.Restarting:
		if state.stream != nil {
			err := state.stream.CloseSend()
			if err != nil {
				plog.Error("EndpointWriter error when closing the stream", log.Error(err))
			}
		}
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
