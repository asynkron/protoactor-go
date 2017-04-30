package actor

import (
	"fmt"
	"testing"
	"time"

	"github.com/AsynkronIT/protoactor-go/eventstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestLocalContext_SpawnNamed(t *testing.T) {
	pid, p := spawnMockProcess("foo/bar")
	defer removeMockProcess(pid)
	p.On("SendSystemMessage", matchPID(pid), mock.Anything)

	props := &Props{
		spawner: func(id string, _ *Props, _ *PID) (*PID, error) {
			assert.Equal(t, "foo/bar", id)
			return NewLocalPID(id), nil
		},
	}

	parent := &localContext{self: NewLocalPID("foo")}
	parent.SpawnNamed(props, "bar")
	p.AssertExpectations(t)
}

// TestLocalContext_Stop verifies if context is stopping and receives a Watch message, it should
// immediately respond with a Terminated message
func TestLocalContext_Stop(t *testing.T) {
	pid, p := spawnMockProcess("foo")
	defer removeMockProcess(pid)

	other, o := spawnMockProcess("watcher")
	defer removeMockProcess(other)

	o.On("SendSystemMessage", other, &Terminated{Who: pid})

	lc := newLocalContext(nullProducer, DefaultSupervisorStrategy(), nil, nil, nil)
	lc.self = pid
	lc.InvokeSystemMessage(&Stop{})
	lc.InvokeSystemMessage(&Watch{Watcher: other})

	p.AssertExpectations(t)
	o.AssertExpectations(t)
}

func TestLocalContext_SendMessage_WithOutboundMiddleware(t *testing.T) {
	// Define a local context with no-op outbound middlware
	mw := func(next SenderFunc) SenderFunc {
		return func(ctx Context, target *PID, envelope MessageEnvelope) {
			next(ctx, target, envelope)
		}
	}

	ctx := newLocalContext(nullProducer, DefaultSupervisorStrategy(), nil, []OutboundMiddleware{mw}, nil)

	// Define a receiver to which the local context will send a message
	var counter int
	receiver := Spawn(FromFunc(func(ctx Context) {
		switch ctx.Message().(type) {
		case bool:
			counter++
		}
	}))

	// Send a message with Tell
	// Then wait a little to allow the receiver to process the message
	// TODO: There should be a better way to wait.
	timeout := 3 * time.Millisecond
	ctx.Tell(receiver, true)
	time.Sleep(timeout)
	assert.Equal(t, 1, counter)

	// Send a message with Request
	counter = 0 // Reset the counter
	ctx.Request(receiver, true)
	time.Sleep(3 * time.Millisecond)
	assert.Equal(t, 1, counter)

	// Send a message with RequestFuture
	counter = 0 // Reset the counter
	ctx.RequestFuture(receiver, true, timeout).Wait()
	assert.Equal(t, 1, counter)
}

func BenchmarkLocalContext_ProcessMessageNoMiddleware(b *testing.B) {
	var m interface{} = 1

	ctx := &localContext{actor: nullReceive}
	ctx.SetBehavior(nullReceive.Receive)
	for i := 0; i < b.N; i++ {
		ctx.processMessage(m)
	}
}

func TestLocalContext_Respond(t *testing.T) {
	// Defined a responder actor
	// It simply echoes a received string.
	responder := Spawn(FromFunc(func(ctx Context) {
		switch m := ctx.Message().(type) {
		case string:
			ctx.Respond(fmt.Sprintf("Got a string: %s", m))
		}
	}))

	// Be prepared to catch a response that the responder will send to nil
	var gotResponseToNil bool
	deadLetterSubscriber = eventstream.Subscribe(func(msg interface{}) {
		if deadLetter, ok := msg.(*DeadLetterEvent); ok {
			if deadLetter.PID == nil && deadLetter.Sender == responder {
				gotResponseToNil = true
			}
		}
	})

	// Send a message to the responder using Request
	// The responder should send something back.
	timeout := 3 * time.Millisecond
	res, err := responder.RequestFuture("hello", timeout).Result()
	assert.Nil(t, err)
	assert.NotNil(t, res)

	resStr, ok := res.(string)
	assert.True(t, ok)
	assert.Equal(t, "Got a string: hello", resStr)

	// Ensure that the responder did not send anything to nil
	time.Sleep(timeout)
	assert.False(t, gotResponseToNil)

	// Send a message using Tell
	responder.Tell("hello")

	// Ensure that the responder actually send something to nil
	time.Sleep(timeout)
	assert.True(t, gotResponseToNil)

	// Cleanup
	eventstream.Unsubscribe(deadLetterSubscriber)
}

func BenchmarkLocalContext_ProcessMessageWithMiddleware(b *testing.B) {
	var m interface{} = 1

	fn := func(next ActorFunc) ActorFunc {
		return func(context Context) {
			next(context)
		}
	}

	ctx := newLocalContext(nullProducer, DefaultSupervisorStrategy(), []InboundMiddleware{fn, fn}, nil, nil)

	for i := 0; i < b.N; i++ {
		ctx.processMessage(m)
	}
}

func benchmarkLocalContext_SpawnWithMiddlewareN(n int, b *testing.B) {
	middlwareFn := func(next ActorFunc) ActorFunc {
		return func(context Context) {
			next(context)
		}
	}

	props := FromProducer(nullProducer)
	for i := 0; i < n; i++ {
		props = props.WithMiddleware(middlwareFn)
	}

	parent := &localContext{self: NewLocalPID("foo")}
	for i := 0; i < b.N; i++ {
		parent.Spawn(props)
	}
}

func BenchmarkLocalContext_SpawnWithMiddleware0(b *testing.B) {
	benchmarkLocalContext_SpawnWithMiddlewareN(0, b)
}

func BenchmarkLocalContext_SpawnWithMiddleware1(b *testing.B) {
	benchmarkLocalContext_SpawnWithMiddlewareN(1, b)
}

func BenchmarkLocalContext_SpawnWithMiddleware2(b *testing.B) {
	benchmarkLocalContext_SpawnWithMiddlewareN(2, b)
}

func BenchmarkLocalContext_SpawnWithMiddleware5(b *testing.B) {
	benchmarkLocalContext_SpawnWithMiddlewareN(5, b)
}

func TestActorContinueFutureInActor(t *testing.T) {
	pid := Spawn(FromFunc(func(ctx Context) {
		if ctx.Message() == "request" {
			ctx.Respond("done")
		}
		if ctx.Message() == "start" {
			f := ctx.RequestFuture(ctx.Self(), "request", 5*time.Second)
			ctx.AwaitFuture(f, func(res interface{}, err error) {
				ctx.Respond(res)
			})
		}
	}))
	res, err := pid.RequestFuture("start", time.Second).Result()
	assert.NoError(t, err)
	assert.Equal(t, "done", res)
}
