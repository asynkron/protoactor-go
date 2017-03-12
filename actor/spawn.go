package actor

import (
	"errors"
)

// ErrNameExists is the error used when an existing name is used for spawning an actor.
var ErrNameExists = errors.New("spawn: name exists")

type SpawnFunc func(id string, props *Props, parent *PID) (*PID, error)

// DefaultSpawner conforms to Spawner and is used to spawn a local actor
var DefaultSpawner SpawnFunc = spawn

// Spawn starts a new actor based on props and named with a unique id
func Spawn(props *Props) *PID {
	pid, _ := props.spawn(ProcessRegistry.NextId(), nil)
	return pid
}

// SpawnPrefix starts a new actor based on props and named using a prefix followed by a unique id
func SpawnPrefix(props *Props, prefix string) (*PID, error) {
	return props.spawn(prefix+ProcessRegistry.NextId(), nil)
}

// SpawnNamed starts a new actor based on props and named using the specified name
//
// If name exists, error will be ErrNameExists
func SpawnNamed(props *Props, name string) (*PID, error) {
	return props.spawn(name, nil)
}

func spawn(id string, props *Props, parent *PID) (*PID, error) {
	lp := &localProcess{}
	pid, absent := ProcessRegistry.Add(lp, id)
	if !absent {
		return pid, ErrNameExists
	}

	cell := newLocalContext(props.actorProducer, props.getSupervisor(), props.middlewareChain, props.outboundMiddlewareChain, parent)
	mb := props.produceMailbox(cell, props.getDispatcher())
	lp.mailbox = mb
	var ref Process = lp
	pid.p = &ref
	cell.self = pid
	mb.Start()
	mb.PostSystemMessage(startedMessage)

	return pid, nil
}
