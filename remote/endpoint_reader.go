package remote

import (
	io "io"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/log"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type endpointReader struct {
	suspended bool
	remote    *Remote
}

func newEndpointReader(r *Remote) *endpointReader {
	return &endpointReader{
		remote: r,
	}
}

func (s *endpointReader) Connect(ctx context.Context, req *ConnectRequest) (*ConnectResponse, error) {
	if s.suspended {
		return nil, status.Error(codes.Canceled, "Suspended")
	}

	return &ConnectResponse{DefaultSerializerId: DefaultSerializerID}, nil
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
			err := stream.SendMsg(&Unit{})
			if err != nil {
				plog.Error("EndpointReader failed to send disconnection message", log.Error(err))
			}
		} else {
			s.remote.edpManager.endpointReaderConnections.Delete(stream)
			plog.Debug("EndpointReader removed active endpoint from endpointManager")
		}
	}()

	targets := make([]*actor.PID, 100)
	for {
		batch, err := stream.Recv()
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

		// only grow pid lookup if needed
		if len(batch.TargetNames) > len(targets) {
			targets = make([]*actor.PID, len(batch.TargetNames))
		}

		for i := 0; i < len(batch.TargetNames); i++ {
			targets[i] = s.remote.actorSystem.NewLocalPID(batch.TargetNames[i])
		}

		for _, envelope := range batch.Envelopes {
			pid := targets[envelope.Target]
			message, err := Deserialize(envelope.MessageData, batch.TypeNames[envelope.TypeId], envelope.SerializerId)
			if err != nil {
				plog.Debug("EndpointReader failed to deserialize", log.Error(err))
				return err
			}
			// if message is system message send it as sysmsg instead of usermsg

			sender := envelope.Sender

			switch msg := message.(type) {
			case *actor.Terminated:
				rt := &remoteTerminate{
					Watchee: msg.Who,
					Watcher: pid,
				}
				s.remote.edpManager.remoteTerminate(rt)
			case actor.SystemMessage:
				ref, _ := s.remote.actorSystem.ProcessRegistry.GetLocal(pid.Id)
				ref.SendSystemMessage(pid, msg)
			default:
				var header map[string]string
				if envelope.MessageHeader != nil {
					header = envelope.MessageHeader.HeaderData
				}
				localEnvelope := &actor.MessageEnvelope{
					Header:  header,
					Message: message,
					Sender:  sender,
				}
				s.remote.actorSystem.Root.Send(pid, localEnvelope)
			}
		}
	}
}

func (s *endpointReader) suspend(toSuspend bool) {
	s.suspended = toSuspend
	if toSuspend {
		plog.Debug("Suspended EndpointReader")
	}
}
