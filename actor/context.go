package actor

import (
	"fmt"
	"log"
	"reflect"

	"github.com/emirpasic/gods/stacks/linkedliststack"
)

type Request struct {
	Message interface{}
	Sender  *PID
}

type MessageInvoker interface {
	InvokeSystemMessage(SystemMessage)
	InvokeUserMessage(interface{})
}

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
	Receive(interface{})
	//Stashes the current message
	Stash()

	//Respond to the current Sender()
	Respond(response interface{})

	//the actor instance
	Actor() Actor
}

func (cell *actorCell) Actor() Actor {
	return cell.actor
}

func (cell *actorCell) Message() interface{} {
	userMessage, ok := cell.message.(*Request)
	if ok {
		return userMessage.Message
	}
	return cell.message
}

func (cell *actorCell) Sender() *PID {
	userMessage, ok := cell.message.(*Request)
	if ok {
		return userMessage.Sender
	}
	return nil
}

func (cell *actorCell) Stash() {
	if cell.stash == nil {
		cell.stash = linkedliststack.New()
	}

	cell.stash.Push(cell.message)
}

type actorCell struct {
	message        interface{}
	parent         *PID
	self           *PID
	actor          Actor
	props          Props
	supervisor     SupervisionStrategy
	behavior       behaviorStack
	children       PIDSet
	watchers       PIDSet
	watching       PIDSet
	stash          *linkedliststack.Stack
	receivePlugins []Receive
	receiveIndex   int
	stopping       bool
	restarting     bool
}

func (cell *actorCell) Children() []*PID {
	r := make([]*PID, cell.children.Len())
	cell.children.ForEach(func(i int, p PID) {
		r[i] = &p
	})
	return r
}

func (cell *actorCell) Self() *PID {
	return cell.self
}

func (cell *actorCell) Parent() *PID {
	return cell.parent
}

func NewActorCell(props Props, parent *PID) *actorCell {
	bs := make(behaviorStack, 0, 8)
	cell := actorCell{
		parent:         parent,
		props:          props,
		supervisor:     props.Supervisor(),
		behavior:       bs,
		receivePlugins: props.receivePluins,
	}
	cell.incarnateActor()
	return &cell
}

func (cell *actorCell) Receive(message interface{}) {
	i := cell.receiveIndex
	m := cell.message

	cell.receiveIndex = 0
	cell.message = message
	cell.Next()

	cell.receiveIndex = i
	cell.message = m
}

func (cell *actorCell) Next() {
	if cell.receiveIndex < len(cell.receivePlugins) {
		receive := cell.receivePlugins[cell.receiveIndex]
		cell.receiveIndex++
		receive(cell)
	} else {
		cell.AutoReceiveOrUser()
	}
}
func (cell *actorCell) incarnateActor() {
	actor := cell.props.ProduceActor()
	cell.restarting = false
	cell.stopping = false
	cell.actor = actor
	cell.Become(actor.Receive)
}

func (cell *actorCell) InvokeSystemMessage(message SystemMessage) {
	switch msg := message.(interface{}).(type) {
	default:
		fmt.Printf("Unknown system message %T", msg)
	case *Stop:
		cell.handleStop(msg)
	case *Terminated:
		cell.handleTerminated(msg)
	case *Watch:
		cell.watchers.Add(msg.Watcher)
	case *Unwatch:
		cell.watchers.Remove(msg.Watcher)
	case *Failure:
		cell.handleFailure(msg)
	case *Restart:
		cell.handleRestart(msg)
	}
}

func (cell *actorCell) handleRestart(msg *Restart) {
	cell.stopping = false
	cell.restarting = true
	cell.InvokeUserMessage(&Restarting{})
	cell.children.ForEach(func(i int, pid PID) {
		pid.Stop()
	})
	cell.tryRestartOrTerminate()
}

//I am stopping
func (cell *actorCell) handleStop(msg *Stop) {
	cell.stopping = true
	cell.restarting = false
	cell.InvokeUserMessage(&Stopping{})
	cell.children.ForEach(func(i int, pid PID) {
		pid.Stop()
	})
	cell.tryRestartOrTerminate()
}

//child stopped, check if we can stop or restart (if needed)
func (cell *actorCell) handleTerminated(msg *Terminated) {
	cell.children.Remove(msg.Who)
	cell.watching.Remove(msg.Who)

	cell.InvokeUserMessage(msg)
	cell.tryRestartOrTerminate()
}

func (cell *actorCell) handleFailure(msg *Failure) {
	directive := cell.supervisor.Handle(msg.Who, msg.Reason)
	switch directive {
	case ResumeDirective:
		//resume the failing child
		msg.Who.sendSystemMessage(&ResumeMailbox{})
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

func (cell *actorCell) tryRestartOrTerminate() {
	if !cell.children.Empty() {
		return
	}

	if cell.restarting {
		cell.restart()
		return
	}

	if cell.stopping {
		cell.stopped()
	}
}

func (cell *actorCell) restart() {
	cell.incarnateActor()
	cell.InvokeUserMessage(&Started{})
	if cell.stash != nil {
		for !cell.stash.Empty() {
			msg, _ := cell.stash.Pop()
			cell.InvokeUserMessage(msg)
		}
	}
}

func (cell *actorCell) stopped() {
	ProcessRegistry.remove(cell.self)
	cell.InvokeUserMessage(&Stopped{})
	otherStopped := &Terminated{Who: cell.self}
	cell.watchers.ForEach(func(i int, pid PID) {
		pid.sendSystemMessage(otherStopped)
	})
}

func (cell *actorCell) InvokeUserMessage(md interface{}) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[ACTOR] '%v' Recovering from: %v", cell.debugString(), r)
			failure := &Failure{Reason: r, Who: cell.self}
			if cell.parent == nil {
				handleRootFailure(failure, defaultSupervisionStrategy)
			} else {
				cell.self.sendSystemMessage(&SuspendMailbox{})
				cell.parent.sendSystemMessage(failure)
			}
		}
	}()
	cell.receiveIndex = 0
	cell.message = md

	//optimize fast path, remove next from profiler flow
	if cell.receivePlugins == nil {
		cell.AutoReceiveOrUser()
	} else {
		cell.Next()
	}
}

func (cell *actorCell) AutoReceiveOrUser() {
	switch cell.Message().(type) {
	case *PoisonPill:
		cell.self.Stop()
	default:
		receive, _ := cell.behavior.Peek()
		receive(cell)
	}
}

func (cell *actorCell) Become(behavior Receive) {
	cell.behavior.Clear()
	cell.behavior.Push(behavior)
}

func (cell *actorCell) BecomeStacked(behavior Receive) {
	cell.behavior.Push(behavior)
}

func (cell *actorCell) UnbecomeStacked() {
	if cell.behavior.Len() <= 1 {
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

func (cell *actorCell) Respond(response interface{}) {
	if cell.Sender() == nil {
		log.Fatal("[ACTOR] No sender")
	}
	cell.Sender().Tell(response)
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

func (cell *actorCell) debugString() string {
	return fmt.Sprintf("%v/%v:%v", cell.self.Host, cell.self.Id, reflect.TypeOf(cell.actor))
}

func handleRootFailure(msg *Failure, supervisor SupervisionStrategy) {
	directive := supervisor.Handle(msg.Who, msg.Reason)
	switch directive {
	case ResumeDirective:
		//resume the fialing child
		msg.Who.sendSystemMessage(&ResumeMailbox{})
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
