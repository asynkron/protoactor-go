package remote

import (
	"errors"
	"io"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/log"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
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
	config  *Config
	address string
	conn    *grpc.ClientConn
	stream  Remoting_ReceiveClient
	remote  *Remote
}

type restartAfterConnectFailure struct {
	err error
}

func (state *endpointWriter) initialize(ctx actor.Context) {
	now := time.Now()
	plog.Info("Started EndpointWriter. connecting", log.String("address", state.address))

	var err error

	for i := 0; i < state.remote.config.MaxRetryCount; i++ {
		err = state.initializeInternal()
		if err != nil {
			plog.Error("EndpointWriter failed to connect", log.String("address", state.address), log.Error(err), log.Int("retry", i))
			// Wait 2 seconds to restart and retry
			// Replace with Exponential Backoff
			time.Sleep(2 * time.Second)
			continue
		}

		break
	}

	if err != nil {
		terminated := &EndpointTerminatedEvent{
			Address: state.address,
		}
		state.remote.actorSystem.EventStream.Publish(terminated)

		return

		//	plog.Error("EndpointWriter failed to connect", log.String("address", state.address), log.Error(err))

		// Wait 2 seconds to restart and retry
		// TODO: Replace with Exponential Backoff
		// send this as a message to self - do not block the mailbox processing
		// if in the meantime the actor is stopped (EndpointTerminated event), the message will be ignored (deadlettered)
		// TODO: would it be a better idea to just publish EndpointTerminatedEvent here? to use the same path as when the connection is lost?
		//	time.AfterFunc(2*time.Second, func() {
		//		ctx.Send(ctx.Self(), &restartAfterConnectFailure{err})
		//	})

	}

	plog.Info("EndpointWriter connected", log.String("address", state.address), log.Duration("cost", time.Since(now)))
}

func (state *endpointWriter) initializeInternal() error {
	conn, err := grpc.Dial(state.address, state.config.DialOptions...)
	if err != nil {
		return err
	}
	state.conn = conn
	c := NewRemotingClient(conn)
	stream, err := c.Receive(context.Background(), state.config.CallOptions...)
	if err != nil {
		plog.Error("EndpointWriter failed to create receive stream", log.String("address", state.address), log.Error(err))
		return err
	}
	state.stream = stream

	err = stream.Send(&RemoteMessage{
		MessageType: &RemoteMessage_ConnectRequest{
			ConnectRequest: &ConnectRequest{
				ConnectionType: &ConnectRequest_ServerConnection{
					ServerConnection: &ServerConnection{
						SystemId: state.remote.actorSystem.ID,
						Address:  state.remote.actorSystem.Address(),
					},
				},
			},
		},
	})
	if err != nil {
		plog.Error("EndpointWriter failed to send connect request", log.String("address", state.address), log.Error(err))
		return err
	}

	connection, err := stream.Recv()
	if err != nil {
		plog.Error("EndpointWriter failed to receive connect response", log.String("address", state.address), log.Error(err))
		return err
	}

	switch connection.MessageType.(type) {
	case *RemoteMessage_ConnectResponse:
		plog.Debug("Received connect response", log.String("fromAddress", state.address))
		// TODO: handle blocked status received from remote server
		break
	default:
		plog.Error("EndpointWriter got invalid connect response", log.String("address", state.address), log.TypeOf("type", connection.MessageType))
		return errors.New("invalid connect response")
	}

	go func() {
		for {
			_, err := stream.Recv()
			switch {
			case errors.Is(err, io.EOF):
				plog.Debug("EndpointWriter stream completed", log.String("address", state.address))
				return
			case err != nil:
				plog.Error("EndpointWriter lost connection", log.String("address", state.address), log.Error(err))
				terminated := &EndpointTerminatedEvent{
					Address: state.address,
				}
				state.remote.actorSystem.EventStream.Publish(terminated)
				return
			default: // DisconnectRequest
				plog.Info("EndpointWriter got DisconnectRequest form remote", log.String("address", state.address))
				terminated := &EndpointTerminatedEvent{
					Address: state.address,
				}
				state.remote.actorSystem.EventStream.Publish(terminated)
			}
		}
	}()

	connected := &EndpointConnectedEvent{Address: state.address}
	state.remote.actorSystem.EventStream.Publish(connected)
	return nil
}

func (state *endpointWriter) sendEnvelopes(msg []interface{}, ctx actor.Context) {
	envelopes := make([]*MessageEnvelope, len(msg))

	// type name uniqueness map name string to type index
	typeNames := make(map[string]int32)
	typeNamesArr := make([]string, 0)

	targetNames := make(map[string]int32)
	targetNamesArr := make([]*actor.PID, 0)

	senderNames := make(map[string]int32)
	senderNamesArr := make([]*actor.PID, 0)

	var (
		header       *MessageHeader
		typeID       int32
		targetID     int32
		senderID     int32
		serializerID int32
	)

	for i, tmp := range msg {
		switch unwrapped := tmp.(type) {
		case *EndpointTerminatedEvent, EndpointTerminatedEvent:
			plog.Debug("Handling array wrapped terminate event", log.String("address", state.address), log.Object("msg", unwrapped))
			ctx.Stop(ctx.Self())
			return
		}

		rd, _ := tmp.(*remoteDeliver)

		if state.stream == nil { // not connected yet since first connection attempt failed and we are waiting for the retry
			if rd.sender != nil {
				state.remote.actorSystem.Root.Send(rd.sender, &actor.DeadLetterResponse{Target: rd.target})
			} else {
				state.remote.actorSystem.EventStream.Publish(&actor.DeadLetterEvent{Message: rd.message, Sender: rd.sender, PID: rd.target})
			}
			continue
		}

		if rd.header == nil || rd.header.Length() == 0 {
			header = nil
		} else {
			header = &MessageHeader{
				HeaderData: rd.header.ToMap(),
			}
		}

		// if the message can be translated to a serialization representation, we do this here
		// this only apply to root level messages and never to nested child objects inside the message
		message := rd.message
		if v, ok := message.(RootSerializable); ok {
			message = v.Serialize()
		}

		bytes, typeName, err := Serialize(message, serializerID)
		if err != nil {
			panic(err)
		}
		typeID, typeNamesArr = addToLookup(typeNames, typeName, typeNamesArr)
		targetID, targetNamesArr = addToTargetLookup(targetNames, rd.target, targetNamesArr)
		targetRequestID := rd.target.RequestId

		senderID, senderNamesArr = addToSenderLookup(senderNames, rd.sender, senderNamesArr)
		senderRequestID := uint32(0)
		if rd.sender != nil {
			senderRequestID = rd.sender.RequestId
		}

		envelopes[i] = &MessageEnvelope{
			MessageHeader:   header,
			MessageData:     bytes,
			Sender:          senderID,
			Target:          targetID,
			TypeId:          typeID,
			SerializerId:    serializerID,
			TargetRequestId: targetRequestID,
			SenderRequestId: senderRequestID,
		}
	}

	err := state.stream.Send(&RemoteMessage{
		MessageType: &RemoteMessage_MessageBatch{
			MessageBatch: &MessageBatch{
				TypeNames: typeNamesArr,
				Targets:   targetNamesArr,
				Senders:   senderNamesArr,
				Envelopes: envelopes,
			},
		},
	})
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

func addToTargetLookup(m map[string]int32, pid *actor.PID, arr []*actor.PID) (int32, []*actor.PID) {
	max := int32(len(m))
	key := pid.Address + "/" + pid.Id
	id, ok := m[key]
	if !ok {
		c, _ := proto.Clone(pid).(*actor.PID)
		c.RequestId = 0
		m[key] = max
		id = max
		arr = append(arr, c)
	}
	return id, arr
}

func addToSenderLookup(m map[string]int32, pid *actor.PID, arr []*actor.PID) (int32, []*actor.PID) {
	if pid == nil {
		return 0, arr
	}

	max := int32(len(m))
	key := pid.Address + "/" + pid.Id
	id, ok := m[key]
	if !ok {
		c, _ := proto.Clone(pid).(*actor.PID)
		c.RequestId = 0
		m[key] = max
		id = max
		arr = append(arr, c)
	}
	return id + 1, arr
}

func (state *endpointWriter) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		state.initialize(ctx)
	case *actor.Stopped:
		plog.Debug("EndpointWriter stopped", log.String("address", state.address))
		state.closeClientConn()
	case *actor.Restarting:
		plog.Debug("EndpointWriter restarting", log.String("address", state.address))
		state.closeClientConn()
	case *EndpointTerminatedEvent:
		plog.Info("EndpointWriter received EndpointTerminatedEvent, stopping", log.String("address", state.address))
		ctx.Stop(ctx.Self())
	case *restartAfterConnectFailure:
		plog.Debug("EndpointWriter initiating self-restart after failing to connect and a delay", log.String("address", state.address))
		panic(msg.err)
	case []interface{}:
		state.sendEnvelopes(msg, ctx)
	case actor.SystemMessage, actor.AutoReceiveMessage:
		// ignore
	default:
		plog.Error("EndpointWriter received unknown message", log.String("address", state.address), log.TypeOf("type", msg), log.Message(msg))
	}
}

func (state *endpointWriter) closeClientConn() {
	plog.Info("EndpointWriter closing client connection", log.String("address", state.address))
	if state.stream != nil {
		err := state.stream.CloseSend()
		if err != nil {
			plog.Error("EndpointWriter error when closing the stream", log.Error(err))
		}
		state.stream = nil
	}
	if state.conn != nil {
		err := state.conn.Close()
		if err != nil {
			plog.Error("EndpointWriter error when closing the client conn", log.Error(err))
		}
		state.conn = nil
	}
}
