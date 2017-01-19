package router

import (
	"sync"
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"
)

type routerActor struct {
	props  actor.Props
	config RouterConfig
	state  Interface
	wg     sync.WaitGroup
}

func (a *routerActor) Receive(context actor.Context) {
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
		// The removed node should be stopped with a delay to give it a
		// chance to process the messages in its mailbox (best effort).
		// There is no way to send a message with timer atm and blocking
		// the router actor is not a good idea.
		// TODO: Update this when there is such a way
		go func(pid *actor.PID) {
			timer := time.NewTimer(time.Millisecond * 100)
			<-timer.C
			m.PID.Stop()
		}(m.PID)

	case *BroadcastMessage:
		msg := m.Message
		sender := context.Sender()
		a.state.GetRoutees().ForEach(func(i int, pid actor.PID) {
			pid.Request(msg, sender)
		})

	case *GetRoutees:
		r := a.state.GetRoutees()
		routees := make([]*actor.PID, r.Len())
		r.ForEach(func(i int, pid actor.PID) {
			routees[i] = &pid
		})

		context.Sender().Tell(&Routees{routees})
	}
}
