package interfaces

type ActorRef interface {
	Tell(message interface{})
	SendSystemMessage(message SystemMessage)
	Stop()
}
