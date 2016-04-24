package gam

func (pid *PID) Tell(message interface{}) {
	ref, _ := GlobalProcessRegistry.FromPID(pid)
	ref.Tell(message)
}

func Tell(pid *PID, message interface{}) {
	ref, _ := GlobalProcessRegistry.FromPID(pid)
	ref.Tell(message)
}

func (pid *PID) sendSystemMessage(message SystemMessage) {
	ref, _ := GlobalProcessRegistry.FromPID(pid)
	ref.SendSystemMessage(message)
}

func (pid *PID) Stop() {
	ref, _ := GlobalProcessRegistry.FromPID(pid)
	ref.Stop()
}

func (pid *PID) suspend() {
	ref, _ := GlobalProcessRegistry.FromPID(pid)
	ref.(*LocalActorRef).Suspend()
}

func (pid *PID) resume() {
	ref, _ := GlobalProcessRegistry.FromPID(pid)
	ref.(*LocalActorRef).Resume()
}

func NewPID(node string, host string, id string) *PID {
	return &PID {
		Node: node,
		Host: host,
		Id: id,
	}
}