package remoting

import (
	"log"

	"github.com/AsynkronIT/gam/actor"
	"github.com/gogo/protobuf/proto"
)

type remoteActorRef struct {
	pid *actor.PID
	actor.ActorRef
}

func newRemoteActorRef(pid *actor.PID) actor.ActorRef {
	return &remoteActorRef{
		pid: pid,
	}
}

func (ref *remoteActorRef) Tell(pid *actor.PID, message interface{}) {
	switch msg := message.(type) {
	case proto.Message:
		envelope, _ := packMessage(msg, ref.pid, nil)
		endpointManagerPID.Tell(envelope)
	default:
		log.Printf("[REMOTING] failed, trying to send non Proto %v message to %v", msg, ref.pid)
	}
}

func (ref *remoteActorRef) Ask(pid *actor.PID, message interface{}, sender *actor.PID) {
	switch msg := message.(type) {
	case proto.Message:
		envelope, _ := packMessage(msg, ref.pid, sender)
		endpointManagerPID.Tell(envelope)
	default:
		log.Printf("[REMOTING] failed, trying to send non Proto %v message to %v", msg, ref.pid)
	}
}
