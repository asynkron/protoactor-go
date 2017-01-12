package actor

type deadLetterActorRef struct{}

var (
	deadLetter ActorRef = &deadLetterActorRef{}
)

type DeadLetter struct {
	PID     *PID
	Message interface{}
	Sender  *PID
}

func (*deadLetterActorRef) SendUserMessage(pid *PID, message interface{}, sender *PID) {
	EventStream.Publish(&DeadLetter{
		PID:     pid,
		Message: message,
		Sender:  sender,
	})
}

func (*deadLetterActorRef) SendSystemMessage(pid *PID, message SystemMessage) {
	EventStream.Publish(&DeadLetter{
		PID:     pid,
		Message: message,
	})
}

func (ref *deadLetterActorRef) Stop(pid *PID) {
	ref.SendSystemMessage(pid, stopMessage)
}

func (ref *deadLetterActorRef) Watch(pid *PID) {
	ref.SendSystemMessage(pid, &Watch{Watcher: pid})
}

func (ref *deadLetterActorRef) Unwatch(pid *PID) {
	ref.SendSystemMessage(pid, &Unwatch{Watcher: pid})
}
