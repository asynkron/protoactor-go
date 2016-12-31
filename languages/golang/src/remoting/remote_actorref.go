package remoting

import (
	"log"
	"reflect"

	"github.com/AsynkronIT/protoactor/languages/golang/src/actor"
	"github.com/gogo/protobuf/proto"
)

type remoteActorRef struct {
	pid *actor.PID
}

func newRemoteActorRef(pid *actor.PID) actor.ActorRef {
	return &remoteActorRef{
		pid: pid,
	}
}

func (ref *remoteActorRef) SendUserMessage(pid *actor.PID, message interface{}, sender *actor.PID) {
	ref.send(pid, message, sender)
}

func (ref *remoteActorRef) send(pid *actor.PID, message interface{}, sender *actor.PID) {
	switch msg := message.(type) {
	case proto.Message:
		envelope, _ := serialize(msg, ref.pid, sender)
		endpointManagerPID.Tell(envelope)
	default:
		log.Printf("[REMOTING] failed, trying to send non Proto %s message to %v", reflect.TypeOf(msg), ref.pid)
	}
}

func (ref *remoteActorRef) SendSystemMessage(pid *actor.PID, message actor.SystemMessage) {
	ref.send(pid, message, nil)
}

func (ref *remoteActorRef) Stop(pid *actor.PID) {
	ref.SendSystemMessage(pid, &actor.Stop{})
}

func (ref *remoteActorRef) Watch(pid *actor.PID) {
	ref.SendSystemMessage(pid, &actor.Watch{Watcher: pid})
}

func (ref *remoteActorRef) Unwatch(pid *actor.PID) {
	ref.SendSystemMessage(pid, &actor.Unwatch{Watcher: pid})
}
