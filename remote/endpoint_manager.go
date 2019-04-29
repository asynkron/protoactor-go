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
	config             *remoteConfig
	endpointSupervisor *actor.PID
	endpointSub        *eventstream.Subscription
}

func startEndpointManager(config *remoteConfig) {
	plog.Debug("Started EndpointManager")

	props := actor.PropsFromProducer(newEndpointSupervisor).
		WithGuardian(actor.RestartingSupervisorStrategy()).
		WithSupervisor(actor.RestartingSupervisorStrategy()).
		WithDispatcher(mailbox.NewSynchronizedDispatcher(300))
	endpointSupervisor, _ := rootContext.SpawnNamed(props, "EndpointSupervisor")

	endpointManager = &endpointManagerValue{
		connections:        &sync.Map{},
		config:             config,
		endpointSupervisor: endpointSupervisor,
	}

	endpointManager.endpointSub = eventstream.
		Subscribe(endpointManager.endpointEvent).
		WithPredicate(func(m interface{}) bool {
			switch m.(type) {
			case *EndpointTerminatedEvent, *EndpointConnectedEvent:
				return true
			}
			return false
		})
}

func stopEndpointManager() {
	eventstream.Unsubscribe(endpointManager.endpointSub)
	rootContext.StopFuture(endpointManager.endpointSupervisor).Wait()
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
		rootContext.Send(endpoint.watcher, msg)
	}
}

func (em *endpointManagerValue) remoteTerminate(msg *remoteTerminate) {
	address := msg.Watchee.Address
	endpoint := em.ensureConnected(address)
	rootContext.Send(endpoint.watcher, msg)
}

func (em *endpointManagerValue) remoteWatch(msg *remoteWatch) {
	address := msg.Watchee.Address
	endpoint := em.ensureConnected(address)
	rootContext.Send(endpoint.watcher, msg)
}

func (em *endpointManagerValue) remoteUnwatch(msg *remoteUnwatch) {
	address := msg.Watchee.Address
	endpoint := em.ensureConnected(address)
	rootContext.Send(endpoint.watcher, msg)
}

func (em *endpointManagerValue) remoteDeliver(msg *remoteDeliver) {
	address := msg.target.Address
	endpoint := em.ensureConnected(address)
	rootContext.Send(endpoint.writer, msg)
}

func (em *endpointManagerValue) ensureConnected(address string) *endpoint {
	e, ok := em.connections.Load(address)
	if !ok {
		el := &endpointLazy{}
		var once sync.Once
		el.valueFunc = func() *endpoint {
			once.Do(func() {
				rst, _ := rootContext.RequestFuture(em.endpointSupervisor, address, -1).Result()
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
			rootContext.Send(ep.watcher, msg)
			rootContext.Send(ep.writer, msg)
		}
	}
}

type endpointSupervisor struct{}

func newEndpointSupervisor() actor.Actor {
	return &endpointSupervisor{}
}

func (state *endpointSupervisor) Receive(ctx actor.Context) {
	if address, ok := ctx.Message().(string); ok {
		e := &endpoint{
			writer:  state.spawnEndpointWriter(address, ctx),
			watcher: state.spawnEndpointWatcher(address, ctx),
		}
		ctx.Respond(e)
	}
}

func (state *endpointSupervisor) HandleFailure(supervisor actor.Supervisor, child *actor.PID, rs *actor.RestartStatistics, reason interface{}, message interface{}) {
	supervisor.RestartChildren(child)
}

func (state *endpointSupervisor) spawnEndpointWriter(address string, ctx actor.Context) *actor.PID {
	props := actor.
		PropsFromProducer(endpointWriterProducer(address, endpointManager.config)).
		WithMailbox(endpointWriterMailboxProducer(endpointManager.config.endpointWriterBatchSize, endpointManager.config.endpointWriterQueueSize))
	pid := ctx.Spawn(props)
	return pid
}

func (state *endpointSupervisor) spawnEndpointWatcher(address string, ctx actor.Context) *actor.PID {
	props := actor.
		PropsFromProducer(newEndpointWatcher(address))
	pid := ctx.Spawn(props)
	return pid
}
