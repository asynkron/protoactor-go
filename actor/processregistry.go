package actor

import (
	"sync"
	"sync/atomic"
)
import "strconv"

type HostResolver func(*PID) (ActorRef, bool)
type ProcessRegistryValue struct {
	Host           string
	LocalPids      map[string]ActorRef  //maybe this should be replaced with something lockfree like ctrie instead
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

func (pr *ProcessRegistryValue) registerPID(actorRef ActorRef) *PID {
	id := atomic.AddUint64(&pr.SequenceID, 1)

	pid := PID{
		Host: pr.Host,
		Id:   strconv.FormatUint(id, 16),
	}

	pr.rw.Lock()
	defer pr.rw.Unlock()
	pr.LocalPids[pid.Id] = actorRef
	return &pid
}

func (pr *ProcessRegistryValue) unregisterPID(pid *PID) {
	pr.rw.Lock()
	defer pr.rw.Unlock()
	delete(pr.LocalPids,pid.Id)
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

func (pr *ProcessRegistryValue) Register(name string, pid *PID) {
	ref, _ := pr.fromPID(pid)
	pr.LocalPids[name] = ref
}
