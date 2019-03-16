package actor

type deadLetterProcess struct{}

var (
	deadLetter Process = &deadLetterProcess{}
)

type DeadLetter struct {
	PID     *PID
	Message interface{}
	Sender  *PID
}

func (*deadLetterProcess) SendUserMessage(pid *PID, message interface{}, sender *PID) {
	EventStream.Publish(&DeadLetter{
		PID:     pid,
		Message: message,
		Sender:  sender,
	})
}

func (*deadLetterProcess) SendSystemMessage(pid *PID, message SystemMessage) {
	EventStream.Publish(&DeadLetter{
		PID:     pid,
		Message: message,
	})
}
