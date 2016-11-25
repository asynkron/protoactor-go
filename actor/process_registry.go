package actor

import (
	"strconv"
	"sync/atomic"

	cmap "github.com/orcaman/concurrent-map"
)

type ProcessRegistryValue struct {
	Host           string
	LocalPids      cmap.ConcurrentMap
	RemoteHandlers []HostResolver
	SequenceID     uint64
}

var (
	ProcessRegistry = &ProcessRegistryValue{
		Host:           "nonhost",
		LocalPids:      cmap.New(),
		RemoteHandlers: make([]HostResolver, 0),
	}
)

type HostResolver func(*PID) (ActorRef, bool)

func (pr *ProcessRegistryValue) RegisterHostResolver(handler HostResolver) {
	pr.RemoteHandlers = append(pr.RemoteHandlers, handler)

}

func (pr *ProcessRegistryValue) getAutoId() string {
	id := strconv.FormatUint(atomic.AddUint64(&pr.SequenceID, 1), 16)
	return id
}

func (pr *ProcessRegistryValue) add(actorRef ActorRef, id string) (*PID, bool) {

	pid := PID{
		Host: pr.Host,
		Id:   id,
	}

	_, found := pr.LocalPids.Get(pid.Id)
	if found {
		return &pid, false
	}
	pr.LocalPids.Set(pid.Id, actorRef)
	return &pid, true
}

func (pr *ProcessRegistryValue) remove(pid *PID) {
	pr.LocalPids.Remove(pid.Id)
}

func (pr *ProcessRegistryValue) get(pid *PID) (ActorRef, bool) {
	if pid.Host != "nonhost" && pid.Host != pr.Host {
		for _, handler := range pr.RemoteHandlers {
			ref, ok := handler(pid)
			if ok {
				return ref, true
			}
		}
		return deadLetter, false
	}
	ref, ok := pr.LocalPids.Get(pid.Id)
	if !ok {
		//panic("Unknown PID")
		return deadLetter, false
	}
	return ref.(ActorRef), true
}
