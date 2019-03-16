package actor

import (
	"log"
	"strings"
	"time"
)

//Tell a message to a given PID
func (pid *PID) Tell(message interface{}) {
	ref, _ := ProcessRegistry.Get(pid)
	ref.SendUserMessage(pid, message, nil)
}

//Ask a message to a given PID
func (pid *PID) Request(message interface{}, respondTo *PID) {
	ref, _ := ProcessRegistry.Get(pid)
	ref.SendUserMessage(pid, message, respondTo)
}

//RequestFuture sends a message to a given PID and returns a Future
func (pid *PID) RequestFuture(message interface{}, timeout time.Duration) *Future {
	ref, ok := ProcessRegistry.Get(pid)
	if !ok {
		log.Printf("[ACTOR] RequestFuture for missing local PID '%v'", pid.String())
	}

	future := NewFuture(timeout)
	ref.SendUserMessage(pid, message, future.PID())
	return future
}

func (pid *PID) sendSystemMessage(message SystemMessage) {
	ref, _ := ProcessRegistry.Get(pid)
	ref.SendSystemMessage(pid, message)
}

func (pid *PID) StopFuture() *Future {
	future := NewFuture(10 * time.Second)

	pid.sendSystemMessage(&Watch{Watcher: future.PID()})
	pid.Stop()

	return future
}

//Stop the given PID
func (pid *PID) Stop() {
	ref, _ := ProcessRegistry.Get(pid)
	ref.SendSystemMessage(pid, stopMessage)
}

func pidFromKey(key string, p *PID) {
	i := strings.IndexByte(key, '#')
	if i == -1 {
		p.Address = ProcessRegistry.Address
		p.Id = key
	} else {
		p.Address = key[:i]
		p.Id = key[i+1:]
	}
}

func (pid *PID) key() string {
	if pid.Address == ProcessRegistry.Address {
		return pid.Id
	}
	return pid.Address + "#" + pid.Id
}

func (pid *PID) Empty() bool {
	return pid.Address == "" && pid.Id == ""
}

func (pid *PID) String() string {
	return pid.Address + "/" + pid.Id
}

//NewPID returns a new instance of the PID struct
func NewPID(address, id string) *PID {
	return &PID{
		Address: address,
		Id:      id,
	}
}

//NewLocalPID returns a new instance of the PID struct with the address preset
func NewLocalPID(id string) *PID {
	return &PID{
		Address: ProcessRegistry.Address,
		Id:      id,
	}
}
