package routing

import "github.com/AsynkronIT/protoactor-go/actor"

type routerProcess struct {
	router *actor.PID
	state  RouterState
}

func (ref *routerProcess) SendUserMessage(pid *actor.PID, message interface{}, sender *actor.PID) {
	if _, ok := message.(ManagementMessage); ok {
		r, _ := actor.ProcessRegistry.Get(ref.router)
		r.SendUserMessage(pid, message, sender)
	} else {
		ref.state.RouteMessage(message, sender)
	}
}

func (ref *routerProcess) Watch(pid *actor.PID) {
	ref.SendSystemMessage(pid, &actor.Watch{Watcher: pid})
}

func (ref *routerProcess) Unwatch(pid *actor.PID) {
	ref.SendSystemMessage(pid, &actor.Unwatch{Watcher: pid})
}

func (ref *routerProcess) SendSystemMessage(pid *actor.PID, message actor.SystemMessage) {
	r, _ := actor.ProcessRegistry.Get(ref.router)
	r.SendSystemMessage(pid, message)
}

func (ref *routerProcess) Stop(pid *actor.PID) {
	ref.SendSystemMessage(pid, &actor.Stop{})
}

type RouterState interface {
	RouteMessage(message interface{}, sender *actor.PID)
	SetRoutees(routees *actor.PIDSet)
	GetRoutees() *actor.PIDSet
}
