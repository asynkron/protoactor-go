package actor

import "errors"

var (
	ErrNameExists = errors.New("spawn: name exists")
)

type Spawner func(id string, props Props, parent *PID) (*PID, error)

// DefaultSpawner conforms to Spawner and is used to spawn a local actor
var DefaultSpawner Spawner = spawn

// Spawn starts a new actor with an unique id
func Spawn(props Props) *PID {
	pid, _ := props.spawn(ProcessRegistry.NextId(), nil)
	return pid
}

// SpawnNamed starts a new actor based on props
//
// if name exists, error will be ErrNameExists
func SpawnNamed(props Props, name string) (*PID, error) {
	return props.spawn(name, nil)
}

func spawn(id string, props Props, parent *PID) (*PID, error) {
	cell := newLocalContext(props.actorProducer, props.Supervisor(), props.middlewareChain, parent)
	mailbox := props.ProduceMailbox()
	var ref Process = &localProcess{mailbox: mailbox}
	pid, absent := ProcessRegistry.Add(ref, id)
	if !absent {
		return pid, ErrNameExists
	}

	pid.p = &ref
	cell.self = pid
	mailbox.RegisterHandlers(cell, props.Dispatcher())
	mailbox.PostSystemMessage(startedMessage)

	return pid, nil
}
