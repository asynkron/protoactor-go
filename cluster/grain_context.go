package cluster

import (
	"time"

	"github.com/asynkron/protoactor-go/actor"
)

type GrainContext interface {
	// Self returns the PID for the current actor
	Self() *actor.PID

	// Children returns a slice of the actors children
	Children() []*actor.PID

	// Watch registers the actor as a monitor for the specified PID
	Watch(pid *actor.PID)

	// Unwatch unregisters the actor as a monitor for the specified PID
	Unwatch(pid *actor.PID)

	// Sender returns the PID of actor that sent currently processed message
	Sender() *actor.PID

	// Message returns the current message to be processed
	Message() interface{}

	// Send sends a message to the given PID
	Send(pid *actor.PID, message interface{})

	// Request sends a message to the given PID and also provides a Sender PID
	Request(pid *actor.PID, message interface{})

	// RequestFuture sends a message to a given PID and returns a Future
	RequestFuture(pid *actor.PID, message interface{}, timeout time.Duration) *actor.Future

	// Spawn starts a new child actor based on props and named with a unique identity
	Spawn(props *actor.Props) *actor.PID

	// SpawnPrefix starts a new child actor based on props and named using a prefix followed by a unique identity
	SpawnPrefix(props *actor.Props, prefix string) *actor.PID

	// SpawnNamed starts a new child actor based on props and named using the specified name
	//
	// ErrNameExists will be returned if identity already exists
	SpawnNamed(props *actor.Props, id string) (*actor.PID, error)

	Identity() string
	Kind() string
	Cluster() *Cluster
}

type grainContextImpl struct {
	actor.Context
	ci      *ClusterIdentity
	cluster *Cluster
}

func (g grainContextImpl) Identity() string {
	return g.ci.Identity
}

func (g grainContextImpl) Kind() string {
	return g.ci.Kind
}

func (g grainContextImpl) Cluster() *Cluster {
	return g.cluster
}

func NewGrainContext(context actor.Context, identity *ClusterIdentity, cluster *Cluster) GrainContext {
	return &grainContextImpl{
		Context: context,
		ci:      identity,
		cluster: cluster,
	}
}
