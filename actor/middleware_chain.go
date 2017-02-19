package actor

func makeMiddlewareChain(middleware []func(ActorFunc) ActorFunc, actorReceiver ActorFunc) ActorFunc {
	if len(middleware) == 0 {
		return nil
	}

	h := middleware[len(middleware)-1](actorReceiver)
	for i := len(middleware) - 2; i >= 0; i-- {
		h = middleware[i](h)
	}
	return h
}

func makeMiddleware2Chain(middleware2 []func(SenderFunc) SenderFunc, actorReceiver SenderFunc) SenderFunc {
	if len(middleware2) == 0 {
		return nil
	}

	h := middleware2[len(middleware2)-1](actorReceiver)
	for i := len(middleware2) - 2; i >= 0; i-- {
		h = middleware2[i](h)
	}
	return h
}
