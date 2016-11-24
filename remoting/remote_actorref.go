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
	ref.Ask(pid, message, nil)
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

func (ref *remoteActorRef) SendSystemMessage(pid *actor.PID, message actor.SystemMessage) {
	switch msg := message.(type) {
	case *actor.Watch:
		ref.Ask(pid, msg, nil)
	case *actor.Unwatch:
		ref.Ask(pid, msg, nil)
	case *actor.Terminated:
		ref.Ask(pid, msg, nil)
	default:
		log.Printf("[REMOTING] failed, trying to send non Proto %v message to %v", msg, ref.pid)
	}
}

func (ref *remoteActorRef) Stop(pid *actor.PID) {
	ref.SendSystemMessage(pid, &actor.Stop{})
}

func (ref *remoteActorRef) Watch(pid *actor.PID) {
	ref.SendSystemMessage(pid, &actor.Watch{Watcher: pid})
}

func (ref *remoteActorRef) UnWatch(pid *actor.PID) {
	ref.SendSystemMessage(pid, &actor.Unwatch{Watcher: pid})
}
