package actor

import "time"

type MessageInvoker interface {
	InvokeSystemMessage(SystemMessage)
	InvokeUserMessage(interface{})
}

type Context interface {
	// Watch registers the actor as a monitor for the specified PID
	Watch(*PID)

	// Unwatch unregisters the actor as a monitor for the specified PID
	Unwatch(*PID)

	// Message returns the current message to be processed
	Message() interface{}

	// SetReceiveTimeout sets the inactivity timeout, after which a ReceiveTimeout message will be sent to the actor.
	// A duration of less than 1ms will disable the inactivity timer.
	SetReceiveTimeout(d time.Duration)

	// ReceiveTimeout returns the current timeout
	ReceiveTimeout() time.Duration

	// Sender returns the PID of actor that sent currently processed message
	Sender() *PID

	// Become replaces the actors current Receive handler with a new handler
	Become(Receive)

	// BecomeStacked pushes a new Receive handler on the current handler stack
	BecomeStacked(Receive)

	// UnbecomeStacked reverts to the previous Receive handler
	UnbecomeStacked()

	// Self returns the PID for the current actor
	Self() *PID

	// Parent returns the PID for the current actors parent
	Parent() *PID

	// Spawn spawns a child actor using the given Props
	Spawn(Props) *PID

	// SpawnNamed spawns a named child actor using the given Props
	SpawnNamed(Props, string) *PID

	// Returns a slice of the current actors children
	Children() []*PID

	// Next performs the next middleware or base Receive handler
	Next()

	// Receive processes a custom user message synchronously
	Receive(interface{})

	// Stash stashes the current message on a stack for reprocessing when the actor restarts
	Stash()

	// Respond sends a response to the to the current `Sender`
	Respond(response interface{})

	// Actor returns the actor associated with this context
	Actor() Actor
}
