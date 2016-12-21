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
		r = append(r, m.PID)
		a.state.SetRoutees(r)

	case *RouterRemoveRoutee:
		r := a.state.GetRoutees()
		for i, pid := range r {
			if pid.Equal(m.PID) {
				l := len(r) - 1
				r[i] = r[l]
				r[l] = nil
				r = r[:l]
				break
			}
		}
		if len(r) == 0 {
			r = nil
		}
		a.state.SetRoutees(r)

	case *RouterBroadcastMessage:
		msg := m.Message
		sender := context.Sender()
		r := a.state.GetRoutees()
		for _, pid := range r {
			pid.Request(msg, sender)
		}

	case *RouterGetRoutees:
		context.Sender().Tell(&RouterRoutees{a.state.GetRoutees()})
	}
}
