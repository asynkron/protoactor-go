package actor

type emptyActor struct {
	receive Receive
}

func (state *emptyActor) Receive(context Context) {
	switch context.Message().(type) {
	case Started:
		context.Become(state.receive)
		state.receive(context) //forward start message to new behavior
	}
}
