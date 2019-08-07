package router

import (
	"sync"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
)

type poolRouterActor struct {
	props  *actor.Props
	config RouterConfig
	state  RouterState
	wg     *sync.WaitGroup
}

func (a *poolRouterActor) Receive(context actor.Context) {
	switch m := context.Message().(type) {
	case *actor.Started:
		a.config.OnStarted(context, a.props, a.state)
		a.wg.Done()

	case *AddRoutee:
		r := a.state.GetRoutees()
		if r.Contains(m.PID) {
			return
		}
		context.Watch(m.PID)
		r.Add(m.PID)
		a.state.SetRoutees(r)

	case *RemoveRoutee:
		r := a.state.GetRoutees()
		if !r.Contains(m.PID) {
			return
		}

		context.Unwatch(m.PID)
		r.Remove(m.PID)
		a.state.SetRoutees(r)
		// sleep for 1ms before sending the poison pill
		// This is to give some time to the routee actor receive all
		// the messages. Specially due to the synchronization conditions in
		// consistent hash router, where a copy of hmc can be obtained before
		// the update and cause messages routed to a dead routee if there is no
		// delay. This is a best effort approach and 1ms seems to be acceptable
		// in terms of both delay it cause to the router actor and the time it
		// provides for the routee to receive messages before it dies.
		time.Sleep(time.Millisecond * 1)
		context.Send(m.PID, &actor.PoisonPill{})

	case *BroadcastMessage:
		msg := m.Message
		sender := context.Sender()
		a.state.GetRoutees().ForEach(func(i int, pid actor.PID) {
			context.RequestWithCustomSender(&pid, msg, sender)
		})

	case *GetRoutees:
		r := a.state.GetRoutees()
		routees := make([]*actor.PID, r.Len())
		r.ForEach(func(i int, pid actor.PID) {
			routees[i] = &pid
		})

		context.Respond(&Routees{routees})
	case *actor.Terminated:
		r := a.state.GetRoutees()
		if r.Remove(m.Who) {
			a.state.SetRoutees(r)
		}
	}
}
