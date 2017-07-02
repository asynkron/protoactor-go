package remote

import (
	"github.com/AsynkronIT/protoactor-go/actor"
)

type process struct {
	pid *actor.PID
}

func newProcess(pid *actor.PID) actor.Process {
	return &process{
		pid: pid,
	}
}

func (ref *process) SendUserMessage(pid *actor.PID, message interface{}) {
	msg, sender := actor.UnwrapEnvelope(message)
	SendMessage(pid, msg, sender, -1)
}

func SendMessage(pid *actor.PID, message interface{}, sender *actor.PID, serializerID int32) {
	rd := &remoteDeliver{
		message:      message,
		sender:       sender,
		target:       pid,
		serializerID: serializerID,
	}

	endpointManagerPID.Tell(rd)
}

func (ref *process) SendSystemMessage(pid *actor.PID, message interface{}) {

	//intercept any Watch messages and direct them to the endpoint manager
	switch msg := message.(type) {
	case *actor.Watch:
		rw := &remoteWatch{
			Watcher: msg.Watcher,
			Watchee: pid,
		}
		endpointManagerPID.Tell(rw)
	case *actor.Unwatch:
		ruw := &remoteUnwatch{
			Watcher: msg.Watcher,
			Watchee: pid,
		}
		endpointManagerPID.Tell(ruw)
	default:
		SendMessage(pid, message, nil, -1)
	}
}

func (ref *process) Stop(pid *actor.PID) {
	ref.SendSystemMessage(pid, stopMessage)
}

type remoteDeliver struct {
	message      interface{}
	target       *actor.PID
	sender       *actor.PID
	serializerID int32
}
