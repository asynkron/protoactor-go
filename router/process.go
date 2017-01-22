package router

import "github.com/AsynkronIT/protoactor-go/actor"

// process serves as a proxy to the router implementation and forwards messages directly to the routee. This
// optimization avoids serializing router messages through an actor
type process struct {
	router *actor.PID
	state  Interface
}

func (ref *process) SendUserMessage(pid *actor.PID, message interface{}, sender *actor.PID) {
	if _, ok := message.(ManagementMessage); ok {
		r, _ := actor.ProcessRegistry.Get(ref.router)
		r.SendUserMessage(pid, message, sender)
	} else {
		ref.state.RouteMessage(message, sender)
	}
}

func (ref *process) SendSystemMessage(pid *actor.PID, message interface{}) {
	r, _ := actor.ProcessRegistry.Get(ref.router)
	r.SendSystemMessage(pid, message)
}

func (ref *process) Stop(pid *actor.PID) {
	ref.SendSystemMessage(pid, &actor.Stop{})
}
