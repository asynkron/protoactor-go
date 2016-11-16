package actor

import "fmt"

func (pid *PID) Tell(message interface{}) {
	ref, _ := ProcessRegistry.fromPID(pid)
	ref.Tell(message)
}

func (pid *PID) Ask(message interface{}) (error, *Response) {
	ref, found := ProcessRegistry.fromPID(pid)
	if !found {
		return fmt.Errorf("Unknown PID %s", pid), nil
	}
	pid, result := RequestResponsePID()
	ref.Ask(message, pid)
	return nil, result
}

func (pid *PID) sendSystemMessage(message SystemMessage) {
	ref, _ := ProcessRegistry.fromPID(pid)
	ref.SendSystemMessage(message)
}

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

func NewPID(host, id string) *PID {
	return &PID{
		Host: host,
		Id:   id,
	}
}

func NewLocalPID(id string) *PID {
	return &PID{
		Host: ProcessRegistry.Host,
		Id:   id,
	}
}
