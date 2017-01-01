package actor

//Spawn an actor with an auto generated id
func Spawn(props Props) *PID {
	id := ProcessRegistry.getAutoId()
	pid := spawn(id, props, nil)
	return pid
}

//SpawnNamed spawns a named actor
func SpawnNamed(props Props, name string) *PID {
	pid := spawn(name, props, nil)
	return pid
}

func spawn(id string, props Props, parent *PID) *PID {
	if props.RouterConfig() != nil {
		return spawnRouter(id, props.RouterConfig(), props, parent)
	}

	cell := NewActorCell(props, parent)
	mailbox := props.ProduceMailbox()
	ref := NewLocalActorRef(mailbox)
	pid, new := ProcessRegistry.add(ref, id)

	if new {
		mailbox.RegisterHandlers(cell, props.Dispatcher())
		cell.self = pid
		cell.InvokeUserMessage(&Started{})
	}

	return pid
}
