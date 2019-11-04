package actor

import (
	"github.com/otherview/protoactor-go/eventstream"
	"github.com/otherview/protoactor-go/log"
)

type deadLetterProcess struct{}

var (
	deadLetter           Process = &deadLetterProcess{}
	deadLetterSubscriber *eventstream.Subscription
)

func init() {
	deadLetterSubscriber = eventstream.Subscribe(func(msg interface{}) {
		if deadLetter, ok := msg.(*DeadLetterEvent); ok {
			plog.Debug("[DeadLetter]", log.Stringer("pid", deadLetter.PID), log.Message(deadLetter.Message), log.Stringer("sender", deadLetter.Sender))
		}
	})

	// this subscriber may not be deactivated.
	// it ensures that Watch commands that reach a stopped actor gets a Terminated message back.
	// This can happen if one actor tries to Watch a PID, while another thread sends a Stop message.
	eventstream.Subscribe(func(msg interface{}) {
		if deadLetter, ok := msg.(*DeadLetterEvent); ok {
			if m, ok := deadLetter.Message.(*Watch); ok {
				// we know that this is a local actor since we get it on our own event stream, thus the address is not terminated
				m.Watcher.sendSystemMessage(&Terminated{AddressTerminated: false, Who: deadLetter.PID})
			}
		}
	})
}

// A DeadLetterEvent is published via event.Publish when a message is sent to a nonexistent PID
type DeadLetterEvent struct {
	PID     *PID        // The invalid process, to which the message was sent
	Message interface{} // The message that could not be delivered
	Sender  *PID        // the process that sent the Message
}

func (*deadLetterProcess) SendUserMessage(pid *PID, message interface{}) {
	_, msg, sender := UnwrapEnvelope(message)
	eventstream.Publish(&DeadLetterEvent{
		PID:     pid,
		Message: msg,
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
