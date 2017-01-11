package remoting

import (
	"log"

	"github.com/AsynkronIT/protoactor-go/actor"
)

type server struct{}

func (s *server) Receive(stream Remoting_ReceiveServer) error {
	for {
		batch, err := stream.Recv()
		if err != nil {
			log.Printf("[REMOTING] Endpoint reader failed to read. %v", err)
			return err
		}
		for _, envelope := range batch.Envelopes {
			pid := envelope.Target
			message := deserialize(envelope)
			//if message is system message send it as sysmsg instead of usermsg

			sender := envelope.Sender

			switch msg := message.(type) {
			case *actor.Terminated:
				rt := &remoteTerminate{
					Watchee: msg.Who,
					Watcher: pid,
				}
				endpointManagerPID.Tell(rt)
			default:
				//TODO: this only works for user messages
				// system messages needs to be sent correctly
				pid.Request(message, sender)
			}
		}
	}
}
