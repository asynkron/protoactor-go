package remote

import "github.com/otherview/protoactor-go/actor"

func remoteHandler(pid *actor.PID) (actor.Process, bool) {
	ref := newProcess(pid)
	return ref, true
}
