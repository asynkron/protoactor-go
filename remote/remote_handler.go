package remote

import "github.com/awevoke/protoactor-go/actor"

// func remoteHandler(pid *actor.PID) (actor.Process, bool) {
// 	ref := newProcess(pid, nil)
// 	return ref, true
// }

func (r *Remote) remoteHandler(pid *actor.PID) (actor.Process, bool) {
	ref := newProcess(pid, r)
	return ref, true
}
