package actor

func Spawn(props Properties) *PID {
	pid := spawn(props, nil)
	return pid
}

func spawn(props Properties, parent *PID) *PID {
	cell := NewActorCell(props, parent)
	mailbox := props.ProduceMailbox()
	mailbox.RegisterHandlers(cell.invokeUserMessage, cell.invokeSystemMessage)
	ref := NewLocalActorRef(mailbox)
	pid := ProcessRegistry.registerPID(ref)
	cell.self = pid
	cell.invokeUserMessage(Started{})
	return pid
}
