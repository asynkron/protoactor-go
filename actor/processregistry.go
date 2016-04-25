package actor

import "sync/atomic"
import "strconv"

type RemoteHandler func(*PID) (ActorRef, bool)
type ProcessRegistry struct {
	Host           string
	LocalPids      map[string]ActorRef
	RemoteHandlers []RemoteHandler
	SequenceID     uint64
}

var GlobalProcessRegistry = &ProcessRegistry{
	Host:           "nonhost",
	LocalPids:      make(map[string]ActorRef),
	RemoteHandlers: make([]RemoteHandler, 0),
}

func (pr *ProcessRegistry) AddRemoteHandler(handler RemoteHandler) {
	pr.RemoteHandlers = append(pr.RemoteHandlers, handler)
}

func (pr *ProcessRegistry) RegisterPID(actorRef ActorRef) *PID {
	id := atomic.AddUint64(&pr.SequenceID, 1)

	pid := PID{
		Host: pr.Host,
		Id:   strconv.FormatUint(id, 16),
	}

	pr.LocalPids[pid.Id] = actorRef
	return &pid
}

func (pr *ProcessRegistry) FromPID(pid *PID) (ActorRef, bool) {
	if pid.Host != pr.Host {
		for _, handler := range pr.RemoteHandlers {
			ref, ok := handler(pid)
			if ok {
				return ref, true
			}
		}
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

func (pr *ProcessRegistry) Register(name string, pid *PID) {
	ref, _ := pr.FromPID(pid)
	pr.LocalPids[name] = ref
}
