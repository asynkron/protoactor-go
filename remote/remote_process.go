package remote

import (
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/gogo/protobuf/proto"
)

type remoteProcess struct {
	pid *actor.PID
}

func newRemoteProcess(pid *actor.PID) actor.Process {
	return &remoteProcess{
		pid: pid,
	}
}

func (ref *remoteProcess) SendUserMessage(pid *actor.PID, message interface{}, sender *actor.PID) {
	sendRemoteMessage(pid, message, sender)
}

func sendRemoteMessage(pid *actor.PID, message interface{}, sender *actor.PID) {
	switch msg := message.(type) {
	case proto.Message:

		rd := &remoteDeliver{
			message: msg,
			sender:  sender,
			target:  pid,
		}
		endpointManagerPID.Tell(rd)
	default:
		plog.Error("failed, trying to send non Proto message", log.TypeOf("type", msg), log.Stringer("pid", pid))
	}
}

func (ref *remoteProcess) SendSystemMessage(pid *actor.PID, message interface{}) {

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
		sendRemoteMessage(pid, message, nil)
	}
}

func (ref *remoteProcess) Stop(pid *actor.PID) {
	ref.SendSystemMessage(pid, stopMessage)
}

type remoteDeliver struct {
	message proto.Message
	target  *actor.PID
	sender  *actor.PID
}
