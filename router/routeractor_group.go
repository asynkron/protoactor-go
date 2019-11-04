package router

import (
	"sync"

	"github.com/otherview/protoactor-go/actor"
)

type groupRouterActor struct {
	props  *actor.Props
	config RouterConfig
	state  RouterState
	wg     *sync.WaitGroup
}

func (a *groupRouterActor) Receive(context actor.Context) {
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
	}
}
