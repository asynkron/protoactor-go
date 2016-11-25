package actor

import (
	"fmt"
	"log"

	"github.com/emirpasic/gods/sets/hashset"
	"github.com/emirpasic/gods/stacks/linkedliststack"
)

type Context interface {
	//Subscribes to ???
	Watch(*PID)
	Unwatch(*PID)
	//Returns the currently processed message
	Message() interface{}
	//Returns the PID of actor that sent currently processed message
	Sender() *PID
	//Replaces the current Receive handler with a custom
	Become(Receive)
	//Stacks a new Receive handler ontop of the current
	BecomeStacked(Receive)
	UnbecomeStacked()
	//Returns the PID for the current actor
	Self() *PID
	//Returns the PID for the current actors parent
	Parent() *PID
	//Spawns a child actor using the given Props
	Spawn(Props) *PID
	//Spawns a named child actor using the given Props
	SpawnNamed(Props, string) *PID
	//Returns a slice of the current actors children
	Children() []*PID
	//Executes the next middleware or base Receive handler
	Next()
	//Invoke a custom User message synchronously
	Receive(UserMessage)
	//Stashes the current message
	Stash()

	//the actor instance
	Actor() Actor
}

func (cell *actorCell) Actor() Actor {
	return cell.actor
}

func (cell *actorCell) Message() interface{} {
	return cell.userMessage.Message
}

func (cell *actorCell) Sender() *PID {
	return cell.userMessage.Sender
}

func (cell *actorCell) Stash() {
	if cell.stash == nil {
		cell.stash = linkedliststack.New()
	}

	cell.stash.Push(cell.userMessage)
}

type actorCell struct {
	userMessage    *UserMessage
	parent         *PID
	self           *PID
	actor          Actor
	props          Props
	supervisor     SupervisionStrategy
	behavior       *linkedliststack.Stack
	children       *hashset.Set
	watchers       *hashset.Set
	watching       *hashset.Set
	stash          *linkedliststack.Stack
	receivePlugins []Receive
	receiveIndex   int
	stopping       bool
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

func NewActorCell(props Props, parent *PID) *actorCell {

	cell := actorCell{
		parent:         parent,
		props:          props,
		supervisor:     props.Supervisor(),
		behavior:       linkedliststack.New(),
		children:       hashset.New(),
		watchers:       hashset.New(),
		watching:       hashset.New(),
		userMessage:    nil,
		receivePlugins: append(props.receivePluins, AutoReceive),
	}
	cell.incarnateActor()
	return &cell
}

func (cell *actorCell) Receive(userMessage UserMessage) {
	i := cell.receiveIndex
	m := cell.userMessage

	cell.receiveIndex = 0
	cell.userMessage = &userMessage
	cell.Next()

	cell.receiveIndex = i
	cell.userMessage = m
}

func (cell *actorCell) Next() {
	var receive Receive
	if cell.receiveIndex < len(cell.receivePlugins) {
		receive = cell.receivePlugins[cell.receiveIndex]
		cell.receiveIndex++
	} else {
		tmp, _ := cell.behavior.Peek()
		receive = tmp.(Receive)
	}

	receive(cell)
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
	case *Stop:
		cell.handleStop(msg)
	case *Terminated:
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
		cell.self.resume()
	}
}

func (cell *actorCell) handleStop(msg *Stop) {
	cell.stopping = true
	cell.invokeUserMessage(UserMessage{Message: &Stopping{}})
	for _, child := range cell.children.Values() {
		child.(*PID).Stop()
	}
	cell.tryRestartOrTerminate()
}

func (cell *actorCell) handleOtherStopped(msg *Terminated) {
	cell.children.Remove(msg.Who)
	cell.watching.Remove(msg.Who)
	cell.tryRestartOrTerminate()
}

func (cell *actorCell) handleFailure(msg *Failure) {
	directive := cell.supervisor.Handle(msg.Who, msg.Reason)
	switch directive {
	case ResumeDirective:
		//resume the failing child
		msg.Who.sendSystemMessage(&Resume{})
	case RestartDirective:
		//restart the failing child
		msg.Who.sendSystemMessage(&Restart{})
	case StopDirective:
		//stop the failing child
		msg.Who.Stop()
	case EscalateDirective:
		//send failure to parent
		cell.parent.sendSystemMessage(msg)
	}
}

func (cell *actorCell) handleRestart(msg *Restart) {
	cell.stopping = false
	cell.invokeUserMessage(UserMessage{Message: &Restarting{}})
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
	cell.invokeUserMessage(UserMessage{Message: &Started{}})
	if cell.stash != nil {
		for !cell.stash.Empty() {
			msg, _ := cell.stash.Pop()
			pu := msg.(*UserMessage)
			cell.invokeUserMessage(*pu)
		}
	}
}

func (cell *actorCell) stopped() {
	ProcessRegistry.remove(cell.self)
	cell.invokeUserMessage(UserMessage{Message: &Stopped{}})
	otherStopped := &Terminated{Who: cell.self}
	for _, watcher := range cell.watchers.Values() {
		watcher.(*PID).sendSystemMessage(otherStopped)
	}
}

func (cell *actorCell) invokeUserMessage(md UserMessage) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[ACTOR] Recovering from: %v", r)
			failure := &Failure{Reason: r, Who: cell.self}
			if cell.parent == nil {
				handleRootFailure(failure, defaultSupervisionStrategy)
			} else {
				cell.self.suspend()
				cell.parent.sendSystemMessage(failure)
			}
		}
	}()
	cell.receiveIndex = 0
	cell.userMessage = &md
	cell.Next()
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
	who.sendSystemMessage(&Watch{
		Watcher: cell.self,
	})
	cell.watching.Add(who)
}

func (cell *actorCell) Unwatch(who *PID) {
	who.sendSystemMessage(&Unwatch{
		Watcher: cell.self,
	})
	cell.watching.Remove(who)
}

func (cell *actorCell) Spawn(props Props) *PID {
	id := ProcessRegistry.getAutoId()
	return cell.SpawnNamed(props, id)
}

func (cell *actorCell) SpawnNamed(props Props, name string) *PID {
	var fullName string
	if cell.parent != nil {
		fullName = cell.parent.Id + "/" + name
	} else {
		fullName = name
	}

	pid := spawn(fullName, props, cell.self)
	cell.children.Add(pid)
	cell.Watch(pid)
	return pid
}

func handleRootFailure(msg *Failure, supervisor SupervisionStrategy) {
	directive := supervisor.Handle(msg.Who, msg.Reason)
	switch directive {
	case ResumeDirective:
		//resume the fialing child
		msg.Who.sendSystemMessage(&Resume{})
	case RestartDirective:
		//restart the failing child
		msg.Who.sendSystemMessage(&Restart{})
	case StopDirective:
		//stop the failing child
		msg.Who.Stop()
	case EscalateDirective:
		//send failure to parent
		panic("Can not escalate root level failures")
	}
}
