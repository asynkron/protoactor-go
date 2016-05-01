package actor

func Spawn(props Properties) *PID {
	id := ProcessRegistry.getAutoId()
	pid := spawn(id, props, nil)
	return pid
}

func SpawnNamed(props Properties, name string) *PID {
	pid := spawn(name, props, nil)
	return pid
}

func spawn(id string, props Properties, parent *PID) *PID {
	cell := NewActorCell(props, parent)
	mailbox := props.ProduceMailbox()
	mailbox.RegisterHandlers(cell.invokeUserMessage, cell.invokeSystemMessage)
	ref := NewLocalActorRef(mailbox)
	pid := ProcessRegistry.registerPID(ref, id)
	cell.self = pid
	cell.invokeUserMessage(Started{})
	return pid
}
