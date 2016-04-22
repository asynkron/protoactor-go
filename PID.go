package gam

func (pid *PID) Tell(message interface{}) {
	ref, _ := FromPID(pid)
	ref.Tell(message)
}

func Tell(pid *PID, message interface{}) {
	ref, _ := FromPID(pid)
	ref.Tell(message)
}

func (pid *PID) SendSystemMessage(message SystemMessage) {
	ref, _ := FromPID(pid)
	ref.SendSystemMessage(message)
}

func (pid *PID) Stop() {
	ref, _ := FromPID(pid)
	ref.Stop()
}

func (pid *PID) suspend() {
	ref, _ := FromPID(pid)
	ref.(*LocalActorRef).Suspend()
}

func (pid *PID) resume() {
	ref, _ := FromPID(pid)
	ref.(*LocalActorRef).Resume()
}
