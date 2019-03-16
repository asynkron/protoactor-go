package actor

// Process is an interface that defines the base contract for interaction of actors
type Process interface {
	SendUserMessage(pid *PID, message interface{}, sender *PID)
	SendSystemMessage(pid *PID, message SystemMessage)
	// Stop(pid *PID)
}
