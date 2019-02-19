package actor

func makeReceiverMiddlewareChain(receiverMiddleware []ReceiverMiddleware, lastReceiver ReceiverFunc) ReceiverFunc {
	if len(receiverMiddleware) == 0 {
		return nil
	}

	h := receiverMiddleware[len(receiverMiddleware)-1](lastReceiver)
	for i := len(receiverMiddleware) - 2; i >= 0; i-- {
		h = receiverMiddleware[i](h)
	}
	return h
}

func makeSenderMiddlewareChain(senderMiddleware []SenderMiddleware, lastSender SenderFunc) SenderFunc {
	if len(senderMiddleware) == 0 {
		return nil
	}

	h := senderMiddleware[len(senderMiddleware)-1](lastSender)
	for i := len(senderMiddleware) - 2; i >= 0; i-- {
		h = senderMiddleware[i](h)
	}
	return h
}

func makeContextDecoratorChain(decorator []ContextDecorator, lastDecorator ContextDecoratorFunc) ContextDecoratorFunc {
	if len(decorator) == 0 {
		return nil
	}

	h := decorator[len(decorator)-1](lastDecorator)
	for i := len(decorator) - 2; i >= 0; i-- {
		h = decorator[i](h)
	}
	return h
}

func makeSpawnMiddlewareChain(spawnMiddleware []SpawnMiddleware, lastSpawn SpawnFunc) SpawnFunc {
	if len(spawnMiddleware) == 0 {
		return nil
	}

	h := spawnMiddleware[len(spawnMiddleware)-1](lastSpawn)
	for i := len(spawnMiddleware) - 2; i >= 0; i-- {
		h = spawnMiddleware[i](h)
	}
	return h
}
