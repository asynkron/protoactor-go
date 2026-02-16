package actor

// A Process is an interface that defines the base contract for interaction of actors
type Process interface {
	SendUserMessage(pid *PID, message any)
	SendSystemMessage(pid *PID, message any)
	Stop(pid *PID)
}
