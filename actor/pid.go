package actor

import (
	"log"
	"reflect"
	"strings"
	"time"
)

//Tell a message to a given PID
func (pid *PID) Tell(message interface{}) {
	ref, _ := ProcessRegistry.get(pid)
	ref.SendUserMessage(pid, message, nil)
}

//Ask a message to a given PID
func (pid *PID) Request(message interface{}, respondTo *PID) {
	ref, _ := ProcessRegistry.get(pid)
	ref.SendUserMessage(pid, message, respondTo)
}

//RequestFuture sends a message to a given PID and returns a Future
func (pid *PID) RequestFuture(message interface{}, timeout time.Duration) *Future {
	ref, ok := ProcessRegistry.get(pid)
	if !ok {
		log.Fatalf("[ACTOR] Failed to register future actor with id %v", pid.Id)
	}

	future := NewFuture(timeout)
	ref.SendUserMessage(pid, message, future.PID())
	return future
}

func (pid *PID) sendSystemMessage(message SystemMessage) {
	ref, _ := ProcessRegistry.get(pid)
	ref.SendSystemMessage(pid, message)
}

func (pid *PID) StopFuture() *Future {
	ref, _ := ProcessRegistry.get(pid)

	future := NewFuture(10 * time.Second)

	ref, ok := ref.(*LocalActorRef)
	if !ok {
		log.Fatalf("[ACTOR] Trying to stop non local actorref %s", reflect.TypeOf(ref))
	}

	ref.Watch(future.PID())

	ref.Stop(pid)

	return future
}

//Stop the given PID
func (pid *PID) Stop() {
	ref, _ := ProcessRegistry.get(pid)
	ref.Stop(pid)
}

func pidFromKey(key string, p *PID) {
	i := strings.IndexByte(key, ':')
	if i == -1 {
		p.Host = ProcessRegistry.Host
		p.Id = key
	} else {
		p.Host = key[:i]
		p.Id = key[i+1:]
	}
}

func (pid *PID) key() string {
	if pid.Host == ProcessRegistry.Host {
		return pid.Id
	}
	return pid.Host + ":" + pid.Id
}

func (pid *PID) Empty() bool {
	return pid.Host == "" && pid.Id == ""
}

func (pid *PID) String() string {
	return pid.Host + "/" + pid.Id
}

//NewPID returns a new instance of the PID struct
func NewPID(host, id string) *PID {
	return &PID{
		Host: host,
		Id:   id,
	}
}

//NewLocalPID returns a new instance of the PID struct with the host preset
func NewLocalPID(id string) *PID {
	return &PID{
		Host: ProcessRegistry.Host,
		Id:   id,
	}
}
