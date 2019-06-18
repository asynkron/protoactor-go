package actor

import "time"

// Context contains contextual information for actors
type Context interface {
	infoPart
	basePart
	messagePart
	senderPart
	receiverPart
	spawnerPart
	stopperPart
}

type SenderContext interface {
	infoPart
	senderPart
	messagePart
}

type ReceiverContext interface {
	infoPart
	receiverPart
	messagePart
}

type SpawnerContext interface {
	infoPart
	spawnerPart
}

type infoPart interface {
	// Parent returns the PID for the current actors parent
	Parent() *PID

	// Self returns the PID for the current actor
	Self() *PID

	// Actor returns the actor associated with this context
	Actor() Actor
}

type basePart interface {
	// ReceiveTimeout returns the current timeout
	ReceiveTimeout() time.Duration

	// Returns a slice of the actors children
	Children() []*PID

	// Respond sends a response to the to the current `Sender`
	// If the Sender is nil, the actor will panic
	Respond(response interface{})

	// Stash stashes the current message on a stack for reprocessing when the actor restarts
	Stash()

	// Watch registers the actor as a monitor for the specified PID
	Watch(pid *PID)

	// Unwatch unregisters the actor as a monitor for the specified PID
	Unwatch(pid *PID)

	// SetReceiveTimeout sets the inactivity timeout, after which a ReceiveTimeout message will be sent to the actor.
	// A duration of less than 1ms will disable the inactivity timer.
	//
	// If a message is received before the duration d, the timer will be reset. If the message conforms to
	// the NotInfluenceReceiveTimeout interface, the timer will not be reset
	SetReceiveTimeout(d time.Duration)

	CancelReceiveTimeout()

	// Forward forwards current message to the given PID
	Forward(pid *PID)

	AwaitFuture(f *Future, continuation func(res interface{}, err error))
}

type messagePart interface {
	// Message returns the current message to be processed
	Message() interface{}

	// MessageHeader returns the meta information for the currently processed message
	MessageHeader() ReadonlyMessageHeader
}

type senderPart interface {
	// Sender returns the PID of actor that sent currently processed message
	Sender() *PID

	// Send sends a message to the given PID
	Send(pid *PID, message interface{})

	// Request sends a message to the given PID
	Request(pid *PID, message interface{})

	// Request sends a message to the given PID and also provides a Sender PID
	RequestWithCustomSender(pid *PID, message interface{}, sender *PID)

	// RequestFuture sends a message to a given PID and returns a Future
	RequestFuture(pid *PID, message interface{}, timeout time.Duration) *Future
}

type receiverPart interface {
	Receive(envelope *MessageEnvelope)
}

type spawnerPart interface {
	// Spawn starts a new child actor based on props and named with a unique id
	Spawn(props *Props) *PID

	// SpawnPrefix starts a new child actor based on props and named using a prefix followed by a unique id
	SpawnPrefix(props *Props, prefix string) *PID

	// SpawnNamed starts a new child actor based on props and named using the specified name
	//
	// ErrNameExists will be returned if id already exists
	//
	// Please do not use name sharing same pattern with system actors, for example "YourPrefix$1", "Remote$1", "future$1"
	SpawnNamed(props *Props, id string) (*PID, error)
}

type stopperPart interface {
	// Stop will stop actor immediately regardless of existing user messages in mailbox.
	Stop(pid *PID)

	// StopFuture will stop actor immediately regardless of existing user messages in mailbox, and return its future.
	StopFuture(pid *PID) *Future

	// Poison will tell actor to stop after processing current user messages in mailbox.
	Poison(pid *PID)

	// PoisonFuture will tell actor to stop after processing current user messages in mailbox, and return its future.
	PoisonFuture(pid *PID) *Future
}
