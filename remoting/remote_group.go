package remoting

import (
	"sync"

	"github.com/AsynkronIT/gam/actor"
)

// Act as goroutine safe proxy. Calling SetRoutes will block Route method
type RouterStateAsyncProxy struct {
	destination actor.RouterState
	rw          sync.RWMutex
	actor.RouterState
}

func NewRouterStateAsyncProxy(destination actor.RouterState) actor.RouterState {
	proxy := &RouterStateAsyncProxy{
		destination: destination,
	}
	return proxy
}

func (self *RouterStateAsyncProxy) Route(message interface{}) {
	self.rw.RLock()
	defer self.rw.RUnlock()
	self.destination.Route(message)
}

func (self *RouterStateAsyncProxy) SetRoutees(routees []*actor.PID) {
	self.rw.Lock()
	defer self.rw.Unlock()
	self.destination.SetRoutees(routees)
}

type RouterStateProducerFunc func(actor.GroupRouterConfig) actor.RouterState
type DestinationProducerFunc func(actor.Context, actor.RouterState)

type RemoteGroupRouter struct {
	routees             []*actor.PID
	service             string
	strategyProducer    RouterStateProducerFunc
	destinationProducer DestinationProducerFunc
	actor.GroupRouterConfig
}

func NewRemoteGroupRouter(service string) *RemoteGroupRouter {
	router := &RemoteGroupRouter{
		service: service,
	}
	return router
}

func (self *RemoteGroupRouter) GroupRouter() {}

func (self *RemoteGroupRouter) WithStrategyProducer(strategyProducer RouterStateProducerFunc) *RemoteGroupRouter {
	self.strategyProducer = strategyProducer
	return self
}

func (self *RemoteGroupRouter) WithDestinationProducer(destinationProducer DestinationProducerFunc) *RemoteGroupRouter {
	self.destinationProducer = destinationProducer
	return self
}

func (self *RemoteGroupRouter) Create() actor.RouterState {

	var strategy actor.RouterState

	if self.strategyProducer != nil {
		strategy = self.strategyProducer(self)
	}

	if strategy == nil {

	}

	return NewRouterStateAsyncProxy(strategy)
}

func (self *RemoteGroupRouter) OnStarted(context actor.Context, props actor.Props, router actor.RouterState) {
	self.destinationProducer(context, router)
}

func CreateDestinations(context actor.Context, service string, hosts []string) []*actor.PID {
	pids := make([]*actor.PID, len(hosts))

	for i, h := range hosts {
		pid := actor.NewPID(h, service)
		// Unfortunately this doesn't work since router hasn't been registred yet
		//context.Watch(pid)
		pids[i] = pid
	}
	return pids
}
