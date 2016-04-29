package remoting

import (
	"log"

	"github.com/gogo/protobuf/proto"
	"github.com/rogeralsing/gam/actor"
)

type RemoteActorRef struct {
	pid *actor.PID
}

func newRemoteActorRef(pid *actor.PID) actor.ActorRef {
	return &RemoteActorRef{
		pid: pid,
	}
}

func (ref *RemoteActorRef) Tell(message interface{}) {
	switch msg := message.(type) {
	case proto.Message:
		envelope, _ := PackMessage(msg, ref.pid)
		endpointManagerPID.Tell(envelope)
	default:
		log.Printf("failed, trying to send non Proto %v message to %v", msg, ref.pid)
	}
}

func (ref *RemoteActorRef) SendSystemMessage(message actor.SystemMessage) {}

func (ref *RemoteActorRef) Stop() {}
