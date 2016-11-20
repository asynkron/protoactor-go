package actor

//ActorRef is an interface that defines the base contract for interaction of actors
type ActorRef interface {
	Tell(pid *PID, message interface{})
	Ask(pid *PID, message interface{}, sender *PID)
	SendSystemMessage(pid *PID, message SystemMessage)
	Stop(pid *PID)
	Watch(pid *PID)
	UnWatch(pid *PID)
}

type LocalActorRef struct {
	mailbox Mailbox
}

func NewLocalActorRef(mailbox Mailbox) *LocalActorRef {
	return &LocalActorRef{
		mailbox: mailbox,
	}
}

func (ref *LocalActorRef) Tell(pid *PID, message interface{}) {
	ref.mailbox.PostUserMessage(UserMessage{Message: message})
}

func (ref *LocalActorRef) Ask(pid *PID, message interface{}, sender *PID) {
	ref.mailbox.PostUserMessage(UserMessage{Message: message, Sender: sender})
}

func (ref *LocalActorRef) SendSystemMessage(pid *PID, message SystemMessage) {
	ref.mailbox.PostSystemMessage(message)
}

func (ref *LocalActorRef) Stop(pid *PID) {
	ref.SendSystemMessage(pid, &stop{})
}

func (ref *LocalActorRef) Watch(pid *PID) {
	ref.SendSystemMessage(pid, &watch{Watcher: pid})
}

func (ref *LocalActorRef) UnWatch(pid *PID) {
	ref.SendSystemMessage(pid, &unwatch{Watcher: pid})
}

func (ref *LocalActorRef) Suspend() {
	ref.mailbox.Suspend()
}

func (ref *LocalActorRef) Resume() {
	ref.mailbox.Resume()
}

type DeadLetterActorRef struct {
}

var deadLetter ActorRef = new(DeadLetterActorRef)

type DeadLetter struct {
	PID     *PID
	Message interface{}
}

func (DeadLetterActorRef) Tell(pid *PID, message interface{}) {
	EventStream.Publish(&DeadLetter{
		PID:     pid,
		Message: message,
	})
}

func (DeadLetterActorRef) Ask(pid *PID, message interface{}, sender *PID) {
	EventStream.Publish(&DeadLetter{
		PID:     pid,
		Message: message,
	})
}

func (DeadLetterActorRef) SendSystemMessage(pid *PID, message SystemMessage) {
}

func (DeadLetterActorRef) Stop(pid *PID) {
}

func (DeadLetterActorRef) Watch(pid *PID) {
}

func (DeadLetterActorRef) UnWatch(pid *PID) {
}

//log.Printf("Deadletter for %v got %+v", pid, message)
