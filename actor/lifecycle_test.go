package actor

import (
	"testing"
)

func TestActorCanReplyOnStarting(t *testing.T) {
	future := NewFuture(testTimeout)
	a := Spawn(FromFunc(func(context Context) {
		switch context.Message().(type) {
		case *Started:
			future.PID().Tell(EchoResponse{})
		}
	}))
	defer a.StopFuture().Wait()
	assertFutureSuccess(future, t)
}

func TestActorCanReplyOnStopping(t *testing.T) {
	future := NewFuture(testTimeout)
	a := Spawn(FromFunc(func(context Context) {
		switch context.Message().(type) {
		case *Stopping:
			future.PID().Tell(EchoResponse{})
		}
	}))
	defer a.StopFuture().Wait()
	assertFutureSuccess(future, t)
}
