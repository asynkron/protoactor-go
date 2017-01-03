package actor

import (
	"fmt"
	"log"
	"reflect"
	"time"

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
	// Watch registers the actor as a monitor for the specified PID
	Watch(*PID)

	// Unwatch unregisters the actor as a monitor for the specified PID
	Unwatch(*PID)

	// Message returns the current message to be processed
	Message() interface{}

	// SetReceiveTimeout sets the inactivity timeout, after which a ReceiveTimeout message will be sent to the actor.
	// A duration of less than 1ms will disable the inactivity timer.
	SetReceiveTimeout(d time.Duration)

	// ReceiveTimeout returns the current timeout
	ReceiveTimeout() time.Duration

	// Sender returns the PID of actor that sent currently processed message
	Sender() *PID

	// Become replaces the actors current Receive handler with a new handler
	Become(Receive)

	// BecomeStacked pushes a new Receive handler on the current handler stack
	BecomeStacked(Receive)

	// UnbecomeStacked reverts to the previous Receive handler
	UnbecomeStacked()

	// Self returns the PID for the current actor
	Self() *PID

	// Parent returns the PID for the current actors parent
	Parent() *PID

	// Spawn spawns a child actor using the given Props
	Spawn(Props) *PID

	// SpawnNamed spawns a named child actor using the given Props
	SpawnNamed(Props, string) *PID

	// Returns a slice of the current actors children
	Children() []*PID

	// Next performs the next middleware or base Receive handler
	Next()

	// Receive processes a custom user message synchronously
	Receive(interface{})

	// Stash stashes the current message on a stack for reprocessing when the actor restarts
	Stash()

	// Respond sends a response to the to the current `Sender`
	Respond(response interface{})

	// Actor returns the actor associated with this context
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

func (cell *actorCell) cancelTimer() {
	if cell.t != nil {
		cell.t.Stop()
		cell.t = nil
	}
}

func (cell *actorCell) receiveTimeoutHandler() {
	cell.self.Request(&ReceiveTimeout{}, nil)
}

func (cell *actorCell) SetReceiveTimeout(d time.Duration) {
	if d == cell.receiveTimeout {
		return
	}
	if cell.t != nil {
		cell.t.Stop()
	}

	if d < time.Millisecond {
		// anything less than than 1 millisecond is set to zero
		d = 0
	}

	cell.receiveTimeout = d
	if d > 0 {
		if cell.t == nil {
			cell.t = time.AfterFunc(d, cell.receiveTimeoutHandler)
		} else {
			cell.t.Reset(d)
		}
	}
}

func (cell *actorCell) ReceiveTimeout() time.Duration {
	return cell.receiveTimeout
}

type actorCell struct {
	message        interface{}
	parent         *PID
	self           *PID
	actor          Actor
	props          Props
	behavior       behaviorStack
	children       PIDSet
	watchers       PIDSet
	watching       PIDSet
	stash          *linkedliststack.Stack
	receiveIndex   int
	stopping       bool
	restarting     bool
	receiveTimeout time.Duration
	t              *time.Timer
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
	cell := &actorCell{
		parent:   parent,
		props:    props,
		behavior: make(behaviorStack, 0, 8),
	}
	cell.incarnateActor()
	return cell
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
	if cell.receiveIndex < len(cell.props.receivePlugins) {
		receive := cell.props.receivePlugins[cell.receiveIndex]
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

//offload the supervision completely to the supervisor strategy
func (cell *actorCell) handleFailure(msg *Failure) {
	cell.props.Supervisor().HandleFailure(cell, msg.Who, msg.Reason)
}

func (cell *actorCell) EscalateFailure(who *PID, reason interface{}) {
	//suspend self
	cell.Self().sendSystemMessage(&SuspendMailbox{})
	//send failure to parent
	cell.Parent().sendSystemMessage(&Failure{Reason: reason, Who: who})
}

func (cell *actorCell) tryRestartOrTerminate() {
	if cell.t != nil {
		cell.t.Stop()
		cell.t = nil
		cell.receiveTimeout = 0
	}

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
				handleRootFailure(failure)
			} else {
				//TODO: Akka recursively suspends all children also on failure
				//Not sure if I think this is the right way to go, why do children need to wait for their parents failed state to recover?
				cell.self.sendSystemMessage(&SuspendMailbox{})
				cell.parent.sendSystemMessage(failure)
			}
		}
	}()
	cell.receiveIndex = 0
	cell.message = md

	influenceTimeout := true
	if cell.receiveTimeout > 0 {
		_, influenceTimeout = md.(NotInfluenceReceiveTimeout)
		influenceTimeout = !influenceTimeout
		if influenceTimeout {
			cell.t.Stop()
		}
	}

	//optimize fast path, remove next from profiler flow
	if cell.props.receivePlugins == nil {
		cell.AutoReceiveOrUser()
	} else {
		cell.Next()
	}

	if cell.receiveTimeout > 0 && influenceTimeout {
		cell.t.Reset(cell.receiveTimeout)
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

func handleRootFailure(msg *Failure) {
	defaultSupervisionStrategy.HandleFailure(nil, msg.Who, msg.Reason)
}
