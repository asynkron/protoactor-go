package remoting

import "github.com/AsynkronIT/protoactor/languages/golang/src/actor"

func remoteHandler(pid *actor.PID) (actor.ActorRef, bool) {
	ref := newRemoteActorRef(pid)
	return ref, true
}
