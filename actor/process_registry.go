package actor

import (
	"strconv"
	"sync"
	"sync/atomic"
)

type HostResolver func(*PID) (ActorRef, bool)
type ProcessRegistryValue struct {
	Host           string
	LocalPids      map[string]ActorRef //maybe this should be replaced with something lockfree like ctrie instead
	RemoteHandlers []HostResolver
	SequenceID     uint64
	rw             sync.RWMutex
}

var ProcessRegistry = &ProcessRegistryValue{
	Host:           "nonhost",
	LocalPids:      make(map[string]ActorRef),
	RemoteHandlers: make([]HostResolver, 0),
}

func (pr *ProcessRegistryValue) RegisterHostResolver(handler HostResolver) {
	pr.RemoteHandlers = append(pr.RemoteHandlers, handler)
}

func (pr *ProcessRegistryValue) getAutoId() string {
	id := strconv.FormatUint(atomic.AddUint64(&pr.SequenceID, 1), 16)
	return id
}

func (pr *ProcessRegistryValue) registerPID(actorRef ActorRef, id string) (*PID, bool) {

	pid := PID{
		Host: pr.Host,
		Id:   id,
	}

	pr.rw.Lock()
	defer pr.rw.Unlock()
	_, found := pr.LocalPids[pid.Id]
	if found {
		return &pid, false
	}
	pr.LocalPids[pid.Id] = actorRef
	return &pid, true
}

func (pr *ProcessRegistryValue) unregisterPID(pid *PID) {
	pr.rw.Lock()
	defer pr.rw.Unlock()
	delete(pr.LocalPids, pid.Id)
}

func (pr *ProcessRegistryValue) fromPID(pid *PID) (ActorRef, bool) {
	if pid.Host != pr.Host {
		for _, handler := range pr.RemoteHandlers {
			ref, ok := handler(pid)
			if ok {
				return ref, true
			}
		}
		//panic("Unknown host or node")
		return deadLetter, false
	}
	pr.rw.RLock()
	defer pr.rw.RUnlock()
	ref, ok := pr.LocalPids[pid.Id]
	if !ok {
		//panic("Unknown PID")
		return deadLetter, false
	}
	return ref, true
}
