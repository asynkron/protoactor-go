package gam


func SpawnFunc(producer ActorProducer) *PID {
	props := Props(producer)
	pid := spawnChild(props, nil)
	return pid
}

func SpawnTemplate(template Actor) *PID {
	producer := func() Actor {
		return template
	}
	props := Props(producer)
	pid := spawnChild(props, nil)
	return pid
}

func Spawn(props Properties) *PID {
	pid := spawnChild(props, nil)
	return pid
}

func spawnChild(props Properties, parent *PID) *PID {
	cell := NewActorCell(props, parent)
	mailbox := props.ProduceMailbox()
	mailbox.RegisterHandlers(cell.invokeUserMessage, cell.invokeSystemMessage)
	ref := NewLocalActorRef(mailbox)	
	pid := registerPID(ref)
	cell.self = pid
	cell.invokeUserMessage(Started{})
	return pid
}
