package remote

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/log"
)

type server struct{}

func (s *server) Receive(stream Remoting_ReceiveServer) error {
	for {
		batch, err := stream.Recv()
		if err != nil {
			plog.Debug("EndpointReader failed to read", log.Error(err))
			return err
		}
		for _, envelope := range batch.Envelopes {
			targetName := batch.TargetNames[envelope.Target]
			pid := actor.NewLocalPID(targetName)
			message := deserialize(envelope, batch.TypeNames[envelope.TypeId])
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
