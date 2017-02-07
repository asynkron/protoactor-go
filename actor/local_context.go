package actor

import (
	"time"

	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/emirpasic/gods/stacks/linkedliststack"
)

type localContext struct {
	message        interface{}
	parent         *PID
	self           *PID
	actor          Actor
	supervisor     SupervisorStrategy
	producer       Producer
	middleware     ActorFunc
	behavior       behaviorStack
	receive        ActorFunc
	children       PIDSet
	watchers       PIDSet
	watching       PIDSet
	stash          *linkedliststack.Stack
	stopping       bool
	restarting     bool
	receiveTimeout time.Duration
	t              *time.Timer
	restartStats   *RestartStatistics
}

func newLocalContext(producer Producer, supervisor SupervisorStrategy, middleware ActorFunc, parent *PID) *localContext {
	cell := &localContext{
		parent:     parent,
		producer:   producer,
		supervisor: supervisor,
		middleware: middleware,
	}
	cell.incarnateActor()
	return cell
}

func (ctx *localContext) Actor() Actor {
	return ctx.actor
}

func (ctx *localContext) Message() interface{} {
	userMessage, ok := ctx.message.(*messageSender)
	if ok {
		return userMessage.Message
	}
	return ctx.message
}

func (ctx *localContext) Sender() *PID {
	userMessage, ok := ctx.message.(*messageSender)
	if ok {
		return userMessage.Sender
	}
	return nil
}

func (ctx *localContext) Stash() {
	if ctx.stash == nil {
		ctx.stash = linkedliststack.New()
	}

	ctx.stash.Push(ctx.message)
}

func (ctx *localContext) cancelTimer() {
	if ctx.t != nil {
		ctx.t.Stop()
		ctx.t = nil
	}
}

func (ctx *localContext) receiveTimeoutHandler() {
	ctx.self.Request(receiveTimeoutMessage, nil)
}

func (ctx *localContext) SetReceiveTimeout(d time.Duration) {
	if d == ctx.receiveTimeout {
		return
	}
	if ctx.t != nil {
		ctx.t.Stop()
	}

	if d < time.Millisecond {
		// anything less than than 1 millisecond is set to zero
		d = 0
	}

	ctx.receiveTimeout = d
	if d > 0 {
		if ctx.t == nil {
			ctx.t = time.AfterFunc(d, ctx.receiveTimeoutHandler)
		} else {
			ctx.t.Reset(d)
		}
	}
}

func (ctx *localContext) ReceiveTimeout() time.Duration {
	return ctx.receiveTimeout
}

func (ctx *localContext) Children() []*PID {
	r := make([]*PID, ctx.children.Len())
	ctx.children.ForEach(func(i int, p PID) {
		r[i] = &p
	})
	return r
}

func (ctx *localContext) Self() *PID {
	return ctx.self
}

func (ctx *localContext) Parent() *PID {
	return ctx.parent
}

func (ctx *localContext) Receive(message interface{}) {
	ctx.processMessage(message)
}

func (ctx *localContext) RestartStats() *RestartStatistics {
	//lazy initialize the child restart stats if this is the first time
	//further mutations are handled within "restart"
	if ctx.restartStats == nil {
		ctx.restartStats = &RestartStatistics{
			FailureCount: 0,
		}
	}
	return ctx.restartStats
}

func (ctx *localContext) EscalateFailure(reason interface{}, message interface{}) {
	failure := &Failure{Reason: reason, Who: ctx.self, RestartStats: ctx.RestartStats()}
	if ctx.parent == nil {
		handleRootFailure(failure)
	} else {
		//TODO: Akka recursively suspends all children also on failure
		//Not sure if I think this is the right way to go, why do children need to wait for their parents failed state to recover?

		ctx.self.sendSystemMessage(suspendMailboxMessage)
		ctx.parent.sendSystemMessage(failure)
	}
}

func (ctx *localContext) InvokeUserMessage(md interface{}) {
	influenceTimeout := true
	if ctx.receiveTimeout > 0 {
		_, influenceTimeout = md.(NotInfluenceReceiveTimeout)
		influenceTimeout = !influenceTimeout
		if influenceTimeout {
			ctx.t.Stop()
		}
	}

	ctx.processMessage(md)

	if ctx.receiveTimeout > 0 && influenceTimeout {
		ctx.t.Reset(ctx.receiveTimeout)
	}
}

// localContextReceiver is used when middleware chain is required
func localContextReceiver(ctx Context) {
	a := ctx.(*localContext)
	if _, ok := a.message.(*PoisonPill); ok {
		a.self.Stop()
	} else {
		a.receive(ctx)
	}
}

func (ctx *localContext) processMessage(m interface{}) {
	ctx.message = m

	if ctx.middleware != nil {
		ctx.middleware(ctx)
	} else {
		if _, ok := m.(*PoisonPill); ok {
			ctx.self.Stop()
		} else {
			ctx.receive(ctx)
		}
	}

	ctx.message = nil
}

func (ctx *localContext) incarnateActor() {
	actor := ctx.producer()
	ctx.restarting = false
	ctx.stopping = false
	ctx.actor = actor
	ctx.receive = actor.Receive
}

func (ctx *localContext) InvokeSystemMessage(message interface{}) {
	switch msg := message.(type) {
	case *continuation:
		ctx.message = msg.message // apply the message that was present when we started the await
		msg.f()                   // invoke the continuation in the current actor context
		ctx.message = nil         // release the message
	case *Started:
		ctx.InvokeUserMessage(msg) // forward
	case *Watch:
		if ctx.stopping {
			msg.Watcher.sendSystemMessage(&Terminated{Who: ctx.self})
		} else {
			ctx.watchers.Add(msg.Watcher)
		}
	case *Unwatch:
		ctx.watchers.Remove(msg.Watcher)
	case *Stop:
		ctx.handleStop(msg)
	case *Terminated:
		ctx.handleTerminated(msg)
	case *Failure:
		ctx.handleFailure(msg)
	case *Restart:
		ctx.handleRestart(msg)
	default:
		plog.Error("unknown system message", log.Message(msg))
	}
}

func (ctx *localContext) handleRestart(msg *Restart) {
	ctx.stopping = false
	ctx.restarting = true
	ctx.InvokeUserMessage(restartingMessage)
	ctx.children.ForEach(func(_ int, pid PID) {
		pid.Stop()
	})
	ctx.tryRestartOrTerminate()
}

//I am stopping
func (ctx *localContext) handleStop(msg *Stop) {
	ctx.stopping = true
	ctx.restarting = false

	ctx.InvokeUserMessage(stoppingMessage)
	ctx.children.ForEach(func(_ int, pid PID) {
		pid.Stop()
	})
	ctx.tryRestartOrTerminate()
}

//child stopped, check if we can stop or restart (if needed)
func (ctx *localContext) handleTerminated(msg *Terminated) {
	ctx.children.Remove(msg.Who)
	ctx.watching.Remove(msg.Who)

	ctx.InvokeUserMessage(msg)
	ctx.tryRestartOrTerminate()
}

//offload the supervision completely to the supervisor strategy
func (ctx *localContext) handleFailure(msg *Failure) {
	if strategy, ok := ctx.actor.(SupervisorStrategy); ok {
		strategy.HandleFailure(ctx, msg.Who, msg.RestartStats, msg.Reason, msg.Message)
		return
	}
	ctx.supervisor.HandleFailure(ctx, msg.Who, msg.RestartStats, msg.Reason, msg.Message)
}

func (ctx *localContext) tryRestartOrTerminate() {
	if ctx.t != nil {
		ctx.t.Stop()
		ctx.t = nil
		ctx.receiveTimeout = 0
	}

	if !ctx.children.Empty() {
		return
	}

	if ctx.restarting {
		ctx.restart()
		return
	}

	if ctx.stopping {
		ctx.stopped()
	}
}

func (ctx *localContext) restart() {
	ctx.incarnateActor()
	ctx.InvokeUserMessage(startedMessage)
	ctx.RestartStats().Restart()
	if ctx.stash != nil {
		for !ctx.stash.Empty() {
			msg, _ := ctx.stash.Pop()
			ctx.InvokeUserMessage(msg)
		}
	}
	ctx.self.sendSystemMessage(resumeMailboxMessage)
}

func (ctx *localContext) stopped() {
	ProcessRegistry.Remove(ctx.self)
	ctx.InvokeUserMessage(stoppedMessage)
	otherStopped := &Terminated{Who: ctx.self}
	ctx.watchers.ForEach(func(i int, pid PID) {
		pid.sendSystemMessage(otherStopped)
	})
}

func (ctx *localContext) SetBehavior(behavior ActorFunc) {
	ctx.behavior.Clear()
	ctx.receive = behavior
}

func (ctx *localContext) PushBehavior(behavior ActorFunc) {
	ctx.behavior.Push(ctx.receive)
	ctx.receive = behavior
}

func (ctx *localContext) PopBehavior() {
	if ctx.behavior.Len() == 0 {
		panic("Cannot unbecome actor base behavior")
	}
	ctx.receive, _ = ctx.behavior.Pop()
}

func (ctx *localContext) Watch(who *PID) {
	who.sendSystemMessage(&Watch{
		Watcher: ctx.self,
	})
	ctx.watching.Add(who)
}

func (ctx *localContext) Unwatch(who *PID) {
	who.sendSystemMessage(&Unwatch{
		Watcher: ctx.self,
	})
	ctx.watching.Remove(who)
}

func (ctx *localContext) Respond(response interface{}) {
	ctx.Sender().Tell(response)
}

func (ctx *localContext) Spawn(props *Props) *PID {
	pid, _ := ctx.SpawnNamed(props, ProcessRegistry.NextId())
	return pid
}

func (ctx *localContext) SpawnPrefix(props *Props, prefix string) *PID {
	pid, _ := ctx.SpawnNamed(props, prefix+ProcessRegistry.NextId())
	return pid
}

func (ctx *localContext) SpawnNamed(props *Props, name string) (*PID, error) {
	pid, err := props.spawn(ctx.self.Id+"/"+name, ctx.self)
	if err != nil {
		return pid, err
	}

	ctx.children.Add(pid)
	ctx.Watch(pid)

	return pid, nil
}

func (ctx *localContext) GoString() string {
	return ctx.self.String()
}

func (ctx *localContext) String() string {
	return ctx.self.String()
}

func (ctx *localContext) AwaitFuture(f *Future, cont func(res interface{}, err error)) {
	wrapper := func() {
		cont(f.result, f.err)
	}

	//invoke the callback when the future completes
	f.continueWith(func(res interface{}, err error) {
		//send the wrapped callaback as a continuation message to self
		ctx.self.sendSystemMessage(&continuation{
			f:       wrapper,
			message: ctx.message,
		})
	})
}

func (*localContext) RestartChildren(pids ...*PID) {
	for _, pid := range pids {
		pid.sendSystemMessage(restartMessage)
	}
}

func (*localContext) StopChildren(pids ...*PID) {
	for _, pid := range pids {
		pid.sendSystemMessage(stopMessage)
	}
}

func (*localContext) ResumeChildren(pids ...*PID) {
	for _, pid := range pids {
		pid.sendSystemMessage(resumeMailboxMessage)
	}
}
