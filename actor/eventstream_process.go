package actor

type EventStreamProcess struct {
	system *ActorSystem
}

func NewEventStreamProcess(actorSystem *ActorSystem) *EventStreamProcess {
	return &EventStreamProcess{system: actorSystem}
}

func (e *EventStreamProcess) SendUserMessage(pid *PID, message interface{}) {
	_, msg, _ := UnwrapEnvelope(message)
	e.system.EventStream.Publish(msg)
}

func (e *EventStreamProcess) SendSystemMessage(pid *PID, message interface{}) {
	// pass
}

func (e *EventStreamProcess) Stop(pid *PID) {
	// pass
}
