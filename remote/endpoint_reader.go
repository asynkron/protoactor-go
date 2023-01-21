package remote

import (
	"errors"
	"io"

	"google.golang.org/protobuf/proto"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/log"
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
			plog.Debug("EndpointReader is telling to remote that it's leaving")
			err := stream.Send(&RemoteMessage{
				MessageType: &RemoteMessage_DisconnectRequest{
					DisconnectRequest: &DisconnectRequest{},
				},
			})
			if err != nil {
				plog.Error("EndpointReader failed to send disconnection message", log.Error(err))
			}
		} else {
			s.remote.edpManager.endpointReaderConnections.Delete(stream)
			plog.Debug("EndpointReader removed active endpoint from endpointManager")
		}
	}()

	for {
		msg, err := stream.Recv()
		switch {
		case errors.Is(err, io.EOF):
			plog.Info("EndpointReader stream closed")
			disconnectChan <- false
			return nil
		case err != nil:
			plog.Info("EndpointReader failed to read", log.Error(err))
			return err
		case s.suspended:
			continue
		}

		switch t := msg.MessageType.(type) {
		case *RemoteMessage_ConnectRequest:
			plog.Debug("EndpointReader received connect request", log.Message(t.ConnectRequest))
			c := t.ConnectRequest
			_, err := s.OnConnectRequest(stream, c)
			if err != nil {
				plog.Error("EndpointReader failed to handle connect request", log.Error(err))
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
				plog.Warn("EndpointReader received unknown message type")
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
			plog.Error("ClientConnection not implemented")
		}
	default:
		plog.Error("EndpointReader received unknown connection type")
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
			plog.Error("EndpointReader received message with unknown target", log.Int("target", int(envelope.Target)), log.Int("targetRequestId", int(envelope.TargetRequestId)))
			return errors.New("unknown target")
		}

		message, err := Deserialize(data, m.TypeNames[envelope.TypeId], envelope.SerializerId)
		if err != nil {
			plog.Error("EndpointReader failed to deserialize", log.Error(err))
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
		plog.Debug("EndpointReader is blocked", log.String("systemId", sc.SystemId))

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
			plog.Error("EndpointReader failed to send ConnectResponse message", log.Error(err))
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
			plog.Error("EndpointReader failed to send ConnectResponse message", log.Error(err))
		}
	}
}

func (s *endpointReader) suspend(toSuspend bool) {
	s.suspended = toSuspend
	if toSuspend {
		plog.Debug("Suspended EndpointReader")
	}
}
