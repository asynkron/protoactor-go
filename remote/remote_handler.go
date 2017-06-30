package remote

import "github.com/AsynkronIT/protoactor-go/actor"

func remoteHandler(pid *actor.PID) (actor.Process, bool) {
	ref := newProcess(pid)
	return ref, true
}
