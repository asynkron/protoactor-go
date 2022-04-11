package remote

import (
	"github.com/asynkron/protoactor-go/actor"
)

type process struct {
	pid    *actor.PID
	remote *Remote
}

func newProcess(pid *actor.PID, r *Remote) actor.Process {
	return &process{
		pid:    pid,
		remote: r,
	}
}

var _ actor.Process = &process{}

func (ref *process) SendUserMessage(pid *actor.PID, message interface{}) {
	header, msg, sender := actor.UnwrapEnvelope(message)
	ref.remote.SendMessage(pid, header, msg, sender, -1)
}

func (ref *process) SendSystemMessage(pid *actor.PID, message interface{}) {
	// intercept any Watch messages and direct them to the endpoint manager
	switch msg := message.(type) {
	case *actor.Watch:
		rw := &remoteWatch{
			Watcher: msg.Watcher,
			Watchee: pid,
		}
		// endpointManager.remoteWatch(rw)
		ref.remote.edpManager.remoteWatch(rw)
	case *actor.Unwatch:
		ruw := &remoteUnwatch{
			Watcher: msg.Watcher,
			Watchee: pid,
		}
		// endpointManager.remoteUnwatch(ruw)
		ref.remote.edpManager.remoteUnwatch(ruw)
	default:
		ref.remote.SendMessage(pid, nil, message, nil, -1)
	}
}

func (ref *process) Stop(pid *actor.PID) {
	ref.SendSystemMessage(pid, stopMessage)
}
