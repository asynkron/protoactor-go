package remote

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/eventstream"
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
	if endpointManager.stopped {
		ref.publishDeadLetter(pid, message)
		return
	}

	header, msg, sender := actor.UnwrapEnvelope(message)
	SendMessage(pid, header, msg, sender, -1)
}

func SendMessage(pid *actor.PID, header actor.ReadonlyMessageHeader, message interface{}, sender *actor.PID, serializerID int32) {
	rd := &remoteDeliver{
		header:       header,
		message:      message,
		sender:       sender,
		target:       pid,
		serializerID: serializerID,
	}

	endpointManager.remoteDeliver(rd)
}

func (ref *process) SendSystemMessage(pid *actor.PID, message interface{}) {
	if endpointManager.stopped {
		ref.publishDeadLetter(pid, message)
		return
	}

	// intercept any Watch messages and direct them to the endpoint manager
	switch msg := message.(type) {
	case *actor.Watch:
		rw := &remoteWatch{
			Watcher: msg.Watcher,
			Watchee: pid,
		}
		endpointManager.remoteWatch(rw)
	case *actor.Unwatch:
		ruw := &remoteUnwatch{
			Watcher: msg.Watcher,
			Watchee: pid,
		}
		endpointManager.remoteUnwatch(ruw)
	default:
		SendMessage(pid, nil, message, nil, -1)
	}
}

func (ref *process) Stop(pid *actor.PID) {
	ref.SendSystemMessage(pid, stopMessage)
}

func (ref *process) publishDeadLetter(receiver *actor.PID, message interface{}) {
	eventstream.Publish(&actor.DeadLetterEvent{
		PID:     receiver,
		Message: message,
		Sender:  ref.pid,
	})
}
