package actor

import (
	"testing"
)

func TestActorCanReplyOnStarting(t *testing.T) {
	future := NewFuture(testTimeout)
	a := rootContext.Spawn(PropsFromFunc(func(context Context) {
		switch context.Message().(type) {
		case *Started:
			context.Send(future.PID(), EchoResponse{})
		}
	}))
	rootContext.StopFuture(a).Wait()
	assertFutureSuccess(future, t)
}

func TestActorCanReplyOnStopping(t *testing.T) {
	future := NewFuture(testTimeout)
	a := rootContext.Spawn(PropsFromFunc(func(context Context) {
		switch context.Message().(type) {
		case *Stopping:
			context.Send(future.PID(), EchoResponse{})
		}
	}))
	rootContext.StopFuture(a).Wait()
	assertFutureSuccess(future, t)
}
