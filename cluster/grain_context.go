package cluster

import (
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
)

type GrainContext interface {
	// Self returns the PID for the current actor
	Self() *actor.PID

	// Returns a slice of the actors children
	Children() []*actor.PID

	// Watch registers the actor as a monitor for the specified PID
	Watch(pid *actor.PID)

	// Unwatch unregisters the actor as a monitor for the specified PID
	Unwatch(pid *actor.PID)

	// Sender returns the PID of actor that sent currently processed message
	Sender() *actor.PID

	// Message returns the current message to be processed
	Message() interface{}

	// Tell sends a message to the given PID
	Send(pid *actor.PID, message interface{})

	// Request sends a message to the given PID and also provides a Sender PID
	Request(pid *actor.PID, message interface{})

	// RequestFuture sends a message to a given PID and returns a Future
	RequestFuture(pid *actor.PID, message interface{}, timeout time.Duration) *actor.Future

	// Spawn starts a new child actor based on props and named with a unique id
	Spawn(props *actor.Props) *actor.PID

	// SpawnPrefix starts a new child actor based on props and named using a prefix followed by a unique id
	SpawnPrefix(props *actor.Props, prefix string) *actor.PID

	// SpawnNamed starts a new child actor based on props and named using the specified name
	//
	// ErrNameExists will be returned if id already exists
	SpawnNamed(props *actor.Props, id string) (*actor.PID, error)
}
