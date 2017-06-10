package actor

// A Process is an interface that defines the base contract for interaction of actors
type Process interface {
	SendUserMessage(pid *PID, message interface{})
	SendSystemMessage(pid *PID, message interface{})
	Stop(pid *PID)
}
