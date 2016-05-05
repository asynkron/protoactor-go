package remoting

import (
	"log"

	"github.com/gogo/protobuf/proto"
	"github.com/rogeralsing/gam/actor"
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

func (ref *remoteActorRef) Tell(message interface{}) {
	switch msg := message.(type) {
	case proto.Message:
		envelope, _ := packMessage(msg, ref.pid)
		endpointManagerPID.Tell(envelope)
	default:
		log.Printf("failed, trying to send non Proto %v message to %v", msg, ref.pid)
	}
}
