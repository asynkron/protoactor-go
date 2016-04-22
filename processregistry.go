package gam

import "sync/atomic"

var node = "nonnode"
var host = "nonhost"
var processDirectory = make(map[uint64]ActorRef)
var sequenceID uint64

func registerPID(actorRef ActorRef) *PID {
	id := atomic.AddUint64(&sequenceID, 1)

	pid := PID{
		Node: node,
		Host: host,
		Id:   id,
	}

	processDirectory[pid.Id] = actorRef
	return &pid
}

func FromPID(pid *PID) (ActorRef, bool) {
	if pid.Host != host || pid.Node != node {
		panic("Unknown host or node")
		return deadLetter, false
	}
	ref, ok := processDirectory[pid.Id]
	if !ok {
		panic("Unknown PID")
		return deadLetter, false
	}
	return ref, true
}
