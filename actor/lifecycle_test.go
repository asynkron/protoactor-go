package actor

import (
	"testing"
)

type (
	dummyRequest  struct{}
	dummyResponse struct{}
)

func TestActorCanReplyOnStarting(t *testing.T) {
	future := NewFuture(system, testTimeout)
	a := rootContext.Spawn(PropsFromFunc(func(context Context) {
		switch context.Message().(type) {
		case *Started:
			context.Send(future.PID(), dummyResponse{})
		}
	}))
	_ = rootContext.StopFuture(a).Wait()
	assertFutureSuccess(future, t)
}

func TestActorCanReplyOnStopping(t *testing.T) {
	future := NewFuture(system, testTimeout)
	a := rootContext.Spawn(PropsFromFunc(func(context Context) {
		switch context.Message().(type) {
		case *Stopping:
			context.Send(future.PID(), dummyResponse{})
		}
	}))
	_ = rootContext.StopFuture(a).Wait()
	assertFutureSuccess(future, t)
}

func TestActorReceivesStartedMessage(t *testing.T) {
	future := NewFuture(system, testTimeout)
	_ = rootContext.Spawn(PropsFromFunc(func(context Context) {
		switch context.Message().(type) {
		case *Started:
			context.Send(future.PID(), dummyResponse{})
		}
	}))
	_ = future.Wait()
	assertFutureSuccess(future, t)
}

func TestActorReceivesRestartingMessage(t *testing.T) {
	future := NewFuture(system, testTimeout)
	a := rootContext.Spawn(PropsFromFunc(func(context Context) {
		switch context.Message().(type) {
		case *dummyRequest:
			panic("fail")
		case *Restarting:
			context.Send(future.PID(), dummyResponse{})
		}
	}))
	rootContext.Send(a, &dummyRequest{})
	assertFutureSuccess(future, t)
}

func TestActorReceivesStoppingMessage(t *testing.T) {
	future := NewFuture(system, testTimeout)
	a := rootContext.Spawn(PropsFromFunc(func(context Context) {
		switch context.Message().(type) {
		case *Stopping:
			context.Send(future.PID(), dummyResponse{})
		}
	}))
	_ = rootContext.StopFuture(a).Wait()
	assertFutureSuccess(future, t)
}
