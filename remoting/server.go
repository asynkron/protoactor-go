package remoting

type server struct{}

func (s *server) Receive(stream Remoting_ReceiveServer) error {
	for {
		batch, err := stream.Recv()
		if err != nil {
			return err
		}
		for _, envelope := range batch.Envelopes {
			pid := envelope.Target
			message := UnpackMessage(envelope)
			pid.Tell(message)
		}
	}
}
