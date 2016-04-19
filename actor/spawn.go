package actor

func ActorOf(props Properties) ActorRef {
	return spawnChild(props, nil)
}

func spawnChild(props Properties, parent ActorRef) ActorRef {
	cell := NewActorCell(props, parent)
	mailbox := props.Mailbox()
	mailbox.RegisterHandlers(cell.invokeUserMessage, cell.invokeSystemMessage)
	ref := LocalActorRef{
		mailbox: mailbox,
	}
	cell.self = &ref //TODO: this is fugly
	ref.Tell(Starting{})
	return &ref
}
