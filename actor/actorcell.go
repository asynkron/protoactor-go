package actor

import "fmt"
import "github.com/emirpasic/gods/sets/hashset"
import "github.com/emirpasic/gods/stacks/linkedliststack"
import "github.com/rogeralsing/goactor/interfaces"

type ActorCell struct {
	parent     interfaces.ActorRef
	self       *LocalActorRef
	actor      interfaces.Actor
	props      interfaces.Props
	supervisor interfaces.SupervisionStrategy
	behavior   *linkedliststack.Stack
	children   *hashset.Set
	watchers   *hashset.Set
	watching   *hashset.Set
	stopping   bool
}

func (cell *ActorCell) Self() interfaces.ActorRef {
	return cell.self
}

func (cell *ActorCell) Parent() interfaces.ActorRef {
	return cell.parent
}

func NewActorCell(props interfaces.Props, parent interfaces.ActorRef) *ActorCell {

	cell := ActorCell{
		parent:     parent,
		props:      props,
		supervisor: props.Supervisor(),
		behavior:   linkedliststack.New(),
		children:   hashset.New(),
		watchers:   hashset.New(),
		watching:   hashset.New(),
	}
	cell.incarnateActor()
	return &cell
}

func (cell *ActorCell) incarnateActor() {
	actor := cell.props.ProduceActor()
	cell.actor = actor
	cell.Become(actor.Receive)
}

func (cell *ActorCell) invokeSystemMessage(message interfaces.SystemMessage) {
	switch msg := message.(interface{}).(type) {
	default:
		fmt.Printf("Unknown system message %T", msg)
	case *Stop:
		cell.handleStop(msg)
	case *OtherStopped:
		cell.handleOtherStopped(msg)
	case *Watch:
		cell.watchers.Add(msg.Watcher)
	case *Unwatch:
		cell.watchers.Remove(msg.Watcher)
	case *Failure:
		cell.handleFailure(msg)
	case *Restart:
		cell.handleRestart(msg)
	case *Resume:
		cell.self.Resume()
	}
}

func (cell *ActorCell) handleStop(msg *Stop) {
	cell.stopping = true
	cell.invokeUserMessage(Stopping{})
	for _, child := range cell.children.Values() {
		child.(interfaces.ActorRef).Stop()
	}
	cell.tryTerminate()
}

func (cell *ActorCell) handleOtherStopped(msg *OtherStopped) {
	cell.children.Remove(msg.Who)
	cell.watching.Remove(msg.Who)
	cell.tryTerminate()
}

func (cell *ActorCell) handleFailure(msg *Failure) {
	directive := cell.supervisor.Handle(msg.Who, msg.Reason)
	switch directive {
	case interfaces.Resume:
		//resume the fialing child
		msg.Who.SendSystemMessage(&Resume{})
	case interfaces.Restart:
		//restart the failing child
		msg.Who.SendSystemMessage(&Restart{})
	case interfaces.Stop:
		//stop the failing child
		msg.Who.Stop()
	case interfaces.Escalate:
		//send failure to parent
		cell.parent.SendSystemMessage(msg)
	}
}

func (cell *ActorCell) handleRestart(msg *Restart) {
	cell.incarnateActor()
	cell.invokeUserMessage(Starting{})
}

func (cell *ActorCell) tryTerminate() {
	if !cell.stopping {
		return
	}

	if !cell.children.Empty() {
		return
	}

	cell.invokeUserMessage(Stopped{})
	otherStopped := &OtherStopped{Who: cell.self}
	for _, watcher := range cell.watchers.Values() {
		watcher.(interfaces.ActorRef).SendSystemMessage(otherStopped)
	}
}

func (cell *ActorCell) invokeUserMessage(message interface{}) {
	defer func() {
		if r := recover(); r != nil {
			if cell.parent == nil {
				fmt.Println("TODO: implement root supervisor")
			} else {
				cell.self.Suspend()
				cell.parent.SendSystemMessage(&Failure{Reason: r, Who: cell.self})
			}
		}
	}()
	behavior, _ := cell.behavior.Peek()
	behavior.(interfaces.Receive)(NewContext(cell, message))
}

func (cell *ActorCell) Become(behavior interfaces.Receive) {
	cell.behavior.Clear()
	cell.behavior.Push(behavior)
}

func (cell *ActorCell) BecomeStacked(behavior interfaces.Receive) {
	cell.behavior.Push(behavior)
}

func (cell *ActorCell) UnbecomeStacked() {
	if cell.behavior.Size() <= 1 {
		panic("Can not unbecome actor base behavior")
	}
	cell.behavior.Pop()
}

func (cell *ActorCell) Watch(who interfaces.ActorRef) {
	who.SendSystemMessage(&Watch{
		Watcher: cell.self,
	})
	cell.watching.Add(who)
}

func (cell *ActorCell) Unwatch(who interfaces.ActorRef) {
	who.SendSystemMessage(&Unwatch{
		Watcher: cell.self,
	})
	cell.watching.Remove(who)
}

func (cell *ActorCell) SpawnChild(props interfaces.Props) interfaces.ActorRef {
	ref := spawnChild(props, cell.self)
	cell.children.Add(ref)
	cell.Watch(ref)
	return ref
}
