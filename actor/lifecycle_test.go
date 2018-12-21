package actor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestActorCanReplyOnStarting(t *testing.T) {
	future := NewFuture(testTimeout)
	a, err := EmptyRootContext.Spawn(PropsFromFunc(func(context Context) {
		switch context.Message().(type) {
		case *Started:
			context.Send(future.PID(), EchoResponse{})
		}
	}))
	assert.NoError(t, err)
	a.GracefulStop()
	assertFutureSuccess(future, t)
}

func TestActorCanReplyOnStopping(t *testing.T) {
	future := NewFuture(testTimeout)
	a, err := EmptyRootContext.Spawn(PropsFromFunc(func(context Context) {
		switch context.Message().(type) {
		case *Stopping:
			context.Send(future.PID(), EchoResponse{})
		}
	}))
	assert.NoError(t, err)
	a.GracefulStop()
	assertFutureSuccess(future, t)
}
