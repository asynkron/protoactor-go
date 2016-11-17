package actor

import "fmt"

//Tell a message to a given PID
func (pid *PID) Tell(message interface{}) {
	ref, _ := ProcessRegistry.fromPID(pid)
	ref.Tell(message)
}

//Ask a message to a given PID
func (pid *PID) Ask(message interface{}) (*Future, error) {
	ref, found := ProcessRegistry.fromPID(pid)
	if !found {
		return nil, fmt.Errorf("Unknown PID %s", pid)
	}
	future := NewFuture()
	ref.Ask(message, future.PID())
	return future, nil
}

func (pid *PID) sendSystemMessage(message SystemMessage) {
	ref, _ := ProcessRegistry.fromPID(pid)
	ref.SendSystemMessage(message)
}

//Stop the given PID
func (pid *PID) Stop() {
	ref, _ := ProcessRegistry.fromPID(pid)
	ref.Stop()
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
