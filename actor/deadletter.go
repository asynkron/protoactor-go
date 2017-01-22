package actor

import (
	"log"

	"github.com/AsynkronIT/protoactor-go/eventstream"
)

type deadLetterProcess struct{}

var (
	deadLetter           Process = &deadLetterProcess{}
	deadLetterSubscriber *eventstream.Subscription
)

func init() {
	deadLetterSubscriber = eventstream.Subscribe(func(msg interface{}) {
		if deadLetter, ok := msg.(*DeadLetterEvent); ok {
			log.Printf("[ACTOR] [DeadLetter] %v got %+v from %v", deadLetter.PID, deadLetter.Message, deadLetter.Sender)
		}
	})
}

// A DeadLetterEvent is published via event.Publish when a message is sent to a nonexistent PID
type DeadLetterEvent struct {
	PID     *PID        // The dead letter process
	Message interface{} // The message that could not be delivered
	Sender  *PID        // the process that sent the Message
}

func (*deadLetterProcess) SendUserMessage(pid *PID, message interface{}, sender *PID) {
	eventstream.Publish(&DeadLetterEvent{
		PID:     pid,
		Message: message,
		Sender:  sender,
	})
}

func (*deadLetterProcess) SendSystemMessage(pid *PID, message interface{}) {
	eventstream.Publish(&DeadLetterEvent{
		PID:     pid,
		Message: message,
	})
}

func (ref *deadLetterProcess) Stop(pid *PID) {
	ref.SendSystemMessage(pid, stopMessage)
}
