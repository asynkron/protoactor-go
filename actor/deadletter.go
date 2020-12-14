package actor

import (
	"github.com/AsynkronIT/protoactor-go/log"
)

type deadLetterProcess struct {
	actorSystem *ActorSystem
}

func NewDeadLetter(actorSystem *ActorSystem) *deadLetterProcess {

	dp := &deadLetterProcess{
		actorSystem: actorSystem,
	}

	actorSystem.ProcessRegistry.Add(dp, "deadletter")
	_ = actorSystem.EventStream.Subscribe(func(msg interface{}) {
		if deadLetter, ok := msg.(*DeadLetterEvent); ok {
			plog.Debug("[DeadLetter]", log.Stringer("pid", deadLetter.PID), log.TypeOf("msg", deadLetter.Message), log.Stringer("sender", deadLetter.Sender))
			// send back a response instead of timeout.
			if deadLetter.Sender != nil {
				actorSystem.Root.Send(deadLetter.Sender, &DeadLetterResponse{})
			}
		}
	})

	// this subscriber may not be deactivated.
	// it ensures that Watch commands that reach a stopped actor gets a Terminated message back.
	// This can happen if one actor tries to Watch a PID, while another thread sends a Stop message.
	actorSystem.EventStream.Subscribe(func(msg interface{}) {
		if deadLetter, ok := msg.(*DeadLetterEvent); ok {
			if m, ok := deadLetter.Message.(*Watch); ok {
				// we know that this is a local actor since we get it on our own event stream, thus the address is not terminated
				m.Watcher.sendSystemMessage(actorSystem, &Terminated{AddressTerminated: false, Who: deadLetter.PID})
			}
		}
	})

	return dp
}

// A DeadLetterEvent is published via event.Publish when a message is sent to a nonexistent PID
type DeadLetterEvent struct {
	PID     *PID        // The invalid process, to which the message was sent
	Message interface{} // The message that could not be delivered
	Sender  *PID        // the process that sent the Message
}

func (dp *deadLetterProcess) SendUserMessage(pid *PID, message interface{}) {
	_, msg, sender := UnwrapEnvelope(message)
	dp.actorSystem.EventStream.Publish(&DeadLetterEvent{
		PID:     pid,
		Message: msg,
		Sender:  sender,
	})
}

func (dp *deadLetterProcess) SendSystemMessage(pid *PID, message interface{}) {
	dp.actorSystem.EventStream.Publish(&DeadLetterEvent{
		PID:     pid,
		Message: message,
	})
}

func (dp *deadLetterProcess) Stop(pid *PID) {
	dp.SendSystemMessage(pid, stopMessage)
}
