package remote

import (
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/log"
	"golang.org/x/net/context"
)

type endpointReader struct {
	suspended bool
}

func (s *endpointReader) Connect(ctx context.Context, req *ConnectRequest) (*ConnectResponse, error) {
	return &ConnectResponse{DefaultSerializerId: DefaultSerializerID}, nil
}

func (s *endpointReader) Receive(stream Remoting_ReceiveServer) error {
	for {
		if s.suspended {
			time.Sleep(time.Millisecond * 500)
			continue
		}

		batch, err := stream.Recv()
		if err != nil {
			plog.Debug("EndpointReader failed to read", log.Error(err))
			return err
		}

		for _, envelope := range batch.Envelopes {
			targetName := batch.TargetNames[envelope.Target]
			pid := actor.NewLocalPID(targetName)
			message, err := Deserialize(envelope.MessageData, batch.TypeNames[envelope.TypeId], envelope.SerializerId)
			if err != nil {
				plog.Debug("EndpointReader failed to deserialize", log.Error(err))
				return err
			}
			//if message is system message send it as sysmsg instead of usermsg

			sender := envelope.Sender

			switch msg := message.(type) {
			case *actor.Terminated:
				rt := &remoteTerminate{
					Watchee: msg.Who,
					Watcher: pid,
				}
				endpointManagerPID.Tell(rt)
			case actor.SystemMessage:
				ref, _ := actor.ProcessRegistry.GetLocal(pid.Id)
				ref.SendSystemMessage(pid, msg)
			default:
				pid.Request(message, sender)
			}
		}
	}
}

func (s *endpointReader) suspend(toSuspend bool) {
	s.suspended = toSuspend
}
