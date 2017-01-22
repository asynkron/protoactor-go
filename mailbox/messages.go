package mailbox

type ControlMessage interface {
	ControlMessage()
}

// ResumeMailbox is message sent by the actor system to control the lifecycle of an actor.
//
// This will not be forwarded to the Receive method
type ResumeMailbox struct{}

// SuspendMailbox is message sent by the actor system to control the lifecycle of an actor.
//
// This will not be forwarded to the Receive method
type SuspendMailbox struct{}

func (ResumeMailbox) ControlMessage()  {}
func (SuspendMailbox) ControlMessage() {}
