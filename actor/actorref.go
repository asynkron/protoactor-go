package actor

//ActorRef is an interface that defines the base contract for interaction of actors
type ActorRef interface {
	SendUserMessage(pid *PID, message interface{}, sender *PID)
	SendSystemMessage(pid *PID, message SystemMessage)
	Stop(pid *PID)
	Watch(pid *PID)
	Unwatch(pid *PID)
}
