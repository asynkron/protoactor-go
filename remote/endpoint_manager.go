package remote

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/eventstream"
)

type endpointLazy struct {
	// valueFunc func() *endpoint
	unloaded atomic.Bool
	once     sync.Once
	endpoint atomic.Value
	manager  *endpointManager
	address  string
}

// newEndpointLazy creates an endpoint that connects to the remote address on first use.
func newEndpointLazy(em *endpointManager, address string) *endpointLazy {
	return &endpointLazy{
		manager: em,
		address: address,
	}
}

func (el *endpointLazy) connect() {
	el.manager.remote.actorSystem.Logger().Debug("connecting to remote address", slog.String("address", el.address))
	em := el.manager
	system := em.remote.actorSystem
	rst, err := system.Root.RequestFuture(em.endpointSupervisor, el.address, -1).Result()
	if err != nil {
		system.Logger().Error("failed to connect to remote address", slog.String("address", el.address), slog.Any("error", err))
		return
	}
	ep, ok := rst.(*endpoint)
	if !ok {
		system.Logger().Error("failed to connect to remote address: unexpected response type", slog.String("address", el.address))
		return
	}
	el.Set(ep)
}

func (el *endpointLazy) Set(ep *endpoint) {
	el.endpoint.Store(ep)
}

func (el *endpointLazy) Get() *endpoint {
	el.once.Do(el.connect)
	ep, _ := el.endpoint.Load().(*endpoint)
	return ep
}

type endpoint struct {
	writer  *actor.PID
	watcher *actor.PID
}

func (ep *endpoint) Address() string {
	return ep.watcher.GetAddress()
}

type endpointManager struct {
	connections               *sync.Map
	remote                    *Remote
	endpointSub               *eventstream.Subscription
	endpointSupervisor        *actor.PID
	activator                 *actor.PID
	stopped                   atomic.Bool
	endpointReaderConnections *sync.Map
}

func newEndpointManager(r *Remote) *endpointManager {
	em := &endpointManager{
		connections:               &sync.Map{},
		remote:                    r,
		endpointReaderConnections: &sync.Map{},
	}
	em.stopped.Store(false)
	return em
}

func (em *endpointManager) start() error {
	eventStream := em.remote.actorSystem.EventStream
	em.endpointSub = eventStream.
		SubscribeWithPredicate(em.endpointEvent, func(m any) bool {
			switch m.(type) {
			case *EndpointTerminatedEvent, *EndpointConnectedEvent:
				return true
			}
			return false
		})
	if err := em.startActivator(); err != nil {
		return err
	}
	if err := em.startSupervisor(); err != nil {
		return err
	}

	if err := em.waiting(3 * time.Second); err != nil {
		return err
	}

	em.remote.Logger().Info("Started EndpointManager")
	return nil
}

func (em *endpointManager) waiting(timeout time.Duration) error {
	ctx := em.remote.actorSystem.Root
	if _, err := ctx.RequestFuture(em.activator, &Ping{}, timeout).Result(); err != nil {
		return err
	}
	return nil
}

func (em *endpointManager) stop() {
	em.stopped.Store(true)
	r := em.remote
	r.actorSystem.EventStream.Unsubscribe(em.endpointSub)
	if err := em.stopActivator(); err != nil {
		em.remote.Logger().Error("stop endpoint activator failed", slog.Any("error", err))
	}
	if err := em.stopSupervisor(); err != nil {
		em.remote.Logger().Error("stop endpoint supervisor failed", slog.Any("error", err))
	}
	em.endpointSub = nil
	em.connections = nil
	if em.endpointReaderConnections != nil {
		em.endpointReaderConnections.Range(func(key any, value any) bool {
			if channel, ok := value.(chan bool); ok {
				channel <- true
			}
			em.endpointReaderConnections.Delete(key)
			return true
		})
	}
	em.remote.Logger().Info("Stopped EndpointManager")
}

func (em *endpointManager) startActivator() error {
	p := newActivatorActor(em.remote)
	props := actor.PropsFromProducer(p, actor.WithGuardian(actor.RestartingSupervisorStrategy()))
	pid, err := em.remote.actorSystem.Root.SpawnNamed(props, "activator")
	if err != nil {
		return fmt.Errorf("failed to start activator: %w", err)
	}
	em.activator = pid
	return nil
}

func (em *endpointManager) stopActivator() error {
	return em.remote.actorSystem.Root.StopFuture(em.activator).Wait()
}

func (em *endpointManager) startSupervisor() error {
	r := em.remote
	props := actor.PropsFromProducer(func() actor.Actor {
		return newEndpointSupervisor(r)
	},
		actor.WithGuardian(actor.RestartingSupervisorStrategy()),
		actor.WithDispatcher(actor.NewSynchronizedDispatcher(300)))

	pid, err := r.actorSystem.Root.SpawnNamed(props, "EndpointSupervisor")
	if err != nil {
		return fmt.Errorf("failed to start endpoint supervisor: %w", err)
	}
	em.endpointSupervisor = pid
	return nil
}

func (em *endpointManager) stopSupervisor() error {
	r := em.remote
	return r.actorSystem.Root.StopFuture(em.endpointSupervisor).Wait()
}

func (em *endpointManager) endpointEvent(evn any) {
	switch msg := evn.(type) {
	case *EndpointTerminatedEvent:
		em.remote.Logger().Debug("EndpointManager received endpoint terminated event, removing endpoint", slog.Any("message", evn))
		em.removeEndpoint(msg)
	case *EndpointConnectedEvent:
		endpoint := em.ensureConnected(msg.Address)
		if endpoint == nil {
			em.remote.Logger().Error("EndpointManager failed to handle endpoint connected event", slog.String("address", msg.Address))
			return
		}
		em.remote.actorSystem.Root.Send(endpoint.watcher, msg)
	}
}

func (em *endpointManager) remoteTerminate(msg *remoteTerminate) {
	if em.stopped.Load() {
		return
	}
	address := msg.Watchee.Address
	endpoint := em.ensureConnected(address)
	if endpoint == nil {
		terminated := &actor.Terminated{
			Who: msg.Watchee,
			Why: actor.TerminatedReason_Stopped,
		}
		if ref, ok := em.remote.actorSystem.ProcessRegistry.GetLocal(msg.Watcher.Id); ok {
			ref.SendSystemMessage(msg.Watcher, terminated)
		}
		return
	}
	em.remote.actorSystem.Root.Send(endpoint.watcher, msg)
}

func (em *endpointManager) remoteWatch(msg *remoteWatch) {
	if em.stopped.Load() {
		return
	}
	address := msg.Watchee.Address
	endpoint := em.ensureConnected(address)
	if endpoint == nil {
		terminated := &actor.Terminated{
			Who: msg.Watchee,
			Why: actor.TerminatedReason_AddressTerminated,
		}
		if ref, ok := em.remote.actorSystem.ProcessRegistry.GetLocal(msg.Watcher.Id); ok {
			ref.SendSystemMessage(msg.Watcher, terminated)
		}
		return
	}
	em.remote.actorSystem.Root.Send(endpoint.watcher, msg)
}

func (em *endpointManager) remoteUnwatch(msg *remoteUnwatch) {
	if em.stopped.Load() {
		return
	}
	address := msg.Watchee.Address
	endpoint := em.ensureConnected(address)
	if endpoint == nil {
		return
	}
	em.remote.actorSystem.Root.Send(endpoint.watcher, msg)
}

func (em *endpointManager) remoteDeliver(msg *remoteDeliver) {
	if em.stopped.Load() {
		// send to deadletter
		em.remote.actorSystem.EventStream.Publish(&actor.DeadLetterEvent{
			PID:     msg.target,
			Message: msg.message,
			Sender:  msg.sender,
		})
		return
	}
	address := msg.target.Address
	endpoint := em.ensureConnected(address)
	if endpoint == nil {
		em.remote.actorSystem.EventStream.Publish(&actor.DeadLetterEvent{
			PID:     msg.target,
			Message: msg.message,
			Sender:  msg.sender,
		})
		return
	}
	em.remote.actorSystem.Root.Send(endpoint.writer, msg)
}

func (em *endpointManager) ensureConnected(address string) *endpoint {
	e, ok := em.connections.Load(address)
	if !ok {
		el := newEndpointLazy(em, address)
		e, _ = em.connections.LoadOrStore(address, el)
	}
	el, ok := e.(*endpointLazy)
	if !ok {
		return nil
	}
	ep := el.Get()
	if ep == nil {
		em.connections.Delete(address)
	}
	return ep
}

// func (em *endpointManager) ensureConnected(address string) *endpoint {
// 	e, ok := em.connections.Load(address)
// 	if !ok {
// 		el := &endpointLazy{}
// 		var once sync.Once
// 		el.valueFunc = func() *endpoint {
// 			once.Do(func() {
// 				rst, _ := em.remote.actorSystem.Root.RequestFuture(em.endpointSupervisor, address, -1).Result()
// 				ep := rst.(*endpoint)
// 				el.valueFunc = func() *endpoint {
// 					return ep
// 				}
// 			})
// 			return el.valueFunc()
// 		}
// 		e, _ = em.connections.LoadOrStore(address, el)
// 	}

// 	el := e.(*endpointLazy)
// 	return el.valueFunc()
// }

func (em *endpointManager) removeEndpoint(msg *EndpointTerminatedEvent) {
	v, ok := em.connections.Load(msg.Address)
	if ok {
		le, ok := v.(*endpointLazy)
		if !ok {
			return
		}
		if le.unloaded.CompareAndSwap(false, true) {
			em.connections.Delete(msg.Address)
			ep := le.Get()
			if ep == nil {
				return
			}
			em.remote.Logger().Debug("Sending EndpointTerminatedEvent to EndpointWatcher and EndpointWriter", slog.String("address", msg.Address))
			em.remote.actorSystem.Root.Send(ep.watcher, msg)
			em.remote.actorSystem.Root.Send(ep.writer, msg)
		}
	}
}

type endpointSupervisor struct {
	remote         *Remote
	childAddresses map[string]string // PID.Id -> remote address
}

func newEndpointSupervisor(remote *Remote) actor.Actor {
	return &endpointSupervisor{
		remote:         remote,
		childAddresses: make(map[string]string),
	}
}

func (state *endpointSupervisor) Receive(ctx actor.Context) {
	if address, ok := ctx.Message().(string); ok {
		ctx.Logger().Debug("EndpointSupervisor spawning EndpointWriter and EndpointWatcher", slog.String("address", address))
		e := &endpoint{
			writer:  state.spawnEndpointWriter(state.remote, address, ctx),
			watcher: state.spawnEndpointWatcher(state.remote, address, ctx),
		}
		state.childAddresses[e.writer.Id] = address
		state.childAddresses[e.watcher.Id] = address
		ctx.Logger().Debug("id", slog.String("ewr", e.writer.Id), slog.String("ewa", e.watcher.Id))
		ctx.Respond(e)
	}
}

func (state *endpointSupervisor) HandleFailure(actorSystem *actor.ActorSystem, supervisor actor.Supervisor, child *actor.PID, rs *actor.RestartStatistics, reason any, message any) {
	actorSystem.Logger().Debug("EndpointSupervisor handling failure",
		slog.Any("reason", reason), slog.Any("message", message))

	if rs.NumberOfFailures(state.remote.config.SupervisorRestartWindow) > state.remote.config.SupervisorMaxRestarts {
		actorSystem.Logger().Warn("EndpointSupervisor stopping child after too many failures",
			slog.String("child", child.Id))
		supervisor.StopChildren(child)

		// Publish termination so the endpoint manager removes this entry
		if address, ok := state.childAddresses[child.Id]; ok {
			actorSystem.EventStream.Publish(&EndpointTerminatedEvent{Address: address})
			delete(state.childAddresses, child.Id)
		}
		return
	}

	supervisor.RestartChildren(child)
}

func (state *endpointSupervisor) spawnEndpointWriter(remote *Remote, address string, ctx actor.Context) *actor.PID {
	props := actor.
		PropsFromProducer(endpointWriterProducer(remote, address, remote.config),
			actor.WithMailbox(endpointWriterMailboxProducer(remote.config.EndpointWriterBatchSize, remote.config.EndpointWriterQueueSize)))
	pid := ctx.Spawn(props)
	return pid
}

func (state *endpointSupervisor) spawnEndpointWatcher(remote *Remote, address string, ctx actor.Context) *actor.PID {
	props := actor.
		PropsFromProducer(newEndpointWatcher(remote, address))
	pid := ctx.Spawn(props)
	return pid
}
