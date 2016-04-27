package actor

import "fmt"
import "github.com/emirpasic/gods/sets/hashset"
import "github.com/emirpasic/gods/stacks/linkedliststack"

type Context interface {
	Watch(*PID)
	Unwatch(*PID)
	Message() interface{}
	Become(Receive)
	BecomeStacked(Receive)
	UnbecomeStacked()
	Self() *PID
	Parent() *PID
	Spawn(Properties) *PID
	SpawnTemplate(Actor) *PID
	SpawnFunc(ActorProducer) *PID
	Children() []*PID
}

type ContextValue struct {
	*ActorCell
	message interface{}
}

func (context *ContextValue) Message() interface{} {
	return context.message
}

func NewContext(cell *ActorCell, message interface{}) Context {
	res := &ContextValue{
		ActorCell: cell,
		message:   message,
	}
	return res
}

type ActorCell struct {
	parent     *PID
	self       *PID
	actor      Actor
	props      Properties
	supervisor SupervisionStrategy
	behavior   *linkedliststack.Stack
	children   *hashset.Set
	watchers   *hashset.Set
	watching   *hashset.Set
	stopping   bool
}

func (cell *ActorCell) Children() []*PID {
	values := cell.children.Values()
	children := make([]*PID, len(values))
	for i, child := range values {
		children[i] = child.(*PID)
	}
	return children
}

func (cell *ActorCell) Self() *PID {
	return cell.self
}

func (cell *ActorCell) Parent() *PID {
	return cell.parent
}

func NewActorCell(props Properties, parent *PID) *ActorCell {

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

func (cell *ActorCell) invokeSystemMessage(message SystemMessage) {
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

func (cell *ActorCell) handleStop(msg *stop) {
	cell.stopping = true
	cell.invokeUserMessage(Stopping{})
	for _, child := range cell.children.Values() {
		child.(*PID).Stop()
	}
	cell.tryRestartOrTerminate()
}

func (cell *ActorCell) handleOtherStopped(msg *otherStopped) {
	cell.children.Remove(msg.Who)
	cell.watching.Remove(msg.Who)
	cell.tryRestartOrTerminate()
}

func (cell *ActorCell) handleFailure(msg *failure) {
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

func (cell *ActorCell) handleRestart(msg *restart) {
	cell.stopping = false
	cell.invokeUserMessage(Restarting{}) //TODO: change to restarting
	for _, child := range cell.children.Values() {
		child.(*PID).Stop()
	}
	cell.tryRestartOrTerminate()
}

func (cell *ActorCell) tryRestartOrTerminate() {
	if !cell.children.Empty() {
		return
	}

	if !cell.stopping {
		cell.incarnateActor()
		cell.invokeUserMessage(Started{})
		return
	}

	ProcessRegistry.unregisterPID(cell.self)	
	cell.invokeUserMessage(Stopped{})
	otherStopped := &otherStopped{Who: cell.self}	
	for _, watcher := range cell.watchers.Values() {
		watcher.(*PID).sendSystemMessage(otherStopped)
	}	
}

func (cell *ActorCell) invokeUserMessage(message interface{}) {
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

func (cell *ActorCell) Watch(who *PID) {
	who.sendSystemMessage(&watch{
		Watcher: cell.self,
	})
	cell.watching.Add(who)
}

func (cell *ActorCell) Unwatch(who *PID) {
	who.sendSystemMessage(&unwatch{
		Watcher: cell.self,
	})
	cell.watching.Remove(who)
}

// func (cell *ActorCell) ActorOf(props Properties) *PID {
// 	_, pid := spawnChild(props, cell.self)
// 	cell.children.Add(pid)
// 	cell.Watch(pid)
// 	return pid
// }

func (cell *ActorCell) Spawn(props Properties) *PID {
	pid := spawnChild(props, cell.self)
	cell.children.Add(pid)
	cell.Watch(pid)
	return pid
}

func (cell *ActorCell) SpawnTemplate(template Actor) *PID {
	producer := func() Actor {
		return template
	}
	props := Props(producer)
	pid := spawnChild(props, cell.self)
	cell.children.Add(pid)
	cell.Watch(pid)
	return pid
}

func (cell *ActorCell) SpawnFunc(producer ActorProducer) *PID {
	props := Props(producer)
	pid := spawnChild(props, cell.self)
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
