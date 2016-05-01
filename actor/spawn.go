package actor

func SpawnFunc(producer ActorProducer) *PID {
	props := Props(producer)
	pid := spawnChild(props, nil)
	return pid
}

func SpawnTemplate(template Actor) *PID {
	//actorType := reflect.TypeOf(template)
	producer := func() Actor {
		//	return reflect.New(actorType).Elem().Interface().(Actor)
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

func SpawnReceiveFunc(receive Receive) *PID {
	pid := SpawnTemplate(&EmptyActor{
		receive: receive,
	})
	return pid
}

func spawnChild(props Properties, parent *PID) *PID {
	cell := NewActorCell(props, parent)
	mailbox := props.ProduceMailbox()
	mailbox.RegisterHandlers(cell.invokeUserMessage, cell.invokeSystemMessage)
	ref := NewLocalActorRef(mailbox)
	pid := ProcessRegistry.registerPID(ref)
	cell.self = pid
	cell.invokeUserMessage(Started{})
	return pid
}
