package actor

import "time"

type Context interface {
	// Watch registers the actor as a monitor for the specified PID
	Watch(*PID)

	// Unwatch unregisters the actor as a monitor for the specified PID
	Unwatch(*PID)

	// Message returns the current message to be processed
	Message() interface{}

	// SetReceiveTimeout sets the inactivity timeout, after which a ReceiveTimeout message will be sent to the actor.
	// A duration of less than 1ms will disable the inactivity timer.
	//
	// If a message is received before the duration d, the timer will be reset, unless the message conforms
	SetReceiveTimeout(d time.Duration)

	// ReceiveTimeout returns the current timeout
	ReceiveTimeout() time.Duration

	// Sender returns the PID of actor that sent currently processed message
	Sender() *PID

	// SetBehavior replaces the actors current behavior stack with the new behavior
	SetBehavior(behavior ReceiveFunc)

	// PushBehavior pushes the current behavior on the stack and sets the current Receive handler to the new behavior
	PushBehavior(behavior ReceiveFunc)

	// PopBehavior reverts to the previous Receive handler
	PopBehavior()

	// Self returns the PID for the current actor
	Self() *PID

	// Parent returns the PID for the current actors parent
	Parent() *PID

	// Spawn spawns a child actor using the given Props
	Spawn(Props) *PID

	// SpawnNamed spawns a named child actor using the given Props
	//
	// ErrNameExists will be returned if id already exists
	SpawnNamed(props Props, id string) (*PID, error)

	// Returns a slice of the current actors children
	Children() []*PID

	// Stash stashes the current message on a stack for reprocessing when the actor restarts
	Stash()

	// Respond sends a response to the to the current `Sender`
	Respond(response interface{})

	// Actor returns the actor associated with this context
	Actor() Actor
}
