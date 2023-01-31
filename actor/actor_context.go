package actor

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/asynkron/protoactor-go/ctxext"
	"github.com/asynkron/protoactor-go/log"
	"github.com/asynkron/protoactor-go/metrics"
	"github.com/emirpasic/gods/stacks/linkedliststack"
	"go.opentelemetry.io/otel/attribute"
)

const (
	stateAlive int32 = iota
	stateRestarting
	stateStopping
	stateStopped
)

type actorContextExtras struct {
	children            PIDSet
	receiveTimeoutTimer *time.Timer
	rs                  *RestartStatistics
	stash               *linkedliststack.Stack
	watchers            PIDSet
	context             Context
	extensions          *ctxext.ContextExtensions
}

func newActorContextExtras(context Context) *actorContextExtras {
	this := &actorContextExtras{
		context:    context,
		extensions: ctxext.NewContextExtensions(),
	}

	return this
}

func (ctxExt *actorContextExtras) restartStats() *RestartStatistics {
	// lazy initialize the child restart stats if this is the first time
	// further mutations are handled within "restart"
	if ctxExt.rs == nil {
		ctxExt.rs = NewRestartStatistics()
	}

	return ctxExt.rs
}

func (ctxExt *actorContextExtras) initReceiveTimeoutTimer(timer *time.Timer) {
	ctxExt.receiveTimeoutTimer = timer
}

func (ctxExt *actorContextExtras) resetReceiveTimeoutTimer(time time.Duration) {
	if ctxExt.receiveTimeoutTimer == nil {
		return
	}

	ctxExt.receiveTimeoutTimer.Reset(time)
}

func (ctxExt *actorContextExtras) stopReceiveTimeoutTimer() {
	if ctxExt.receiveTimeoutTimer == nil {
		return
	}

	ctxExt.receiveTimeoutTimer.Stop()
}

func (ctxExt *actorContextExtras) killReceiveTimeoutTimer() {
	if ctxExt.receiveTimeoutTimer == nil {
		return
	}

	ctxExt.receiveTimeoutTimer.Stop()
	ctxExt.receiveTimeoutTimer = nil
}

func (ctxExt *actorContextExtras) addChild(pid *PID) {
	ctxExt.children.Add(pid)
}

func (ctxExt *actorContextExtras) removeChild(pid *PID) {
	ctxExt.children.Remove(pid)
}

func (ctxExt *actorContextExtras) watch(watcher *PID) {
	ctxExt.watchers.Add(watcher)
}

func (ctxExt *actorContextExtras) unwatch(watcher *PID) {
	ctxExt.watchers.Remove(watcher)
}

type actorContext struct {
	actor             Actor
	actorSystem       *ActorSystem
	extras            *actorContextExtras
	props             *Props
	parent            *PID
	self              *PID
	receiveTimeout    time.Duration
	messageOrEnvelope interface{}
	state             int32
}

var (
	_ SenderContext   = &actorContext{}
	_ ReceiverContext = &actorContext{}
	_ SpawnerContext  = &actorContext{}
	_ basePart        = &actorContext{}
	_ stopperPart     = &actorContext{}
)

func newActorContext(actorSystem *ActorSystem, props *Props, parent *PID) *actorContext {
	this := &actorContext{
		parent:      parent,
		props:       props,
		actorSystem: actorSystem,
	}

	this.incarnateActor()

	return this
}

func (ctx *actorContext) ensureExtras() *actorContextExtras {
	if ctx.extras == nil {
		ctxd := Context(ctx)
		if ctx.props != nil && ctx.props.contextDecoratorChain != nil {
			ctxd = ctx.props.contextDecoratorChain(ctxd)
		}

		ctx.extras = newActorContextExtras(ctxd)
	}

	return ctx.extras
}

//
// Interface: Context
//

func (ctx *actorContext) ActorSystem() *ActorSystem {
	return ctx.actorSystem
}

func (ctx *actorContext) Parent() *PID {
	return ctx.parent
}

func (ctx *actorContext) Self() *PID {
	return ctx.self
}

func (ctx *actorContext) Sender() *PID {
	return UnwrapEnvelopeSender(ctx.messageOrEnvelope)
}

func (ctx *actorContext) Actor() Actor {
	return ctx.actor
}

func (ctx *actorContext) ReceiveTimeout() time.Duration {
	return ctx.receiveTimeout
}

func (ctx *actorContext) Children() []*PID {
	if ctx.extras == nil {
		return make([]*PID, 0)
	}

	return ctx.extras.children.Values()
}

func (ctx *actorContext) Respond(response interface{}) {
	// If the message is addressed to nil forward it to the dead letter channel
	if ctx.Sender() == nil {
		ctx.actorSystem.DeadLetter.SendUserMessage(nil, response)

		return
	}

	ctx.Send(ctx.Sender(), response)
}

func (ctx *actorContext) Stash() {
	extra := ctx.ensureExtras()
	if extra.stash == nil {
		extra.stash = linkedliststack.New()
	}

	extra.stash.Push(ctx.Message())
}

func (ctx *actorContext) Watch(who *PID) {
	who.sendSystemMessage(ctx.actorSystem, &Watch{
		Watcher: ctx.self,
	})
}

func (ctx *actorContext) Unwatch(who *PID) {
	who.sendSystemMessage(ctx.actorSystem, &Unwatch{
		Watcher: ctx.self,
	})
}

func (ctx *actorContext) SetReceiveTimeout(d time.Duration) {
	if d <= 0 {
		panic("Duration must be greater than zero")
	}

	if d == ctx.receiveTimeout {
		return
	}

	if d < time.Millisecond {
		// anything less than 1 millisecond is set to zero
		d = 0
	}

	ctx.receiveTimeout = d

	ctx.ensureExtras()
	ctx.extras.stopReceiveTimeoutTimer()

	if d > 0 {
		if ctx.extras.receiveTimeoutTimer == nil {
			ctx.extras.initReceiveTimeoutTimer(time.AfterFunc(d, ctx.receiveTimeoutHandler))
		} else {
			ctx.extras.resetReceiveTimeoutTimer(d)
		}
	}
}

func (ctx *actorContext) CancelReceiveTimeout() {
	if ctx.extras == nil || ctx.extras.receiveTimeoutTimer == nil {
		return
	}

	ctx.extras.killReceiveTimeoutTimer()
	ctx.receiveTimeout = 0
}

func (ctx *actorContext) receiveTimeoutHandler() {
	if ctx.extras != nil && ctx.extras.receiveTimeoutTimer != nil {
		ctx.CancelReceiveTimeout()
		ctx.Send(ctx.self, receiveTimeoutMessage)
	}
}

func (ctx *actorContext) Forward(pid *PID) {
	if msg, ok := ctx.messageOrEnvelope.(SystemMessage); ok {
		// SystemMessage cannot be forwarded
		plog.Error("SystemMessage cannot be forwarded", log.Message(msg))

		return
	}

	ctx.sendUserMessage(pid, ctx.messageOrEnvelope)
}

func (ctx *actorContext) ReenterAfter(f *Future, cont func(res interface{}, err error)) {
	wrapper := func() {
		cont(f.result, f.err)
	}

	message := ctx.messageOrEnvelope
	// invoke the callback when the future completes
	f.continueWith(func(res interface{}, err error) {
		// send the wrapped callback as a continuation message to self
		ctx.self.sendSystemMessage(ctx.actorSystem, &continuation{
			f:       wrapper,
			message: message,
		})
	})
}

//
// Interface: sender
//

func (ctx *actorContext) Message() interface{} {
	return UnwrapEnvelopeMessage(ctx.messageOrEnvelope)
}

func (ctx *actorContext) MessageHeader() ReadonlyMessageHeader {
	return UnwrapEnvelopeHeader(ctx.messageOrEnvelope)
}

func (ctx *actorContext) Send(pid *PID, message interface{}) {
	ctx.sendUserMessage(pid, message)
}

func (ctx *actorContext) sendUserMessage(pid *PID, message interface{}) {
	if ctx.props.senderMiddlewareChain != nil {
		ctx.props.senderMiddlewareChain(ctx.ensureExtras().context, pid, WrapEnvelope(message))
	} else {
		pid.sendUserMessage(ctx.actorSystem, message)
	}
}

func (ctx *actorContext) Request(pid *PID, message interface{}) {
	env := &MessageEnvelope{
		Header:  nil,
		Message: message,
		Sender:  ctx.Self(),
	}

	ctx.sendUserMessage(pid, env)
}

func (ctx *actorContext) RequestWithCustomSender(pid *PID, message interface{}, sender *PID) {
	env := &MessageEnvelope{
		Header:  nil,
		Message: message,
		Sender:  sender,
	}
	ctx.sendUserMessage(pid, env)
}

func (ctx *actorContext) RequestFuture(pid *PID, message interface{}, timeout time.Duration) *Future {
	future := NewFuture(ctx.actorSystem, timeout)
	env := &MessageEnvelope{
		Header:  nil,
		Message: message,
		Sender:  future.PID(),
	}
	ctx.sendUserMessage(pid, env)

	return future
}

//
// Interface: receiver
//

func (ctx *actorContext) Receive(envelope *MessageEnvelope) {
	ctx.messageOrEnvelope = envelope
	ctx.defaultReceive()
	ctx.messageOrEnvelope = nil
}

func (ctx *actorContext) defaultReceive() {
	switch msg := ctx.Message().(type) {
	case *PoisonPill:
		ctx.Stop(ctx.self)

	case AutoRespond:
		if ctx.props.contextDecoratorChain != nil {
			ctx.actor.Receive(ctx.ensureExtras().context)
		} else {
			ctx.actor.Receive(ctx)
		}

		res := msg.GetAutoResponse(ctx)
		ctx.Respond(res)

	default:
		// are we using decorators, if so, ensure it has been created
		if ctx.props.contextDecoratorChain != nil {
			ctx.actor.Receive(ctx.ensureExtras().context)

			return
		}

		ctx.actor.Receive(ctx)
	}
}

//
// Interface: spawner
//

func (ctx *actorContext) Spawn(props *Props) *PID {
	pid, err := ctx.SpawnNamed(props, ctx.actorSystem.ProcessRegistry.NextId())
	if err != nil {
		panic(err)
	}

	return pid
}

func (ctx *actorContext) SpawnPrefix(props *Props, prefix string) *PID {
	pid, err := ctx.SpawnNamed(props, prefix+ctx.actorSystem.ProcessRegistry.NextId())
	if err != nil {
		panic(err)
	}

	return pid
}

func (ctx *actorContext) SpawnNamed(props *Props, name string) (*PID, error) {
	if props.guardianStrategy != nil {
		panic(errors.New("props used to spawn child cannot have GuardianStrategy"))
	}

	var pid *PID

	var err error

	if ctx.props.spawnMiddlewareChain != nil {
		pid, err = ctx.props.spawnMiddlewareChain(ctx.actorSystem, ctx.self.Id+"/"+name, props, ctx)
	} else {
		pid, err = props.spawn(ctx.actorSystem, ctx.self.Id+"/"+name, ctx)
	}

	if err != nil {
		return pid, err
	}

	ctx.ensureExtras().addChild(pid)

	return pid, nil
}

//
// Interface: stopper
//

// Stop will stop actor immediately regardless of existing user messages in mailbox.
func (ctx *actorContext) Stop(pid *PID) {
	if ctx.actorSystem.Config.MetricsProvider != nil {
		metricsSystem, ok := ctx.actorSystem.Extensions.Get(extensionId).(*Metrics)
		if ok && metricsSystem.enabled {
			_ctx := context.Background()
			if instruments := metricsSystem.metrics.Get(metrics.InternalActorMetrics); instruments != nil {
				instruments.ActorStoppedCount.Add(_ctx, 1, metricsSystem.CommonLabels(ctx)...)
			}
		}
	}

	pid.ref(ctx.actorSystem).Stop(pid)
}

// StopFuture will stop actor immediately regardless of existing user messages in mailbox, and return its future.
func (ctx *actorContext) StopFuture(pid *PID) *Future {
	future := NewFuture(ctx.actorSystem, 10*time.Second)

	pid.sendSystemMessage(ctx.actorSystem, &Watch{Watcher: future.pid})
	ctx.Stop(pid)

	return future
}

// Poison will tell actor to stop after processing current user messages in mailbox.
func (ctx *actorContext) Poison(pid *PID) {
	pid.sendUserMessage(ctx.actorSystem, poisonPillMessage)
}

// PoisonFuture will tell actor to stop after processing current user messages in mailbox, and return its future.
func (ctx *actorContext) PoisonFuture(pid *PID) *Future {
	future := NewFuture(ctx.actorSystem, 10*time.Second)

	pid.sendSystemMessage(ctx.actorSystem, &Watch{Watcher: future.pid})
	ctx.Poison(pid)

	return future
}

//
// Interface: MessageInvoker
//

func (ctx *actorContext) InvokeUserMessage(md interface{}) {
	if atomic.LoadInt32(&ctx.state) == stateStopped {
		// already stopped
		return
	}

	influenceTimeout := true
	if ctx.receiveTimeout > 0 {
		_, influenceTimeout = md.(NotInfluenceReceiveTimeout)
		influenceTimeout = !influenceTimeout

		if influenceTimeout {
			ctx.extras.stopReceiveTimeoutTimer()
		}
	}

	systemMetrics, ok := ctx.actorSystem.Extensions.Get(extensionId).(*Metrics)
	if ok && systemMetrics.enabled {
		t := time.Now()

		ctx.processMessage(md)

		delta := time.Since(t)
		_ctx := context.Background()

		if instruments := systemMetrics.metrics.Get(metrics.InternalActorMetrics); instruments != nil {
			histogram := instruments.ActorMessageReceiveHistogram

			labels := append(
				systemMetrics.CommonLabels(ctx),
				attribute.String("messagetype", fmt.Sprintf("%T", md)),
			)
			histogram.Record(_ctx, delta.Seconds(), labels...)
		}
	} else {
		ctx.processMessage(md)
	}

	if ctx.receiveTimeout > 0 && influenceTimeout {
		ctx.extras.resetReceiveTimeoutTimer(ctx.receiveTimeout)
	}
}

func (ctx *actorContext) processMessage(m interface{}) {
	if ctx.props.receiverMiddlewareChain != nil {
		ctx.props.receiverMiddlewareChain(ctx.ensureExtras().context, WrapEnvelope(m))

		return
	}

	if ctx.props.contextDecoratorChain != nil {
		ctx.ensureExtras().context.Receive(WrapEnvelope(m))

		return
	}

	ctx.messageOrEnvelope = m
	ctx.defaultReceive()
	ctx.messageOrEnvelope = nil // release message
}

func (ctx *actorContext) incarnateActor() {
	atomic.StoreInt32(&ctx.state, stateAlive)
	ctx.actor = ctx.props.producer()

	metricsSystem, ok := ctx.actorSystem.Extensions.Get(extensionId).(*Metrics)
	if ok && metricsSystem.enabled {
		_ctx := context.Background()
		if instruments := metricsSystem.metrics.Get(metrics.InternalActorMetrics); instruments != nil {
			instruments.ActorSpawnCount.Add(_ctx, 1, metricsSystem.CommonLabels(ctx)...)		}
	}
}

func (ctx *actorContext) InvokeSystemMessage(message interface{}) {
	//goland:noinspection GrazieInspection
	switch msg := message.(type) {
	case *continuation:
		ctx.messageOrEnvelope = msg.message // apply the message that was present when we started the await
		msg.f()                             // invoke the continuation in the current actor context

		ctx.messageOrEnvelope = nil // release the message
	case *Started:
		ctx.InvokeUserMessage(msg) // forward
	case *Watch:
		ctx.handleWatch(msg)
	case *Unwatch:
		ctx.handleUnwatch(msg)
	case *Stop:
		ctx.handleStop()
	case *Terminated:
		ctx.handleTerminated(msg)
	case *Failure:
		ctx.handleFailure(msg)
	case *Restart:
		ctx.handleRestart()
	default:
		plog.Error("unknown system message", log.Message(msg))
	}
}

func (ctx *actorContext) handleRootFailure(failure *Failure) {
	defaultSupervisionStrategy.HandleFailure(ctx.actorSystem, ctx, failure.Who, failure.RestartStats, failure.Reason, failure.Message)
}

func (ctx *actorContext) handleWatch(msg *Watch) {
	if atomic.LoadInt32(&ctx.state) >= stateStopping {
		msg.Watcher.sendSystemMessage(ctx.actorSystem, &Terminated{
			Who: ctx.self,
		})
	} else {
		ctx.ensureExtras().watch(msg.Watcher)
	}
}

func (ctx *actorContext) handleUnwatch(msg *Unwatch) {
	if ctx.extras == nil {
		return
	}

	ctx.extras.unwatch(msg.Watcher)
}

func (ctx *actorContext) handleRestart() {
	atomic.StoreInt32(&ctx.state, stateRestarting)
	ctx.InvokeUserMessage(restartingMessage)
	ctx.stopAllChildren()
	ctx.tryRestartOrTerminate()

	metricsSystem, ok := ctx.actorSystem.Extensions.Get(extensionId).(*Metrics)
	if ok && metricsSystem.enabled {
		_ctx := context.Background()
		if instruments := metricsSystem.metrics.Get(metrics.InternalActorMetrics); instruments != nil {
			instruments.ActorRestartedCount.Add(_ctx, 1, metricsSystem.CommonLabels(ctx)...)
		}
	}
}

// I am stopping.
func (ctx *actorContext) handleStop() {
	if atomic.LoadInt32(&ctx.state) >= stateStopping {
		// already stopping or stopped
		return
	}

	atomic.StoreInt32(&ctx.state, stateStopping)

	ctx.InvokeUserMessage(stoppingMessage)
	ctx.stopAllChildren()
	ctx.tryRestartOrTerminate()
}

// child stopped, check if we can stop or restart (if needed).
func (ctx *actorContext) handleTerminated(msg *Terminated) {
	if ctx.extras != nil {
		ctx.extras.removeChild(msg.Who)
	}

	ctx.InvokeUserMessage(msg)
	ctx.tryRestartOrTerminate()
}

// offload the supervision completely to the supervisor strategy.
func (ctx *actorContext) handleFailure(msg *Failure) {
	if strategy, ok := ctx.actor.(SupervisorStrategy); ok {
		strategy.HandleFailure(ctx.actorSystem, ctx, msg.Who, msg.RestartStats, msg.Reason, msg.Message)

		return
	}

	ctx.props.getSupervisor().HandleFailure(ctx.actorSystem, ctx, msg.Who, msg.RestartStats, msg.Reason, msg.Message)
}

func (ctx *actorContext) stopAllChildren() {
	if ctx.extras == nil {
		return
	}

	ctx.extras.children.ForEach(func(_ int, pid *PID) {
		ctx.Stop(pid)
	})
}

func (ctx *actorContext) tryRestartOrTerminate() {
	if ctx.extras != nil && !ctx.extras.children.Empty() {
		return
	}

	switch atomic.LoadInt32(&ctx.state) {
	case stateRestarting:
		ctx.CancelReceiveTimeout()
		ctx.restart()
	case stateStopping:
		ctx.CancelReceiveTimeout()
		ctx.finalizeStop()
	}
}

func (ctx *actorContext) restart() {
	ctx.incarnateActor()
	ctx.self.sendSystemMessage(ctx.actorSystem, resumeMailboxMessage)
	ctx.InvokeUserMessage(startedMessage)

	if ctx.extras != nil && ctx.extras.stash != nil {
		for !ctx.extras.stash.Empty() {
			msg, _ := ctx.extras.stash.Pop()
			ctx.InvokeUserMessage(msg)
		}
	}
}

func (ctx *actorContext) finalizeStop() {
	ctx.actorSystem.ProcessRegistry.Remove(ctx.self)
	ctx.InvokeUserMessage(stoppedMessage)

	otherStopped := &Terminated{Who: ctx.self}
	// Notify watchers
	if ctx.extras != nil {
		ctx.extras.watchers.ForEach(func(i int, pid *PID) {
			pid.sendSystemMessage(ctx.actorSystem, otherStopped)
		})
	}
	// Notify parent
	if ctx.parent != nil {
		ctx.parent.sendSystemMessage(ctx.actorSystem, otherStopped)
	}

	atomic.StoreInt32(&ctx.state, stateStopped)
}

//
// Interface: Supervisor
//

func (ctx *actorContext) EscalateFailure(reason interface{}, message interface{}) {
	// debug setting, allows to output supervision failures in console/error level
	if ctx.actorSystem.Config.DeveloperSupervisionLogging {
		fmt.Println("[Supervision] Actor:", ctx.self, " failed with message:", message, " exception:", reason)
		plog.Error("[Supervision]", log.Stringer("actor", ctx.self), log.Object("message", message), log.Object("exception", reason))
	}

	metricsSystem, ok := ctx.actorSystem.Extensions.Get(extensionId).(*Metrics)
	if ok && metricsSystem.enabled {
		_ctx := context.Background()
		if instruments := metricsSystem.metrics.Get(metrics.InternalActorMetrics); instruments != nil {
			instruments.ActorFailureCount.Add(_ctx, 1, metricsSystem.CommonLabels(ctx)...)
		}
	}

	failure := &Failure{Reason: reason, Who: ctx.self, RestartStats: ctx.ensureExtras().restartStats(), Message: message}

	ctx.self.sendSystemMessage(ctx.actorSystem, suspendMailboxMessage)

	if ctx.parent == nil {
		ctx.handleRootFailure(failure)
	} else {
		ctx.parent.sendSystemMessage(ctx.actorSystem, failure)
	}
}

func (ctx *actorContext) RestartChildren(pids ...*PID) {
	for _, pid := range pids {
		pid.sendSystemMessage(ctx.actorSystem, restartMessage)
	}
}

func (ctx *actorContext) StopChildren(pids ...*PID) {
	for _, pid := range pids {
		pid.sendSystemMessage(ctx.actorSystem, stopMessage)
	}
}

func (ctx *actorContext) ResumeChildren(pids ...*PID) {
	for _, pid := range pids {
		pid.sendSystemMessage(ctx.actorSystem, resumeMailboxMessage)
	}
}

//
// Miscellaneous
//

func (ctx *actorContext) GoString() string {
	return ctx.self.String()
}

func (ctx *actorContext) String() string {
	return ctx.self.String()
}

func (ctx *actorContext) Get(id ctxext.ContextExtensionID) ctxext.ContextExtension {
	extras := ctx.ensureExtras()
	ext := extras.extensions.Get(id)

	return ext
}

func (ctx *actorContext) Set(ext ctxext.ContextExtension) {
	extras := ctx.ensureExtras()
	extras.extensions.Set(ext)
}
