package actor

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/AsynkronIT/protoactor-go/eventstream"
	"github.com/stretchr/testify/assert"
)

func TestActorContext_SpawnNamed(t *testing.T) {
	pid, _ := spawnMockProcess("foo/bar")

	defer removeMockProcess(pid)

	props := &Props{
		spawner: func(id string, _ *Props, _ SpawnerContext) (*PID, error) {
			assert.Equal(t, "foo/bar", id)
			return NewLocalPID(id), nil
		},
	}

	parent := &actorContext{self: NewLocalPID("foo"), props: props}
	child, err := parent.SpawnNamed(props, "bar")
	assert.NoError(t, err)
	assert.Equal(t, parent.Children()[0], child)
}

// TestActorContext_Stop verifies if context is stopping and receives a Watch message, it should
// immediately respond with a Terminated message
func TestActorContext_Stop(t *testing.T) {
	pid, p := spawnMockProcess("foo")
	defer removeMockProcess(pid)

	other, o := spawnMockProcess("watcher")
	defer removeMockProcess(other)

	o.On("SendSystemMessage", other, &Terminated{Who: pid})

	props := PropsFromProducer(nullProducer).WithSupervisor(DefaultSupervisorStrategy())
	lc := newActorContext(props, nil)
	lc.self = pid
	lc.InvokeSystemMessage(&Stop{})
	lc.InvokeSystemMessage(&Watch{Watcher: other})

	p.AssertExpectations(t)
	o.AssertExpectations(t)
}

func TestActorContext_SendMessage_WithSenderMiddleware(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	// Define a local context with no-op sender middleware
	mw := func(next SenderFunc) SenderFunc {
		return func(ctx SenderContext, target *PID, envelope *MessageEnvelope) {
			next(ctx, target, envelope)
		}
	}

	props := PropsFromProducer(nullProducer).WithSupervisor(DefaultSupervisorStrategy()).WithSenderMiddleware(mw)
	ctx := newActorContext(props, nil)

	// Define a receiver to which the local context will send a message
	var counter int
	receiver := rootContext.Spawn(PropsFromFunc(func(ctx Context) {
		switch ctx.Message().(type) {
		case bool:
			counter++
			wg.Done()
		}
	}))

	// Send a message with Tell
	// Then wait a little to allow the receiver to process the message
	// TODO: There should be a better way to wait.
	timeout := 3 * time.Millisecond
	ctx.Send(receiver, true)
	wg.Wait()
	assert.Equal(t, 1, counter)

	// Send a message with Request
	counter = 0 // Reset the counter
	wg.Add(1)
	ctx.Request(receiver, true)
	wg.Wait()
	assert.Equal(t, 1, counter)

	// Send a message with RequestFuture
	counter = 0 // Reset the counter
	wg.Add(1)
	ctx.RequestFuture(receiver, true, timeout).Wait()
	wg.Wait()
	assert.Equal(t, 1, counter)
}

func BenchmarkActorContext_ProcessMessageNoMiddleware(b *testing.B) {
	var m interface{} = 1

	ctx := newActorContext(PropsFromFunc(nullReceive), nil)
	for i := 0; i < b.N; i++ {
		ctx.processMessage(m)
	}
}

func TestActorContext_Respond(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	// Defined a responder actor
	// It simply echoes a received string.
	responder := rootContext.Spawn(PropsFromFunc(func(ctx Context) {
		switch m := ctx.Message().(type) {
		case string:
			ctx.Respond(fmt.Sprintf("Got a string: %s", m))
		}
	}))

	// Be prepared to catch a response that the responder will send to nil
	var gotResponseToNil bool
	deadLetterSubscriber = eventstream.Subscribe(func(msg interface{}) {
		if deadLetter, ok := msg.(*DeadLetterEvent); ok {
			if deadLetter.PID == nil {
				gotResponseToNil = true
				wg.Done()
			}
		}
	})

	// Send a message to the responder using Request
	// The responder should send something back.
	timeout := 3 * time.Millisecond
	res, err := rootContext.RequestFuture(responder, "hello", timeout).Result()
	assert.Nil(t, err)
	assert.NotNil(t, res)

	resStr, ok := res.(string)
	assert.True(t, ok)
	assert.Equal(t, "Got a string: hello", resStr)

	// Ensure that the responder did not send anything to nil
	assert.False(t, gotResponseToNil)

	// Send a message using Tell
	rootContext.Send(responder, "hello")

	// Ensure that the responder actually send something to nil
	wg.Wait()
	assert.True(t, gotResponseToNil)

	// Cleanup
	eventstream.Unsubscribe(deadLetterSubscriber)
}

func TestActorContext_Forward(t *testing.T) {
	// Defined a respond actor
	// It simply respond the string message
	responder := rootContext.Spawn(PropsFromFunc(func(ctx Context) {
		switch m := ctx.Message().(type) {
		case string:
			ctx.Respond(fmt.Sprintf("Got a string: %s", m))
		}
	}))

	// Defined a forwarder actor
	// It simply forward the string message to responder
	forwarder := rootContext.Spawn(PropsFromFunc(func(ctx Context) {
		switch ctx.Message().(type) {
		case string:
			ctx.Forward(responder)
		}
	}))

	// Send a message to the responder using Request
	// The responder should send something back.
	timeout := 3 * time.Millisecond
	res, err := rootContext.RequestFuture(forwarder, "hello", timeout).Result()
	assert.Nil(t, err)
	assert.NotNil(t, res)

	resStr, ok := res.(string)
	assert.True(t, ok)
	assert.Equal(t, "Got a string: hello", resStr)
}

func BenchmarkActorContext_ProcessMessageWithMiddleware(b *testing.B) {
	var m interface{} = 1

	fn := func(next ReceiverFunc) ReceiverFunc {
		return func(ctx ReceiverContext, env *MessageEnvelope) {
			next(ctx, env)
		}
	}

	props := PropsFromProducer(nullProducer).WithSupervisor(DefaultSupervisorStrategy()).WithReceiverMiddleware(fn)
	ctx := newActorContext(props, nil)

	for i := 0; i < b.N; i++ {
		ctx.processMessage(m)
	}
}

func benchmarkActorContext_SpawnWithMiddlewareN(n int, b *testing.B) {
	middlewareFn := func(next SenderFunc) SenderFunc {
		return func(ctx SenderContext, pid *PID, env *MessageEnvelope) {
			next(ctx, pid, env)
		}
	}

	props := PropsFromProducer(nullProducer)
	for i := 0; i < n; i++ {
		props = props.WithSenderMiddleware(middlewareFn)
	}

	parent := &actorContext{self: NewLocalPID("foo"), props: props}
	for i := 0; i < b.N; i++ {
		parent.Spawn(props)
	}
}

func BenchmarkActorContext_SpawnWithMiddleware0(b *testing.B) {
	benchmarkActorContext_SpawnWithMiddlewareN(0, b)
}

func BenchmarkActorContext_SpawnWithMiddleware1(b *testing.B) {
	benchmarkActorContext_SpawnWithMiddlewareN(1, b)
}

func BenchmarkActorContext_SpawnWithMiddleware2(b *testing.B) {
	benchmarkActorContext_SpawnWithMiddlewareN(2, b)
}

func BenchmarkActorContext_SpawnWithMiddleware5(b *testing.B) {
	benchmarkActorContext_SpawnWithMiddlewareN(5, b)
}

func TestActorContinueFutureInActor(t *testing.T) {
	pid := rootContext.Spawn(PropsFromFunc(func(ctx Context) {
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
	res, err := rootContext.RequestFuture(pid, "start", time.Second).Result()
	assert.NoError(t, err)
	assert.Equal(t, "done", res)
}
