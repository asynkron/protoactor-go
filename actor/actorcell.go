package actor

import "fmt"

type ActorCell struct {
	Self     ActorRef
	actor    Actor
	behavior func(*Context)
}

func NewActorCell(actor Actor) *ActorCell {
	cell := ActorCell{
		actor:    actor,
		behavior: actor.Receive,
	}
	return &cell
}

func (cell *ActorCell) invokeSystemMessage(message interface{}) {
	fmt.Printf("Received system message %v\n", message)
}

func (cell *ActorCell) invokeUserMessage(message interface{}) {
	cell.behavior(NewContext(cell, message))
}

func (cell *ActorCell) Become(behavior func(*Context)) {
	cell.behavior = behavior
}
