package remote

import (
	"io"

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
		if err == io.EOF {
			plog.Debug("EndpointReader stream closed")
			disconnectChan <- false
			return nil
		} else if err != nil {
			plog.Info("EndpointReader failed to read", log.Error(err))
			return err
		} else if s.suspended {
			// We read all messages ignoring them to gracefully end the request
			continue
		}

		switch t := msg.MessageType.(type) {
		case *RemoteMessage_ConnectRequest:
			plog.Debug("EndpointReader received connect request")
			c := t.ConnectRequest
			err, done := s.OnConnectRequest(stream, c)
			if done {
				return err
			}
		case *RemoteMessage_MessageBatch:
			m := t.MessageBatch
			err := s.OnMessageBatch(m)
			if err != nil {
				return err
			}
		default:
			{

			}
		}
	}
}

func (s *endpointReader) OnConnectRequest(stream Remoting_ReceiveServer, c *ConnectRequest) (error, bool) {
	switch tt := c.ConnectionType.(type) {
	case *ConnectRequest_ServerConnection:
		{
			sc := tt.ServerConnection
			s.onServerConnection(stream, sc)
		}
	case *ConnectRequest_ClientConnection:
		{
			//TODO implement me
		}
	default:
		plog.Error("EndpointReader received unknown connection type")
		return nil, true
	}
	return nil, false
}

func (s *endpointReader) OnMessageBatch(m *MessageBatch) error {
	for _, envelope := range m.Envelopes {
		data := envelope.MessageData
		var (
			sender *actor.PID = nil
			target *actor.PID = nil
		)

		if envelope.Sender > 0 {
			sender = m.Senders[envelope.Sender-1]
		}

		if envelope.Target > 0 {
			target = m.Targets[envelope.Target-1]
		}

		message, err := Deserialize(data, m.TypeNames[envelope.TypeId], envelope.SerializerId)
		if err != nil {
			plog.Error("EndpointReader failed to deserialize", log.Error(err))
			return err
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

		//TODO
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
