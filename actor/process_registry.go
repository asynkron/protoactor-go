package actor

import (
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
	localHost = "nonhost"

	ProcessRegistry = &ProcessRegistryValue{
		Host:      localHost,
		LocalPids: cmap.New(),
	}
)

type HostResolver func(*PID) (ActorRef, bool)

func (pr *ProcessRegistryValue) RegisterHostResolver(handler HostResolver) {
	pr.RemoteHandlers = append(pr.RemoteHandlers, handler)
}

const (
	digits = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ~+"
)

func uint64ToId(u uint64) string {
	var buf [13]byte
	i := 13
	// base is power of 2: use shifts and masks instead of / and %
	for u >= 64 {
		i--
		buf[i] = digits[uintptr(u)&0x3f]
		u >>= 6
	}
	// u < base
	i--
	buf[i] = digits[uintptr(u)]
	i--
	buf[i] = '$'

	return string(buf[i:])
}

func (pr *ProcessRegistryValue) getAutoId() string {
	counter := atomic.AddUint64(&pr.SequenceID, 1)
	return uint64ToId(counter)
}

func (pr *ProcessRegistryValue) add(actorRef ActorRef, id string) (*PID, bool) {

	pid := PID{
		Host: pr.Host,
		Id:   id,
	}

	found := pr.LocalPids.SetIfAbsent(pid.Id, actorRef)
	return &pid, found
}

func (pr *ProcessRegistryValue) remove(pid *PID) {
	pr.LocalPids.Remove(pid.Id)
}

func (pr *ProcessRegistryValue) get(pid *PID) (ActorRef, bool) {
	if pid == nil {
		panic("Pid may not be nil")
	}
	if pid.Host != localHost && pid.Host != pr.Host {
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
		return deadLetter, false
	}
	return ref.(ActorRef), true
}
