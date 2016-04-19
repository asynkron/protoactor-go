package actor

import "fmt"
import "github.com/emirpasic/gods/sets/hashset"
import "github.com/emirpasic/gods/stacks/linkedliststack"

type Receive func(*Context)

type ActorCell struct {
	parent   ActorRef
	self     ActorRef
	actor    Actor
	props    PropsValue
	behavior *linkedliststack.Stack
	children *hashset.Set
	watchers *hashset.Set
	watching *hashset.Set
	stopping bool
}

func (cell *ActorCell) Self() ActorRef {	
	return cell.self
}

func (cell *ActorCell) Parent() ActorRef {	
	return cell.parent
}

func NewActorCell(props PropsValue, parent ActorRef) *ActorCell {

	cell := ActorCell{
		parent:   parent,
		props:    props,
		behavior: linkedliststack.New(),
		children: hashset.New(),
		watchers: hashset.New(),
		watching: hashset.New(),
	}
	cell.incarnateActor()
	return &cell
}

func (cell *ActorCell) incarnateActor() {
	actor := cell.props.actorProducer()
	cell.actor = actor
	cell.Become(actor.Receive)	
}

func (cell *ActorCell) invokeSystemMessage(message interface{}) {
	switch msg := message.(type) {
	default:
		fmt.Printf("Unknown system message %T", msg)
	case Stop:
		cell.stopping = true
		cell.invokeUserMessage(Stopping{})
		for _, child := range cell.children.Values() {
			child.(ActorRef).Stop()
		}
		cell.tryTerminate()
	case OtherStopped:
		cell.children.Remove(msg.Who)
		cell.watching.Remove(msg.Who)
		cell.tryTerminate()
	case Watch:
		cell.watchers.Add(msg.Watcher)
	case Unwatch:
		cell.watchers.Remove(msg.Watcher)
	case Failure:
		//TODO: apply supervision strategy
		msg.Who.SendSystemMessage(Restart{})
	case Restart:
		cell.incarnateActor()
		cell.invokeUserMessage(Starting{})
	}
}

func (cell *ActorCell) tryTerminate() {
	if !cell.stopping {
		return
	}

	if !cell.children.Empty() {
		return
	}

	cell.invokeUserMessage(Stopped{})
	otherStopped := OtherStopped{Who: cell.Self()}
	for _, watcher := range cell.watchers.Values() {
		watcher.(ActorRef).SendSystemMessage(otherStopped)
	}
}

func (cell *ActorCell) invokeUserMessage(message interface{}) {
	defer func() {
		if r := recover(); r != nil {
			cell.Parent().SendSystemMessage(Failure{Reason: r,Who:cell.Self()})
		}
	}()
	behavior, _ := cell.behavior.Peek()
	behavior.(Receive)(NewContext(cell, message))
}

func (cell *ActorCell) Become(behavior Receive) {
	cell.behavior.Clear()
	cell.behavior.Push(behavior)
}

func (cell *ActorCell) BecomeStacked(behavior Receive) {
	cell.behavior.Push(behavior)
}

func (cell *ActorCell) UnbecomeStacked() {
	if cell.behavior.Size() <= 1 {
		panic("Can not unbecome actor base behavior")
	}
	cell.behavior.Pop()
}

func (cell *ActorCell) Watch(who ActorRef) {
	who.SendSystemMessage(Watch{
		Watcher: cell.Self(),
	})
	cell.watching.Add(who)
}

func (cell *ActorCell) Unwatch(who ActorRef) {
	who.SendSystemMessage(Unwatch{
		Watcher: cell.Self(),
	})
	cell.watching.Remove(who)
}

func (cell *ActorCell) SpawnChild(props PropsValue) ActorRef {
	ref := SpawnChild(props, cell.Self())
	cell.children.Add(ref)
	cell.Watch(ref)
	return ref
}
