package actor

import (
	"errors"
	"time"

	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/emirpasic/gods/stacks/linkedliststack"
)

type contextState int32

const (
	stateNone contextState = iota
	stateAlive
	stateRestarting
	stateStopping
	stateStopped
)

type localContext struct {
	message            interface{}
	parent             *PID
	self               *PID
	actor              Actor
	supervisor         SupervisorStrategy
	producer           Producer
	inboundMiddleware  ActorFunc
	outboundMiddleware SenderFunc
	behavior           behaviorStack
	receive            ActorFunc
	children           PIDSet
	watchers           PIDSet
	watching           PIDSet
	stash              *linkedliststack.Stack
	state              contextState
	receiveTimeout     time.Duration
	t                  *time.Timer
	restartStats       *RestartStatistics
}

func newLocalContext(producer Producer, supervisor SupervisorStrategy, inboundMiddleware []InboundMiddleware, outboundMiddleware []OutboundMiddleware, parent *PID) *localContext {
	this := &localContext{
		parent:     parent,
		producer:   producer,
		supervisor: supervisor,
	}

	// Construct the inbound middleware chain with the final receiver at the end
	if inboundMiddleware != nil {
		this.inboundMiddleware = makeInboundMiddlewareChain(inboundMiddleware, func(ctx Context) {
			if _, ok := this.message.(*PoisonPill); ok {
				this.self.Stop()
			} else {
				this.receive(ctx)
			}
		})
	}

	// Construct the outbound middleware chain with the final sender at the end
	this.outboundMiddleware = makeOutboundMiddlewareChain(outboundMiddleware, func(_ Context, target *PID, envelope *MessageEnvelope) {
		target.ref().SendUserMessage(target, envelope)
	})

	this.incarnateActor()
	return this
}

func (ctx *localContext) Actor() Actor {
	return ctx.actor
}

func (ctx *localContext) Message() interface{} {
	envelope, ok := ctx.message.(*MessageEnvelope)
	if ok {
		return envelope.Message
	}
	return ctx.message
}

func (ctx *localContext) Sender() *PID {
	envelope, ok := ctx.message.(*MessageEnvelope)
	if ok {
		return envelope.Sender
	}
	return nil
}

func (ctx *localContext) MessageHeader() ReadonlyMessageHeader {
	envelope, ok := ctx.message.(*MessageEnvelope)
	if ok {
		if envelope.Header != nil {
			return envelope.Header
		}
	}
	return emptyMessageHeader
}

func (ctx *localContext) Tell(pid *PID, message interface{}) {
	ctx.sendUserMessage(pid, message)
}

func (ctx *localContext) Forward(pid *PID) {
	if msg, ok := ctx.Message().(SystemMessage); ok {
		// SystemMessage cannot be forwarded
		plog.Error("SystemMessage cannot be forwarded", log.Message(msg))
		return
	}
	ctx.sendUserMessage(pid, ctx.message)
}

func (ctx *localContext) sendUserMessage(pid *PID, message interface{}) {
	if ctx.outboundMiddleware != nil {
		if env, ok := message.(*MessageEnvelope); ok {
			ctx.outboundMiddleware(ctx, pid, env)
		} else {
			ctx.outboundMiddleware(ctx, pid, &MessageEnvelope{
				Header:  nil,
				Message: message,
				Sender:  nil,
			})
		}
	} else {
		pid.ref().SendUserMessage(pid, message)
	}
}

func (ctx *localContext) Request(pid *PID, message interface{}) {
	env := &MessageEnvelope{
		Header:  nil,
		Message: message,
		Sender:  ctx.Self(),
	}

	ctx.sendUserMessage(pid, env)
}

func (ctx *localContext) RequestFuture(pid *PID, message interface{}, timeout time.Duration) *Future {
	future := NewFuture(timeout)
	env := &MessageEnvelope{
		Header:  nil,
		Message: message,
		Sender:  future.PID(),
	}
	ctx.sendUserMessage(pid, env)

	return future
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
		ctx.receiveTimeout = 0
	}
}

func (ctx *localContext) receiveTimeoutHandler() {
	if ctx.t != nil {
		ctx.cancelTimer()
		ctx.self.Tell(receiveTimeoutMessage)
	}
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
		ctx.restartStats = NewRestartStatistics()
	}
	return ctx.restartStats
}

func (ctx *localContext) EscalateFailure(reason interface{}, message interface{}) {
	failure := &Failure{Reason: reason, Who: ctx.self, RestartStats: ctx.RestartStats()}
	ctx.self.sendSystemMessage(suspendMailboxMessage)
	if ctx.parent == nil {
		handleRootFailure(failure)
	} else {
		//TODO: Akka recursively suspends all children also on failure
		//Not sure if I think this is the right way to go, why do children need to wait for their parents failed state to recover?
		ctx.parent.sendSystemMessage(failure)
	}
}

func (ctx *localContext) InvokeUserMessage(md interface{}) {
	if ctx.state == stateStopped {
		//already stopped
		return
	}

	influenceTimeout := true
	if ctx.receiveTimeout > 0 {
		_, influenceTimeout = md.(NotInfluenceReceiveTimeout)

		envelope, ok := md.(*MessageEnvelope)
		if ok {
			_, influenceTimeout = envelope.Message.(NotInfluenceReceiveTimeout)
		}

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

func (ctx *localContext) processMessage(m interface{}) {
	ctx.message = m

	if ctx.inboundMiddleware != nil {
		ctx.inboundMiddleware(ctx)
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
	pid := ctx.producer()
	ctx.state = stateAlive
	ctx.actor = pid
	ctx.receive = pid.Receive
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
		ctx.handleWatch(msg)
	case *Unwatch:
		ctx.handleUnwatch(msg)
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

func (ctx *localContext) handleWatch(msg *Watch) {
	if ctx.state >= stateStopping {
		msg.Watcher.sendSystemMessage(&Terminated{
			Who: ctx.self,
		})
	} else {
		ctx.watchers.Add(msg.Watcher)
	}
}

func (ctx *localContext) handleUnwatch(msg *Unwatch) {
	ctx.watchers.Remove(msg.Watcher)
}

func (ctx *localContext) handleRestart(msg *Restart) {
	ctx.state = stateRestarting
	ctx.InvokeUserMessage(restartingMessage)
	ctx.children.ForEach(func(_ int, pid PID) {
		pid.Stop()
	})
	ctx.tryRestartOrTerminate()
}

//I am stopping
func (ctx *localContext) handleStop(msg *Stop) {
	if ctx.state >= stateStopping {
		//already stopping or stopped
		return
	}

	ctx.state = stateStopping

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

	switch ctx.state {
	case stateRestarting:
		ctx.restart()
	case stateStopping:
		ctx.stopped()
	}
}

func (ctx *localContext) restart() {
	ctx.incarnateActor()
	ctx.InvokeUserMessage(startedMessage)
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
	ctx.state = stateStopped
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
	// If the message is addressed to nil forward it to the dead letter channel
	if ctx.Sender() == nil {
		deadLetter.SendUserMessage(nil, response)
		return
	}

	ctx.Tell(ctx.Sender(), response)
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
	if props.guardianStrategy != nil {
		panic(errors.New("Props used to spawn child cannot have GuardianStrategy"))
	}

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

	message := ctx.message
	//invoke the callback when the future completes
	f.continueWith(func(res interface{}, err error) {
		//send the wrapped callaback as a continuation message to self
		ctx.self.sendSystemMessage(&continuation{
			f:       wrapper,
			message: message,
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
