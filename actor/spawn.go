package actor

type Spawner func(id string, props Props, parent *PID) *PID

var DefaultSpawner Spawner = spawn

//Spawn an actor with an auto generated id
func Spawn(props Props) *PID {
	return props.spawn(ProcessRegistry.NextId(), nil)
}

//SpawnNamed spawns a named actor
func SpawnNamed(props Props, name string) *PID {
	return props.spawn(name, nil)
}

func spawn(id string, props Props, parent *PID) *PID {
	cell := newActorCell(props, parent)
	mailbox := props.ProduceMailbox()
	ref := newLocalProcess(mailbox)
	pid, absent := ProcessRegistry.Add(ref, id)

	if absent {
		mailbox.RegisterHandlers(cell, props.Dispatcher())
		cell.self = pid
		cell.InvokeUserMessage(startedMessage)
	}

	return pid
}
