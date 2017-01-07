package actor

type DeadLetterActorRef struct{}

var (
	deadLetter ActorRef = &DeadLetterActorRef{}
)

type DeadLetter struct {
	PID     *PID
	Message interface{}
}

func (*DeadLetterActorRef) SendUserMessage(pid *PID, message interface{}, sender *PID) {
	EventStream.Publish(&DeadLetter{
		PID:     pid,
		Message: message,
	})
}

func (*DeadLetterActorRef) SendSystemMessage(pid *PID, message SystemMessage) {
	EventStream.Publish(&DeadLetter{
		PID:     pid,
		Message: message,
	})
}

func (ref *DeadLetterActorRef) Stop(pid *PID) {
	ref.SendSystemMessage(pid, stopMessage)
}

func (ref *DeadLetterActorRef) Watch(pid *PID) {
	ref.SendSystemMessage(pid, &Watch{Watcher: pid})
}

func (ref *DeadLetterActorRef) Unwatch(pid *PID) {
	ref.SendSystemMessage(pid, &Unwatch{Watcher: pid})
}
