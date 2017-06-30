package remote

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/gogo/protobuf/proto"
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
	SendMessage(pid, msg, sender, defaultSerializerID)
}

func SendMessage(pid *actor.PID, message interface{}, sender *actor.PID, serializerID int) {
	switch msg := message.(type) {
	case proto.Message:

		rd := &remoteDeliver{
			message:      msg,
			sender:       sender,
			target:       pid,
			serializerID: serializerID,
		}

		endpointManagerPID.Tell(rd)
	default:
		plog.Error("failed, trying to send non Proto message", log.TypeOf("type", msg), log.Stringer("pid", pid))
	}
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
		SendMessage(pid, message, nil, defaultSerializerID)
	}
}

func (ref *process) Stop(pid *actor.PID) {
	ref.SendSystemMessage(pid, stopMessage)
}

type remoteDeliver struct {
	message      proto.Message
	target       *actor.PID
	sender       *actor.PID
	serializerID int
}
