package remoting

import (
	"log"

	"github.com/AsynkronIT/gam/remoting/messages"
)

type server struct{}

func (s *server) Receive(stream messages.Remoting_ReceiveServer) error {
	for {
		batch, err := stream.Recv()
		if err != nil {
			log.Println("[REMOTING] Endpoint reader failed to read")
			return err
		}
		for _, envelope := range batch.Envelopes {
			pid := envelope.Target
			message := unpackMessage(envelope)
			//if message is system message send it as sysmsg instead of usermsg

			sender := envelope.Sender
			pid.TellWithSender(message, sender)
		}
	}
}
