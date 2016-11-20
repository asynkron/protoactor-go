package actor

import (
	"fmt"
	"time"
	"log"
	"reflect"
)

//Tell a message to a given PID
func (pid *PID) Tell(message interface{}) {
	ref, _ := ProcessRegistry.fromPID(pid)
	ref.Tell(pid, message)
}

//Ask a message to a given PID
func (pid *PID) Ask(message interface{}, sender *PID) error {
	ref, found := ProcessRegistry.fromPID(pid)
	if !found {
		return fmt.Errorf("Unknown PID %s", pid)
	}
	ref.Ask(pid, message, sender)
	return nil
}

//Ask a message to a given PID
func (pid *PID) AskFuture(message interface{}, timeout time.Duration) (*Future, error) {
	ref, found := ProcessRegistry.fromPID(pid)
	if !found {
		return nil, fmt.Errorf("Unknown PID %s", pid)
	}
	future := NewFuture(timeout)
	ref.Ask(pid, message, future.PID())
	return future, nil
}

func (pid *PID) sendSystemMessage(message SystemMessage) {
	ref, _ := ProcessRegistry.fromPID(pid)
	ref.SendSystemMessage(pid, message)
}

func (pid *PID) StopFuture() (*Future, error) {
	ref, found := ProcessRegistry.fromPID(pid)

	if !found {
		return nil, fmt.Errorf("Unknown PID %s", pid)
	}
	future := NewFuture(10 * time.Second)

	log.Printf("ref = %s", reflect.TypeOf(ref))
	ref, ok := ref.(*LocalActorRef)
	if !ok {
		log.Fatalf("Ooops %s", reflect.TypeOf(ref))
	}

	ref.Watch(future.PID())

	ref.Stop(pid)

	return future, nil
}

//Stop the given PID
func (pid *PID) Stop() {
	ref, _ := ProcessRegistry.fromPID(pid)
	ref.Stop(pid)
}

func (pid *PID) suspend() {
	ref, _ := ProcessRegistry.fromPID(pid)
	ref.(*LocalActorRef).Suspend()
}

func (pid *PID) resume() {
	ref, _ := ProcessRegistry.fromPID(pid)
	ref.(*LocalActorRef).Resume()
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
