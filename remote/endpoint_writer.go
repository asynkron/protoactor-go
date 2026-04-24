package remote

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
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
	config     *Config
	address    string
	conn       *grpc.ClientConn
	stream     Remoting_ReceiveClient
	remote     *Remote
	cancelFunc context.CancelFunc
}

type restartAfterConnectFailure struct {
	err error
}

// calcBackoffDelay computes an exponential backoff delay with jitter.
// delay = min(baseDelay * 2^attempt, maxDelay) + rand(0, delay/4)
func calcBackoffDelay(baseDelay, maxDelay time.Duration, attempt int) time.Duration {
	delay := baseDelay
	if attempt < 63 {
		delay = baseDelay * time.Duration(1<<uint(attempt))
	}
	if delay <= 0 || delay > maxDelay {
		delay = maxDelay
	}
	// Add 0-25% jitter to prevent thundering herd
	if quarter := int64(delay / 4); quarter > 0 {
		delay += time.Duration(rand.Int63n(quarter))
	}
	return delay
}

func (state *endpointWriter) initialize(_ actor.Context) {
	now := time.Now()

	state.remote.Logger().Info("Started EndpointWriter. connecting", slog.String("address", state.address))

	var err error

	for i := 0; i < state.remote.config.MaxRetryCount; i++ {
		err = state.initializeInternal()
		if err != nil {
			state.remote.Logger().Error("EndpointWriter failed to connect",
				slog.String("address", state.address), slog.Any("error", err), slog.Int("retry", i))
			delay := calcBackoffDelay(state.config.RetryBaseDelay, state.config.RetryMaxDelay, i)
			time.Sleep(delay)
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
	}

	state.remote.Logger().Info("EndpointWriter connected", slog.String("address", state.address), slog.Duration("cost", time.Since(now)))
}

func (state *endpointWriter) initializeInternal() error {
	conn, err := grpc.NewClient(state.address, state.config.DialOptions...)
	if err != nil {
		return err
	}
	state.conn = conn
	c := NewRemotingClient(conn)
	// Cancel any previous context from a failed retry attempt
	if state.cancelFunc != nil {
		state.cancelFunc()
	}
	ctx, cancel := context.WithCancel(context.Background())
	state.cancelFunc = cancel
	stream, err := c.Receive(ctx, state.config.CallOptions...)
	if err != nil {
		cancel()
		state.cancelFunc = nil
		state.remote.Logger().Error("EndpointWriter failed to create receive stream", slog.String("address", state.address), slog.Any("error", err))
		return err
	}
	state.stream = stream

	err = stream.Send(&RemoteMessage{
		MessageType: &RemoteMessage_ConnectRequest{
			ConnectRequest: &ConnectRequest{
				ConnectionType: &ConnectRequest_ServerConnection{
					ServerConnection: &ServerConnection{
						MemberId:  state.remote.actorSystem.ID,
						Address:   state.remote.actorSystem.Address(),
						BlockList: state.remote.BlockList().BlockedMembers().ToSlice(),
					},
				},
			},
		},
	})
	if err != nil {
		state.remote.Logger().Error("EndpointWriter failed to send connect request", slog.String("address", state.address), slog.Any("error", err))
		return err
	}

	connection, err := stream.Recv()
	if err != nil {
		state.remote.Logger().Error("EndpointWriter failed to receive connect response", slog.String("address", state.address), slog.Any("error", err))
		return err
	}

	switch connection.MessageType.(type) {
	case *RemoteMessage_ConnectResponse:
		connectResponse := connection.GetConnectResponse()
		state.remote.Logger().Debug("Received connect response",
			slog.String("fromAddress", state.address),
			slog.Bool("blocked", connectResponse.GetBlocked()))
		if connectResponse.GetBlocked() {
			state.remote.Logger().Warn("EndpointWriter blocked by remote server",
				slog.String("address", state.address))
			return fmt.Errorf("blocked by remote server %s", state.address)
		}
	default:
		state.remote.Logger().Error("EndpointWriter got invalid connect response", slog.String("address", state.address), slog.Any("type", connection.MessageType))
		return errors.New("invalid connect response")
	}

	go func() {
		for {
			_, err := stream.Recv()
			switch {
			case errors.Is(err, io.EOF):
				state.remote.Logger().Debug("EndpointWriter stream completed", slog.String("address", state.address))
				return
			case err != nil:
				state.remote.Logger().Error("EndpointWriter lost connection", slog.String("address", state.address), slog.Any("error", err))
				terminated := &EndpointTerminatedEvent{
					Address: state.address,
				}
				state.remote.actorSystem.EventStream.Publish(terminated)
				return
			default: // DisconnectRequest
				state.remote.Logger().Info("EndpointWriter got DisconnectRequest form remote", slog.String("address", state.address))
				terminated := &EndpointTerminatedEvent{
					Address: state.address,
				}
				state.remote.actorSystem.EventStream.Publish(terminated)
			}
		}
	}()

	connected := &EndpointConnectedEvent{Address: state.address}
	state.remote.actorSystem.EventStream.Publish(connected)

	if state.remote.metricsEnabled {
		_ctx := context.Background()
		attrs := append(actor.SystemLabels(state.remote.actorSystem), attribute.String("destinationaddress", state.address))
		state.remote.metrics.RemoteEndpointConnectedCount.Add(_ctx, 1, metric.WithAttributes(attrs...))
	}

	return nil
}

func (state *endpointWriter) sendEnvelopes(msg []any, ctx actor.Context) {
	batchSize := len(msg)
	envelopes := make([]*MessageEnvelope, 0, batchSize)

	// type name uniqueness map name string to type index
	typeNames := make(map[string]int32, 16)
	typeNamesArr := make([]string, 0, 16)

	targetNames := make(map[string]int32, batchSize/2+1)
	targetNamesArr := make([]string, 0, batchSize/2+1)

	senderNames := make(map[string]int32, batchSize/4+1)
	senderNamesArr := make([]*actor.PID, 0, batchSize/4+1)

	var (
		header       *MessageHeader
		typeID       int32
		targetID     int32
		senderID     int32
		serializerID int32
	)

	for _, tmp := range msg {
		switch unwrapped := tmp.(type) {
		case *EndpointTerminatedEvent, EndpointTerminatedEvent:
			state.remote.Logger().Debug("Handling array wrapped terminate event", slog.String("address", state.address), slog.Any("message", unwrapped))
			ctx.Stop(ctx.Self())
			return
		}

		// ensure the message is a remoteDeliver before proceeding
		rd, ok := tmp.(*remoteDeliver)
		if !ok {
			state.remote.Logger().Error("EndpointWriter received unknown message", slog.Any("message", tmp))
			continue
		}

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
		var err error
		if v, ok := message.(RootSerializable); ok {
			message, err = v.Serialize()
			if err != nil {
				state.remote.Logger().Error("EndpointWriter failed to serialize message", slog.String("address", state.address), slog.Any("error", err), slog.Any("message", v))
				if rd.sender != nil {
					state.remote.actorSystem.Root.Send(rd.sender, &actor.DeadLetterResponse{Target: rd.target})
				} else {
					state.remote.actorSystem.EventStream.Publish(&actor.DeadLetterEvent{Message: rd.message, Sender: rd.sender, PID: rd.target})
				}
				continue
			}
		}

		bytes, typeName, err := Serialize(message, serializerID)
		if err != nil {
			state.remote.Logger().Error("EndpointWriter failed to serialize message", slog.String("address", state.address), slog.Any("error", err), slog.Any("message", message))
			if rd.sender != nil {
				state.remote.actorSystem.Root.Send(rd.sender, &actor.DeadLetterResponse{Target: rd.target})
			} else {
				state.remote.actorSystem.EventStream.Publish(&actor.DeadLetterEvent{Message: rd.message, Sender: rd.sender, PID: rd.target})
			}
			continue
		}

		if state.remote.metricsEnabled {
			_ctx := context.Background()
			attrs := append(actor.SystemLabels(state.remote.actorSystem), attribute.String("messagetype", typeName))
			state.remote.metrics.RemoteSerializedMessageCount.Add(_ctx, 1, metric.WithAttributes(attrs...))
			state.remote.metrics.RemoteMessageSizeBytes.Record(_ctx, int64(len(bytes)), metric.WithAttributes(attrs...))
		}

		typeID, typeNamesArr = addToLookup(typeNames, typeName, typeNamesArr)
		targetID, targetNamesArr = addToTargetLookup(targetNames, rd.target, targetNamesArr)
		targetRequestID := rd.target.RequestId

		senderID, senderNamesArr = addToSenderLookup(senderNames, rd.sender, senderNamesArr)
		senderRequestID := uint32(0)
		if rd.sender != nil {
			senderRequestID = rd.sender.RequestId
		}

		envelopes = append(envelopes, &MessageEnvelope{
			MessageHeader:   header,
			MessageData:     bytes,
			Sender:          senderID,
			Target:          targetID,
			TypeId:          typeID,
			SerializerId:    serializerID,
			TargetRequestId: targetRequestID,
			SenderRequestId: senderRequestID,
		})
	}

	if len(envelopes) == 0 {
		return
	}

	if state.remote.metricsEnabled {
		_ctx := context.Background()
		attrs := append(actor.SystemLabels(state.remote.actorSystem), attribute.String("destinationaddress", state.address))
		state.remote.metrics.RemoteMessageBatchSize.Record(_ctx, int64(len(envelopes)), metric.WithAttributes(attrs...))
	}

	start := time.Now()
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

	if state.remote.metricsEnabled {
		_ctx := context.Background()
		attrs := append(actor.SystemLabels(state.remote.actorSystem), attribute.String("destinationaddress", state.address))
		state.remote.metrics.RemoteWriteDuration.Record(_ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs...))
	}

	if state.remote.metricsEnabled && state.remote.config.EnablePerEndpointMetrics {
		_ctx := context.Background()
		attrs := append(actor.SystemLabels(state.remote.actorSystem), attribute.String("destinationaddress", state.address))
		state.remote.metrics.RemoteMessageSentTotal.Add(_ctx, int64(len(envelopes)), metric.WithAttributes(attrs...))
	}

	if err != nil {
		ctx.Stash()
		state.remote.Logger().Debug("gRPC Failed to send", slog.String("address", state.address), slog.Any("error", err))
		ctx.Stop(ctx.Self())
	}
}

func addToLookup(m map[string]int32, name string, a []string) (int32, []string) {
	maxIdx := int32(len(m))
	id, ok := m[name]
	if !ok {
		m[name] = maxIdx
		id = maxIdx
		a = append(a, name)
	}
	return id, a
}

func addToTargetLookup(m map[string]int32, pid *actor.PID, arr []string) (int32, []string) {
	maxIdx := int32(len(m))
	key := pid.Address + "/" + pid.Id
	id, ok := m[key]
	if !ok {
		m[key] = maxIdx
		id = maxIdx
		arr = append(arr, pid.Id)
	}
	return id, arr
}

func addToSenderLookup(m map[string]int32, pid *actor.PID, arr []*actor.PID) (int32, []*actor.PID) {
	if pid == nil {
		return 0, arr
	}

	maxIdx := int32(len(m))
	key := pid.Address + "/" + pid.Id
	id, ok := m[key]
	if !ok {
		c, _ := proto.Clone(pid).(*actor.PID)
		c.RequestId = 0
		m[key] = maxIdx
		id = maxIdx
		arr = append(arr, c)
	}
	return id + 1, arr
}

func (state *endpointWriter) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		state.initialize(ctx)
	case *actor.Stopped:
		state.remote.Logger().Debug("EndpointWriter stopped", slog.String("address", state.address))
		state.closeClientConn()
	case *actor.Restarting:
		state.remote.Logger().Debug("EndpointWriter restarting", slog.String("address", state.address))
		state.closeClientConn()
	case *EndpointTerminatedEvent:
		state.remote.Logger().Info("EndpointWriter received EndpointTerminatedEvent, stopping", slog.String("address", state.address))
		ctx.Stop(ctx.Self())
	case *restartAfterConnectFailure:
		state.remote.Logger().Error("EndpointWriter connect failure, terminating endpoint",
			slog.String("address", state.address), slog.Any("error", msg.err))
		terminated := &EndpointTerminatedEvent{Address: state.address}
		state.remote.actorSystem.EventStream.Publish(terminated)
		ctx.Stop(ctx.Self())
	case []any:
		state.sendEnvelopes(msg, ctx)
	case actor.SystemMessage, actor.AutoReceiveMessage:
		// ignore
	default:
		state.remote.Logger().Error("EndpointWriter received unknown message", slog.String("address", state.address), slog.Any("message", msg))
	}
}

func (state *endpointWriter) closeClientConn() {
	state.remote.Logger().Info("EndpointWriter closing client connection", slog.String("address", state.address))

	if state.remote.metricsEnabled {
		_ctx := context.Background()
		state.remote.metrics.RemoteEndpointDisconnectedCount.Add(_ctx, 1, metric.WithAttributes(actor.SystemLabels(state.remote.actorSystem)...))
	}
	// Cancel the stream context first to unblock the background goroutine
	// that is reading from the stream via Recv().
	if state.cancelFunc != nil {
		state.cancelFunc()
		state.cancelFunc = nil
	}
	if state.stream != nil {
		err := state.stream.CloseSend()
		if err != nil {
			state.remote.Logger().Error("EndpointWriter error when closing the stream", slog.Any("error", err))
		}
		state.stream = nil
	}
	if state.conn != nil {
		err := state.conn.Close()
		if err != nil {
			state.remote.Logger().Error("EndpointWriter error when closing the client conn", slog.Any("error", err))
		}
		state.conn = nil
	}
}
