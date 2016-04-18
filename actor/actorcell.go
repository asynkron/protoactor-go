package actor

import "fmt"

type Receive func(*Context)
type SetActorRef map[ActorRef]ActorRef

type ActorCell struct {
	Self     ActorRef
	actor    Actor
	props    PropsValue
	behavior Receive
	children SetActorRef
	watchers SetActorRef
}

func NewActorCell(props PropsValue) *ActorCell {
	actor := props.producer()
	cell := ActorCell{
		actor:    actor,
		props:    props,
		behavior: actor.Receive,
		children: make(SetActorRef),
		watchers: make(SetActorRef),
	}
	return &cell
}

func (cell *ActorCell) invokeSystemMessage(message interface{}) {
	switch msg := message.(type) {
	default:
		fmt.Printf("Unknown system message %T", msg)
	case Stop:
		cell.invokeUserMessage(Stopping{})
		for child := range cell.children {
			child.Stop()
		}
		cell.tryTerminate()
	case Stopped:
		delete(cell.children,msg.Who)
		cell.tryTerminate()
	case Watch:
		cell.watchers[msg.Who] = msg.Who
	}
}

func (cell *ActorCell) tryTerminate() {
	if len(cell.children) > 0 {
		return
	}

	stopped := Stopped{Who: cell.Self}
	for watcher := range cell.watchers {
		watcher.SendSystemMessage(stopped)
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
	ref.SendSystemMessage(Watch{Who:cell.Self})
	return ref
}
