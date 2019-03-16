package remote

import (
	"log"
	"reflect"

	"github.com/AsynkronIT/protoactor-go/actor"
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
		envelope, _ := serialize(msg, pid, sender)
		endpointManagerPID.Tell(envelope)
	default:
		log.Printf("[REMOTING] failed, trying to send non Proto %s message to %v", reflect.TypeOf(msg), pid)
	}
}

func (ref *remoteProcess) SendSystemMessage(pid *actor.PID, message actor.SystemMessage) {

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
