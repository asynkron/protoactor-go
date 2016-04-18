package actor

import "fmt"

type Receive func(*Context)

type ActorCell struct {
	Self     ActorRef
	actor    Actor
	props    PropsValue
	behavior Receive
}

func NewActorCell(props PropsValue) *ActorCell {
	actor := props.producer()
	cell := ActorCell{
		actor:    actor,
		props:    props,
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

func (cell *ActorCell) Become(behavior Receive) {
	cell.behavior = behavior
}

func (cell *ActorCell) Unbecome() {
	cell.behavior = cell.actor.Receive
}