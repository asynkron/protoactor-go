package actor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestLocalContext_SpawnNamed(t *testing.T) {
	pid, p := spawnMockProcess("foo/bar")
	defer removeMockProcess(pid)
	p.On("SendSystemMessage", matchPID(pid), mock.Anything)

	props := Props{
		spawner: func(id string, _ Props, _ *PID) *PID {
			assert.Equal(t, "foo/bar", id)
			return NewLocalPID(id)
		},
	}

	parent := &localContext{self: NewLocalPID("foo")}
	parent.SpawnNamed(props, "bar")
	mock.AssertExpectationsForObjects(t, p)
}

func BenchmarkLocalContext_ProcessMessageNoMiddleware(b *testing.B) {
	var m interface{} = 1

	ctx := &localContext{actor: nullReceive}
	ctx.SetBehavior(nullReceive.Receive)
	for i := 0; i < b.N; i++ {
		ctx.processMessage(m)
	}
}

func BenchmarkLocalContext_ProcessMessageWithMiddleware(b *testing.B) {
	var m interface{} = 1

	fn := func(next ReceiveFunc) ReceiveFunc {
		fn := func(context Context) {
			next(context)
		}
		return fn
	}

	ctx := &localContext{actor: nullReceive, middleware: makeMiddlewareChain([]func(ReceiveFunc) ReceiveFunc{fn, fn}, localContextReceiver)}
	ctx.SetBehavior(nullReceive.Receive)
	for i := 0; i < b.N; i++ {
		ctx.processMessage(m)
	}
}
