package remote

import "github.com/AsynkronIT/protoactor-go/actor"

func remoteHandler(pid *actor.PID) (actor.Process, bool) {
	ref := newRemoteProcess(pid)
	return ref, true
}
