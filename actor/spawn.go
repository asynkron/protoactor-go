package actor

import (
	"github.com/rogeralsing/goactor/interfaces"
)

func Spawn(props interfaces.Props) interfaces.ActorRef {
	return spawnChild(props, nil)
}

func spawnChild(props interfaces.Props, parent interfaces.ActorRef) interfaces.ActorRef {
	cell := NewActorCell(props, parent)
	mailbox := props.ProduceMailbox(cell.invokeUserMessage, cell.invokeSystemMessage)
	ref := LocalActorRef{
		mailbox: mailbox,
	}
	cell.self = &ref //TODO: this is fugly
	ref.Tell(Starting{})
	return &ref
}
