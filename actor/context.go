package actor

import (
	"fmt"

	"github.com/emirpasic/gods/sets/hashset"
	"github.com/emirpasic/gods/stacks/linkedliststack"
)

type Context interface {
	Watch(*PID)
	Unwatch(*PID)
	Message() interface{}
	Become(Receive)
	BecomeStacked(Receive)
	UnbecomeStacked()
	Self() *PID
	Parent() *PID
	Spawn(Props) *PID
	SpawnNamed(Props, string) *PID
	Children() []*PID
	Stash()
}

type contextValue struct {
	*actorCell
	message interface{}
}

func (context *contextValue) Message() interface{} {
	return context.message
}

func (context *contextValue) Stash() {
	context.actorCell.stashMessage(context.message)
}

func newContext(cell *actorCell, message interface{}) Context {
	res := &contextValue{
		actorCell: cell,
		message:   message,
	}
	return res
}

type actorCell struct {
	parent     *PID
	self       *PID
	actor      Actor
	props      Props
	supervisor SupervisionStrategy
	behavior   *linkedliststack.Stack
	children   *hashset.Set
	watchers   *hashset.Set
	watching   *hashset.Set
	stash      *linkedliststack.Stack
	stopping   bool
}

func (cell *actorCell) Children() []*PID {
	values := cell.children.Values()
	children := make([]*PID, len(values))
	for i, child := range values {
		children[i] = child.(*PID)
	}
	return children
}

func (cell *actorCell) Self() *PID {
	return cell.self
}

func (cell *actorCell) Parent() *PID {
	return cell.parent
}

func (cell *actorCell) stashMessage(message interface{}) {
	if cell.stash == nil {
		cell.stash = linkedliststack.New()
	}

	cell.stash.Push(message)
}

func NewActorCell(props Props, parent *PID) *actorCell {

	cell := actorCell{
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

func (cell *actorCell) incarnateActor() {
	actor := cell.props.ProduceActor()
	cell.actor = actor
	cell.Become(actor.Receive)
}

func (cell *actorCell) invokeSystemMessage(message SystemMessage) {
	switch msg := message.(interface{}).(type) {
	default:
		fmt.Printf("Unknown system message %T", msg)
	case *stop:
		cell.handleStop(msg)
	case *otherStopped:
		cell.handleOtherStopped(msg)
	case *watch:
		cell.watchers.Add(msg.Watcher)
	case *unwatch:
		cell.watchers.Remove(msg.Watcher)
	case *failure:
		cell.handleFailure(msg)
	case *restart:
		cell.handleRestart(msg)
	case *resume:
		cell.self.resume()
	}
}

func (cell *actorCell) handleStop(msg *stop) {
	cell.stopping = true
	cell.invokeUserMessage(Stopping{})
	for _, child := range cell.children.Values() {
		child.(*PID).Stop()
	}
	cell.tryRestartOrTerminate()
}

func (cell *actorCell) handleOtherStopped(msg *otherStopped) {
	cell.children.Remove(msg.Who)
	cell.watching.Remove(msg.Who)
	cell.tryRestartOrTerminate()
}

func (cell *actorCell) handleFailure(msg *failure) {
	directive := cell.supervisor.Handle(msg.Who, msg.Reason)
	switch directive {
	case ResumeDirective:
		//resume the fialing child
		msg.Who.sendSystemMessage(&resume{})
	case RestartDirective:
		//restart the failing child
		msg.Who.sendSystemMessage(&restart{})
	case StopDirective:
		//stop the failing child
		msg.Who.Stop()
	case EscalateDirective:
		//send failure to parent
		cell.parent.sendSystemMessage(msg)
	}
}

func (cell *actorCell) handleRestart(msg *restart) {
	cell.stopping = false
	cell.invokeUserMessage(Restarting{}) //TODO: change to restarting
	for _, child := range cell.children.Values() {
		child.(*PID).Stop()
	}
	cell.tryRestartOrTerminate()
}

func (cell *actorCell) tryRestartOrTerminate() {
	if !cell.children.Empty() {
		return
	}

	if !cell.stopping {
		cell.restart()
		return
	}

	cell.stopped()
}

func (cell *actorCell) restart() {
	cell.incarnateActor()
	cell.invokeUserMessage(Started{})
	if cell.stash != nil {
		for !cell.stash.Empty() {
			msg, _ := cell.stash.Pop()
			cell.invokeUserMessage(msg)
		}
	}
}

func (cell *actorCell) stopped() {
	ProcessRegistry.unregisterPID(cell.self)
	cell.invokeUserMessage(Stopped{})
	otherStopped := &otherStopped{Who: cell.self}
	for _, watcher := range cell.watchers.Values() {
		watcher.(*PID).sendSystemMessage(otherStopped)
	}
}

func (cell *actorCell) invokeUserMessage(message interface{}) {
	defer func() {
		if r := recover(); r != nil {
			failure := &failure{Reason: r, Who: cell.self}
			if cell.parent == nil {
				handleRootFailure(failure, defaultSupervisionStrategy)
			} else {
				cell.self.suspend()
				cell.parent.sendSystemMessage(failure)
			}
		}
	}()
	behavior, _ := cell.behavior.Peek()
	behavior.(Receive)(newContext(cell, message))
}

func (cell *actorCell) Become(behavior Receive) {
	cell.behavior.Clear()
	cell.behavior.Push(behavior)
}

func (cell *actorCell) BecomeStacked(behavior Receive) {
	cell.behavior.Push(behavior)
}

func (cell *actorCell) UnbecomeStacked() {
	if cell.behavior.Size() <= 1 {
		panic("Can not unbecome actor base behavior")
	}
	cell.behavior.Pop()
}

func (cell *actorCell) Watch(who *PID) {
	who.sendSystemMessage(&watch{
		Watcher: cell.self,
	})
	cell.watching.Add(who)
}

func (cell *actorCell) Unwatch(who *PID) {
	who.sendSystemMessage(&unwatch{
		Watcher: cell.self,
	})
	cell.watching.Remove(who)
}

func (cell *actorCell) Spawn(props Props) *PID {
	id := ProcessRegistry.getAutoId()
	return cell.SpawnNamed(props, id)
}

func (cell *actorCell) SpawnNamed(props Props, name string) *PID {
	pid := spawn(name, props, cell.self)
	cell.children.Add(pid)
	cell.Watch(pid)
	return pid
}

func handleRootFailure(msg *failure, supervisor SupervisionStrategy) {
	directive := supervisor.Handle(msg.Who, msg.Reason)
	switch directive {
	case ResumeDirective:
		//resume the fialing child
		msg.Who.sendSystemMessage(&resume{})
	case RestartDirective:
		//restart the failing child
		msg.Who.sendSystemMessage(&restart{})
	case StopDirective:
		//stop the failing child
		msg.Who.Stop()
	case EscalateDirective:
		//send failure to parent
		panic("Can not escalate root level failures")
	}
}
