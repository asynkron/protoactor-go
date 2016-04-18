package actor

import "fmt"

type Receive func(*Context)

type ActorCell struct {
	Self     ActorRef
	actor    Actor
	props    PropsValue
	behavior Receive
	children map[ActorRef]ActorRef
}

func NewActorCell(props PropsValue) *ActorCell {
	actor := props.producer()
	cell := ActorCell{
		actor:    actor,
		props:    props,
		behavior: actor.Receive,
		children: make(map[ActorRef]ActorRef),
	}
	return &cell
}

func (cell *ActorCell) invokeSystemMessage(message interface{}) {
	switch msg := message.(type) {
	default:
		fmt.Printf("Unknown system message %T", msg)
	case Stop:
		fmt.Println("stopping")
		for child := range cell.children {
			child.Stop()
		}
	}
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

func (cell *ActorCell) SpawnChild(props PropsValue) ActorRef {
	ref := Spawn(props)
	cell.children[ref] = ref
	return ref
}
