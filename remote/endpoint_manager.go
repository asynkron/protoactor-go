package remote

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/AsynkronIT/protoactor-go/log"
	"github.com/AsynkronIT/protoactor-go/mailbox"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/eventstream"
)

type endpointLazy struct {
	valueFunc func() *endpoint
	unloaded  uint32
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
	stopped                   bool
	endpointReaderConnections *sync.Map
}

func newEndpointManager(r *Remote) *endpointManager {
	return &endpointManager{
		connections:               &sync.Map{},
		remote:                    r,
		stopped:                   false,
		endpointReaderConnections: &sync.Map{},
	}
}

func (em *endpointManager) start() {
	eventStream := em.remote.actorSystem.EventStream
	em.endpointSub = eventStream.
		Subscribe(em.endpointEvent).
		WithPredicate(func(m interface{}) bool {
			switch m.(type) {
			case *EndpointTerminatedEvent, *EndpointConnectedEvent:
				return true
			}
			return false
		})
	em.startActivator()
	em.startSupervisor()

	if err := em.waiting(3 * time.Second); err != nil {
		panic(err)
	}
	plog.Info("Started EndpointManager")
}

func (em *endpointManager) waiting(timeout time.Duration) error {
	ctx := em.remote.actorSystem.Root
	if _, err := ctx.RequestFuture(em.activator, &Ping{}, timeout).Result(); err != nil {
		return err
	}
	return nil
}

func (em *endpointManager) stop() {
	em.stopped = true
	r := em.remote
	r.actorSystem.EventStream.Unsubscribe(em.endpointSub)
	if err := em.stopActivator(); err != nil {
		plog.Error("stop endpoint activator failed", log.Error(err))
	}
	if err := em.stopSupervisor(); err != nil {
		plog.Error("stop endpoint supervisor failed", log.Error(err))
	}
	em.endpointSub = nil
	em.connections = nil
	if em.endpointReaderConnections != nil {
		em.endpointReaderConnections.Range(func(key interface{}, value interface{}) bool {
			channel := value.(chan bool)
			channel <- true
			em.endpointReaderConnections.Delete(key)
			return true
		})
	}
	plog.Info("Stopped EndpointManager")
}

func (em *endpointManager) startActivator() {
	p := newActivatorActor(em.remote)
	props := actor.PropsFromProducer(p).WithGuardian(actor.RestartingSupervisorStrategy())
	pid, err := em.remote.actorSystem.Root.SpawnNamed(props, "activator")
	if err != nil {
		panic(err)
	}
	em.activator = pid
}

func (em *endpointManager) stopActivator() error {
	return em.remote.actorSystem.Root.StopFuture(em.activator).Wait()
}

func (em *endpointManager) startSupervisor() {
	r := em.remote
	props := actor.PropsFromProducer(func() actor.Actor {
		return newEndpointSupervisor(r)
	}).
		WithGuardian(actor.RestartingSupervisorStrategy()).
		WithSupervisor(actor.RestartingSupervisorStrategy()).
		WithDispatcher(mailbox.NewSynchronizedDispatcher(300))
	pid, err := r.actorSystem.Root.SpawnNamed(props, "EndpointSupervisor")
	if err != nil {
		panic(err)
	}
	em.endpointSupervisor = pid
}

func (em *endpointManager) stopSupervisor() error {
	r := em.remote
	return r.actorSystem.Root.StopFuture(em.endpointSupervisor).Wait()
}

func (em *endpointManager) endpointEvent(evn interface{}) {
	switch msg := evn.(type) {
	case *EndpointTerminatedEvent:
		em.removeEndpoint(msg)
	case *EndpointConnectedEvent:
		endpoint := em.ensureConnected(msg.Address)
		em.remote.actorSystem.Root.Send(endpoint.watcher, msg)
	}
}

func (em *endpointManager) remoteTerminate(msg *remoteTerminate) {
	if em.stopped {
		return
	}
	address := msg.Watchee.Address
	endpoint := em.ensureConnected(address)
	em.remote.actorSystem.Root.Send(endpoint.watcher, msg)
}

func (em *endpointManager) remoteWatch(msg *remoteWatch) {
	if em.stopped {
		return
	}
	address := msg.Watchee.Address
	endpoint := em.ensureConnected(address)
	em.remote.actorSystem.Root.Send(endpoint.watcher, msg)
}

func (em *endpointManager) remoteUnwatch(msg *remoteUnwatch) {
	if em.stopped {
		return
	}
	address := msg.Watchee.Address
	endpoint := em.ensureConnected(address)
	em.remote.actorSystem.Root.Send(endpoint.watcher, msg)
}

func (em *endpointManager) remoteDeliver(msg *remoteDeliver) {
	if em.stopped {
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
	em.remote.actorSystem.Root.Send(endpoint.writer, msg)
}

func (em *endpointManager) ensureConnected(address string) *endpoint {
	e, ok := em.connections.Load(address)
	if !ok {
		el := &endpointLazy{}
		var once sync.Once
		el.valueFunc = func() *endpoint {
			once.Do(func() {
				rst, _ := em.remote.actorSystem.Root.RequestFuture(em.endpointSupervisor, address, -1).Result()
				ep := rst.(*endpoint)
				el.valueFunc = func() *endpoint {
					return ep
				}
			})
			return el.valueFunc()
		}
		e, _ = em.connections.LoadOrStore(address, el)
	}

	el := e.(*endpointLazy)
	return el.valueFunc()
}

func (em *endpointManager) removeEndpoint(msg *EndpointTerminatedEvent) {
	v, ok := em.connections.Load(msg.Address)
	if ok {
		le := v.(*endpointLazy)
		if atomic.CompareAndSwapUint32(&le.unloaded, 0, 1) {
			em.connections.Delete(msg.Address)
			ep := le.valueFunc()
			em.remote.actorSystem.Root.Send(ep.watcher, msg)
			em.remote.actorSystem.Root.Send(ep.writer, msg)
		}
	}
}

type endpointSupervisor struct {
	remote *Remote
}

func newEndpointSupervisor(remote *Remote) actor.Actor {
	return &endpointSupervisor{
		remote: remote,
	}
}

func (state *endpointSupervisor) Receive(ctx actor.Context) {
	if address, ok := ctx.Message().(string); ok {
		e := &endpoint{
			writer:  state.spawnEndpointWriter(state.remote, address, ctx),
			watcher: state.spawnEndpointWatcher(state.remote, address, ctx),
		}
		ctx.Respond(e)
	}
}

func (state *endpointSupervisor) HandleFailure(actorSystem *actor.ActorSystem, supervisor actor.Supervisor, child *actor.PID, rs *actor.RestartStatistics, reason interface{}, message interface{}) {
	supervisor.RestartChildren(child)
}

func (state *endpointSupervisor) spawnEndpointWriter(remote *Remote, address string, ctx actor.Context) *actor.PID {
	props := actor.
		PropsFromProducer(endpointWriterProducer(remote, address, remote.config)).
		WithMailbox(endpointWriterMailboxProducer(remote.config.EndpointWriterBatchSize, remote.config.EndpointWriterQueueSize))
	pid := ctx.Spawn(props)
	return pid
}

func (state *endpointSupervisor) spawnEndpointWatcher(remote *Remote, address string, ctx actor.Context) *actor.PID {
	props := actor.
		PropsFromProducer(newEndpointWatcher(remote, address))
	pid := ctx.Spawn(props)
	return pid
}
