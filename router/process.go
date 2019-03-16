package router

import "github.com/AsynkronIT/protoactor-go/actor"

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

func (ref *process) SendSystemMessage(pid *actor.PID, message actor.SystemMessage) {
	r, _ := actor.ProcessRegistry.Get(ref.router)
	r.SendSystemMessage(pid, message)
}
