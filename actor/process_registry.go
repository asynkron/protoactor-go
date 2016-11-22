package actor

import (
	"log"
	"strconv"
	"sync"
	"sync/atomic"

    "github.com/Workiva/go-datastructures/trie/ctrie"
)

type HostResolver func(*PID) (ActorRef, bool)

type ProcessRegistryValue struct {
	Host           string
	LocalPids      *ctrie.Ctrie
	RemoteHandlers []HostResolver
	SequenceID     uint64
}

var ProcessRegistry = &ProcessRegistryValue{
	Host:           "nonhost",
	LocalPids:      ctrie.New(nil),
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

	_, found := pr.LocalPids.Lookup([]byte(pid.Id))
	if found {
		return &pid, false
	}
	pr.LocalPids.Insert([]byte(pid.Id), actorRef)
	return &pid, true
}

func (pr *ProcessRegistryValue) unregisterPID(pid *PID) {
    pr.LocalPids.Remove([]byte(pid.Id))
}

func (pr *ProcessRegistryValue) fromPID(pid *PID) (ActorRef, bool) {
	if pid.Host != "nonhost" && pid.Host != pr.Host {
		for _, handler := range pr.RemoteHandlers {
			ref, ok := handler(pid)
			if ok {
				return ref, true
			}
		}
		//panic("Unknown host or node")
		return deadLetter, false
	}
	ref, ok := pr.LocalPids.Lookup([]byte(pid.Id))
	if !ok {
		//panic("Unknown PID")
		return deadLetter, false
	}

	if original, ok := ref.(ActorRef), ok{
		return original, true
    } else {
		return deadLetter, false
    }

}
