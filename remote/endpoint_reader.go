package remote

import (
	"errors"
	"io"
	"log/slog"

	"google.golang.org/protobuf/proto"

	"github.com/asynkron/protoactor-go/actor"
	"golang.org/x/net/context"
)

type endpointReader struct {
	suspended bool
	remote    *Remote
}

func (s *endpointReader) mustEmbedUnimplementedRemotingServer() {
	// TODO implement me
	panic("implement me")
}

func (s *endpointReader) ListProcesses(ctx context.Context, request *ListProcessesRequest) (*ListProcessesResponse, error) {
	panic("implement me")
}

func (s *endpointReader) GetProcessDiagnostics(ctx context.Context, request *GetProcessDiagnosticsRequest) (*GetProcessDiagnosticsResponse, error) {
	panic("implement me")
}

func newEndpointReader(r *Remote) *endpointReader {
	return &endpointReader{
		remote: r,
	}
}

func (s *endpointReader) Receive(stream Remoting_ReceiveServer) error {
	disconnectChan := make(chan bool, 1)
	s.remote.edpManager.endpointReaderConnections.Store(stream, disconnectChan)
	defer func() {
		close(disconnectChan)
	}()

	go func() {
		// endpointManager sends true
		// endpointReader sends false
		if <-disconnectChan {
			s.remote.Logger().Debug("EndpointReader is telling to remote that it's leaving")
			err := stream.Send(&RemoteMessage{
				MessageType: &RemoteMessage_DisconnectRequest{
					DisconnectRequest: &DisconnectRequest{},
				},
			})
			if err != nil {
				s.remote.Logger().Error("EndpointReader failed to send disconnection message", slog.Any("error", err))
			}
		} else {
			s.remote.edpManager.endpointReaderConnections.Delete(stream)
			s.remote.Logger().Debug("EndpointReader removed active endpoint from endpointManager")
		}
	}()

	for {
		msg, err := stream.Recv()
		switch {
		case errors.Is(err, io.EOF):
			s.remote.Logger().Info("EndpointReader stream closed")
			disconnectChan <- false
			return nil
		case err != nil:
			s.remote.Logger().Info("EndpointReader failed to read", slog.Any("error", err))
			return err
		case s.suspended:
			continue
		}

		switch t := msg.MessageType.(type) {
		case *RemoteMessage_ConnectRequest:
			s.remote.Logger().Debug("EndpointReader received connect request", slog.Any("message", t.ConnectRequest))
			c := t.ConnectRequest
			_, err := s.OnConnectRequest(stream, c)
			if err != nil {
				s.remote.Logger().Error("EndpointReader failed to handle connect request", slog.Any("error", err))
				return err
			}
		case *RemoteMessage_MessageBatch:
			m := t.MessageBatch
			err := s.onMessageBatch(m)
			if err != nil {
				return err
			}
		default:
			{
				s.remote.Logger().Warn("EndpointReader received unknown message type")
			}
		}
	}
}

func (s *endpointReader) OnConnectRequest(stream Remoting_ReceiveServer, c *ConnectRequest) (bool, error) {
	switch tt := c.ConnectionType.(type) {
	case *ConnectRequest_ServerConnection:
		{
			sc := tt.ServerConnection
			s.onServerConnection(stream, sc)
		}
	case *ConnectRequest_ClientConnection:
		{
			// TODO implement me
			s.remote.Logger().Error("ClientConnection not implemented")
		}
	default:
		s.remote.Logger().Error("EndpointReader received unknown connection type")
		return true, nil
	}
	return false, nil
}

func (s *endpointReader) onMessageBatch(m *MessageBatch) error {
	var (
		sender *actor.PID
		target *actor.PID
	)

	for _, envelope := range m.Envelopes {
		data := envelope.MessageData

		sender = deserializeSender(sender, envelope.Sender, envelope.SenderRequestId, m.Senders)
		target = deserializeTarget(target, envelope.Target, envelope.TargetRequestId, m.Targets)
		if target == nil {
			s.remote.Logger().Error("EndpointReader received message with unknown target", slog.Int("target", int(envelope.Target)), slog.Int("targetRequestId", int(envelope.TargetRequestId)))
			return errors.New("unknown target")
		}

		message, err := Deserialize(data, m.TypeNames[envelope.TypeId], envelope.SerializerId)
		if err != nil {
			s.remote.Logger().Error("EndpointReader failed to deserialize", slog.Any("error", err))
			return err
		}

		// translate from on-the-wire representation to in-process representation
		// this only applies to root level messages, and never on nested child messages
		if v, ok := message.(RootSerialized); ok {
			message = v.Deserialize()
		}

		switch msg := message.(type) {
		case *actor.Terminated:
			rt := &remoteTerminate{
				Watchee: msg.Who,
				Watcher: target,
			}
			s.remote.edpManager.remoteTerminate(rt)
		case actor.SystemMessage:
			ref, _ := s.remote.actorSystem.ProcessRegistry.GetLocal(target.Id)
			ref.SendSystemMessage(target, msg)
		default:
			var header map[string]string

			// fast path
			if sender == nil && envelope.MessageHeader == nil {
				s.remote.actorSystem.Root.Send(target, message)
				continue
			}

			// slow path
			if envelope.MessageHeader != nil {
				header = envelope.MessageHeader.HeaderData
			}
			localEnvelope := &actor.MessageEnvelope{
				Header:  header,
				Message: message,
				Sender:  sender,
			}
			s.remote.actorSystem.Root.Send(target, localEnvelope)
		}
	}
	return nil
}

func deserializeSender(pid *actor.PID, index int32, requestId uint32, arr []*actor.PID) *actor.PID {
	if index == 0 {
		pid = nil
	} else {
		pid = arr[index-1]

		// if request id is used. make sure to clone the PID first, so we don't corrupt the lookup
		if requestId > 0 {
			pid, _ = proto.Clone(pid).(*actor.PID)
			pid.RequestId = requestId
		}
	}
	return pid
}

func deserializeTarget(pid *actor.PID, index int32, requestId uint32, arr []*actor.PID) *actor.PID {
	pid = arr[index]

	// if request id is used. make sure to clone the PID first, so we don't corrupt the lookup
	if requestId > 0 {
		pid, _ = proto.Clone(pid).(*actor.PID)
		pid.RequestId = requestId
	}

	return pid
}

func (s *endpointReader) onServerConnection(stream Remoting_ReceiveServer, sc *ServerConnection) {
	if s.remote.BlockList().IsBlocked(sc.SystemId) {
		s.remote.Logger().Debug("EndpointReader is blocked")

		err := stream.Send(
			&RemoteMessage{
				MessageType: &RemoteMessage_ConnectResponse{
					ConnectResponse: &ConnectResponse{
						Blocked:  true,
						MemberId: s.remote.actorSystem.ID,
					},
				},
			})
		if err != nil {
			s.remote.Logger().Error("EndpointReader failed to send ConnectResponse message", slog.Any("error", err))
		}

		address := sc.Address
		systemID := sc.SystemId

		// TODO
		_ = address
		_ = systemID
	} else {
		err := stream.Send(
			&RemoteMessage{
				MessageType: &RemoteMessage_ConnectResponse{
					ConnectResponse: &ConnectResponse{
						Blocked:  false,
						MemberId: s.remote.actorSystem.ID,
					},
				},
			})
		if err != nil {
			s.remote.Logger().Error("EndpointReader failed to send ConnectResponse message", slog.Any("error", err))
		}
	}
}

func (s *endpointReader) suspend(toSuspend bool) {
	s.suspended = toSuspend
	if toSuspend {
		s.remote.Logger().Debug("Suspended EndpointReader")
	}
}
