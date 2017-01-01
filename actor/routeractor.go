package actor

type routerActor struct {
	props  Props
	config RouterConfig
	state  RouterState
}

func (a *routerActor) Receive(context Context) {
	switch m := context.Message().(type) {
	case *Started:
		a.config.OnStarted(context, a.props, a.state)

	case *RouterAddRoutee:
		r := a.state.GetRoutees()
		if r.Contains(m.PID) {
			return
		}
		context.Watch(m.PID)
		r.Add(m.PID)
		a.state.SetRoutees(r)

	case *RouterRemoveRoutee:
		r := a.state.GetRoutees()
		if !r.Contains(m.PID) {
			return
		}

		context.Unwatch(m.PID)
		r.Remove(m.PID)
		a.state.SetRoutees(r)

	case *RouterBroadcastMessage:
		msg := m.Message
		sender := context.Sender()
		a.state.GetRoutees().ForEach(func(i int, pid PID) {
			pid.Request(msg, sender)
		})

	case *RouterGetRoutees:
		r := a.state.GetRoutees()
		routees := make([]*PID, r.Len())
		r.ForEach(func(i int, pid PID) {
			routees[i] = &pid
		})

		context.Sender().Tell(&RouterRoutees{routees})
	}
}
