package remoting

import "github.com/AsynkronIT/protoactor-go/actor"

func remoteHandler(pid *actor.PID) (actor.ActorRef, bool) {
	ref := newRemoteActorRef(pid)
	return ref, true
}
