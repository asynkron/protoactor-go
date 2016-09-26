package actor

func Spawn(props Props) *PID {
	id := ProcessRegistry.getAutoId()
	pid := spawn(id, props, nil)
	return pid
}

func SpawnNamed(props Props, name string) *PID {
	pid := spawn(name, props, nil)
	return pid
}

func spawn(id string, props Props, parent *PID) *PID {
	if props.RouterConfig() != nil {
		return spawnRouter(props.RouterConfig(), props, parent)
	}

	cell := NewActorCell(props, parent)
	mailbox := props.ProduceMailbox()
	mailbox.RegisterHandlers(cell.invokeUserMessage, cell.invokeSystemMessage)
	ref := NewLocalActorRef(mailbox)
	pid := ProcessRegistry.registerPID(ref, id)
	cell.self = pid

	cell.invokeUserMessage(&Started{})
	return pid
}
