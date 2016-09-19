package remoting

import "github.com/AsynkronIT/gam/actor"

func remoteHandler(pid *actor.PID) (actor.ActorRef, bool) {
	ref := newRemoteActorRef(pid)
	return ref, true
}
