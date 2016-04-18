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
	stopping bool
}

func NewActorCell(props PropsValue) *ActorCell {
	actor := props.actorProducer()
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
		cell.stopping = true
		cell.invokeUserMessage(Stopping{})
		for child := range cell.children {
			child.Stop()
		}
		cell.tryTerminate()
	case WatchedStopped:
		delete(cell.children, msg.Who)
		cell.tryTerminate()
	case Watch:
		cell.watchers[msg.Who] = msg.Who
	}
}

func (cell *ActorCell) tryTerminate() {
	if !cell.stopping {
		return
	}

	if len(cell.children) > 0 {
		return
	}

	cell.invokeUserMessage(Stopped{})
	watchedStopped := WatchedStopped{Who: cell.Self}
	for watcher := range cell.watchers {
		watcher.SendSystemMessage(watchedStopped)
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
	ref.SendSystemMessage(Watch{Who: cell.Self})
	return ref
}
