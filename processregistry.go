package gam

import "sync/atomic"

type ProcessRegistry struct {
	Node       string
	Host       string
	LocalPids  map[uint64]ActorRef
	SequenceID uint64
}

var GlobalProcessRegistry = ProcessRegistry{
	Node:      "nonnode",
	Host:      "nonhost",
	LocalPids: make(map[uint64]ActorRef),
}

func (pr ProcessRegistry) RegisterPID(actorRef ActorRef) *PID {
	id := atomic.AddUint64(&pr.SequenceID, 1)

	pid := PID{
		Node: pr.Node,
		Host: pr.Host,
		Id:   id,
	}

	pr.LocalPids[pid.Id] = actorRef
	return &pid
}

func (pr ProcessRegistry) FromPID(pid *PID) (ActorRef, bool) {
	if pid.Host != pr.Host || pid.Node != pr.Node {
		panic("Unknown host or node")
		return deadLetter, false
	}
	ref, ok := pr.LocalPids[pid.Id]
	if !ok {
		panic("Unknown PID")
		return deadLetter, false
	}
	return ref, true
}
