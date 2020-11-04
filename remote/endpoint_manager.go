package remote

import (
	"sync"
	"sync/atomic"

	"github.com/AsynkronIT/protoactor-go/mailbox"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/eventstream"
)

var endpointManager *endpointManagerValue

type endpointLazy struct {
	valueFunc func() *endpoint
	unloaded  uint32
}

type endpoint struct {
	writer  *actor.PID
	watcher *actor.PID
}

type endpointManagerValue struct {
	connections        *sync.Map
	remote             *Remote
	endpointSupervisor *actor.PID
	endpointSub        *eventstream.Subscription
}

func (r *Remote) startEndpointManager() {
	plog.Debug("Started EndpointManager")

	props := actor.PropsFromProducer(func() actor.Actor {
		return newEndpointSupervisor(r)
	}).
		WithGuardian(actor.RestartingSupervisorStrategy()).
		WithSupervisor(actor.RestartingSupervisorStrategy()).
		WithDispatcher(mailbox.NewSynchronizedDispatcher(300))
	endpointSupervisor, _ := r.actorSystem.Root.SpawnNamed(props, "EndpointSupervisor")

	endpointManager = &endpointManagerValue{
		connections:        &sync.Map{},
		remote:             r,
		endpointSupervisor: endpointSupervisor,
	}

	endpointManager.endpointSub = r.actorSystem.EventStream.
		Subscribe(endpointManager.endpointEvent).
		WithPredicate(func(m interface{}) bool {
			switch m.(type) {
			case *EndpointTerminatedEvent, *EndpointConnectedEvent:
				return true
			}
			return false
		})
}

func (r *Remote) stopEndpointManager() {
	r.actorSystem.EventStream.Unsubscribe(endpointManager.endpointSub)
	_ = r.actorSystem.Root.StopFuture(endpointManager.endpointSupervisor).Wait()
	endpointManager.endpointSub = nil
	endpointManager.connections = nil
	plog.Debug("Stopped EndpointManager")
}

func (em *endpointManagerValue) endpointEvent(evn interface{}) {
	switch msg := evn.(type) {
	case *EndpointTerminatedEvent:
		em.removeEndpoint(msg)
	case *EndpointConnectedEvent:
		endpoint := em.ensureConnected(msg.Address)
		em.remote.actorSystem.Root.Send(endpoint.watcher, msg)
	}
}

func (em *endpointManagerValue) remoteTerminate(msg *remoteTerminate) {
	address := msg.Watchee.Address
	endpoint := em.ensureConnected(address)
	em.remote.actorSystem.Root.Send(endpoint.watcher, msg)
}

func (em *endpointManagerValue) remoteWatch(msg *remoteWatch) {
	address := msg.Watchee.Address
	endpoint := em.ensureConnected(address)
	em.remote.actorSystem.Root.Send(endpoint.watcher, msg)
}

func (em *endpointManagerValue) remoteUnwatch(msg *remoteUnwatch) {
	address := msg.Watchee.Address
	endpoint := em.ensureConnected(address)
	em.remote.actorSystem.Root.Send(endpoint.watcher, msg)
}

func (em *endpointManagerValue) remoteDeliver(msg *remoteDeliver) {
	address := msg.target.Address
	endpoint := em.ensureConnected(address)
	em.remote.actorSystem.Root.Send(endpoint.writer, msg)
}

func (em *endpointManagerValue) ensureConnected(address string) *endpoint {
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

func (em *endpointManagerValue) removeEndpoint(msg *EndpointTerminatedEvent) {
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
