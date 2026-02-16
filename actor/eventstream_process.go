package actor

type EventStreamProcess struct {
	system *ActorSystem
}

var _ Process = &EventStreamProcess{}

func NewEventStreamProcess(actorSystem *ActorSystem) *EventStreamProcess {
	return &EventStreamProcess{system: actorSystem}
}

func (e *EventStreamProcess) SendUserMessage(_ *PID, message any) {
	_, msg, _ := UnwrapEnvelope(message)
	e.system.EventStream.Publish(msg)
}

func (e *EventStreamProcess) SendSystemMessage(_ *PID, _ any) {
	// pass
}

func (e *EventStreamProcess) Stop(_ *PID) {
	// pass
}
