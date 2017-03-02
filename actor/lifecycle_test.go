package actor

import (
	"testing"
)

func TestActorCanReplyOnStarting(t *testing.T) {
	future := NewFuture(testTimeout)
	a := Spawn(FromFunc(func(context Context) {
		switch context.Message().(type) {
		case *Started:
			context.Tell(future.PID(), EchoResponse{})
		}
	}))
	a.GracefulStop()
	assertFutureSuccess(future, t)
}

func TestActorCanReplyOnStopping(t *testing.T) {
	future := NewFuture(testTimeout)
	a := Spawn(FromFunc(func(context Context) {
		switch context.Message().(type) {
		case *Stopping:
			context.Tell(future.PID(), EchoResponse{})
		}
	}))
	a.GracefulStop()
	assertFutureSuccess(future, t)
}
