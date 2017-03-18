package actor

func makeInboundMiddlewareChain(middleware []InboundMiddleware, lastReceiver ActorFunc) ActorFunc {
	if len(middleware) == 0 {
		return nil
	}

	h := middleware[len(middleware)-1](lastReceiver)
	for i := len(middleware) - 2; i >= 0; i-- {
		h = middleware[i](h)
	}
	return h
}

func makeOutboundMiddlewareChain(outboundMiddleware []OutboundMiddleware, lastSender SenderFunc) SenderFunc {
	if len(outboundMiddleware) == 0 {
		return nil
	}

	h := outboundMiddleware[len(outboundMiddleware)-1](lastSender)
	for i := len(outboundMiddleware) - 2; i >= 0; i-- {
		h = outboundMiddleware[i](h)
	}
	return h
}
