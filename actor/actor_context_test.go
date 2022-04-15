package actor

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func FuzzSpawnNamed(f *testing.F) {
	f.Add("parent", "child")

	f.Fuzz(func(t *testing.T, parentName string, childName string) {
		combined := parentName + "/" + childName

		pid, _ := spawnMockProcess(parentName)

		defer removeMockProcess(pid)

		props := &Props{
			spawner: func(actorSystem *ActorSystem, id string, _ *Props, _ SpawnerContext) (*PID, error) {
				assert.Equal(t, combined, id)

				return NewPID(actorSystem.Address(), id), nil
			},
		}

		parent := &actorContext{self: NewPID(localAddress, parentName), props: props, actorSystem: system}
		child, err := parent.SpawnNamed(props, childName)
		assert.NoError(t, err)
		assert.Equal(t, parent.Children()[0], child)
	})
}

// TestActorContext_Stop verifies if context is stopping and receives a Watch message, it should
// immediately respond with a Terminated message.
func TestActorContext_Stop(t *testing.T) {
	t.Parallel()

	pid, p := spawnMockProcess("foo")
	defer removeMockProcess(pid)

	other, o := spawnMockProcess("watcher")
	defer removeMockProcess(other)

	o.On("SendSystemMessage", other, &Terminated{Who: pid})

	props := PropsFromProducer(nullProducer, WithSupervisor(DefaultSupervisorStrategy()))
	lc := newActorContext(system, props, nil)
	lc.self = pid
	lc.InvokeSystemMessage(&Stop{})
	lc.InvokeSystemMessage(&Watch{Watcher: other})

	p.AssertExpectations(t)
	o.AssertExpectations(t)
}

func TestActorContext_SendMessage_WithSenderMiddleware(t *testing.T) {
	t.Parallel()

	var wg sync.WaitGroup

	wg.Add(1)

	// Define a local context with no-op sender middleware
	mw := func(next SenderFunc) SenderFunc {
		return func(ctx SenderContext, target *PID, envelope *MessageEnvelope) {
			next(ctx, target, envelope)
		}
	}

	props := PropsFromProducer(nullProducer, WithSupervisor(DefaultSupervisorStrategy()), WithSenderMiddleware(mw))
	ctx := newActorContext(system, props, nil)

	// Define a receiver to which the local context will send a message
	var counter int

	receiver := rootContext.Spawn(PropsFromFunc(func(ctx Context) {
		if _, ok := ctx.Message().(bool); ok {
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

	_ = ctx.RequestFuture(receiver, true, timeout).Wait()
	wg.Wait()
	assert.Equal(t, 1, counter)
}

func BenchmarkActorContext_ProcessMessageNoMiddleware(b *testing.B) {
	var m interface{} = 1

	ctx := newActorContext(system, PropsFromFunc(nullReceive), nil)
	for i := 0; i < b.N; i++ {
		ctx.processMessage(m)
	}
}

func TestActorContext_Respond(t *testing.T) {
	t.Parallel()

	var wg sync.WaitGroup

	wg.Add(1)

	// Defined a responder actor
	// It simply echoes a received string.
	responder := rootContext.Spawn(PropsFromFunc(func(ctx Context) {
		if m, ok := ctx.Message().(string); ok {
			ctx.Respond(fmt.Sprintf("Got a string: %v", m))
		}
	}))

	// Be prepared to catch a response that the responder will send to nil
	var gotResponseToNil bool

	deadLetterSubscriber := system.EventStream.Subscribe(func(msg interface{}) {
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
	system.EventStream.Unsubscribe(deadLetterSubscriber)
}

func TestActorContext_Forward(t *testing.T) {
	t.Parallel()
	// Defined a response actor
	// It simply responds to the string message
	responder := rootContext.Spawn(PropsFromFunc(func(ctx Context) {
		if m, ok := ctx.Message().(string); ok {
			ctx.Respond(fmt.Sprintf("Got a string: %v", m))
		}
	}))

	// Defined a forwarder actor
	// It simply forward the string message to responder
	forwarder := rootContext.Spawn(PropsFromFunc(func(ctx Context) {
		if _, ok := ctx.Message().(string); ok {
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

	props := PropsFromProducer(nullProducer, WithSupervisor(DefaultSupervisorStrategy()), WithReceiverMiddleware(fn))
	ctx := newActorContext(system, props, nil)

	for i := 0; i < b.N; i++ {
		ctx.processMessage(m)
	}
}

func benchmarkactorcontextSpawnwithmiddlewaren(n int, b *testing.B) {
	middlewareFn := func(next SenderFunc) SenderFunc {
		return func(ctx SenderContext, pid *PID, env *MessageEnvelope) {
			next(ctx, pid, env)
		}
	}

	props := PropsFromProducer(nullProducer)
	for i := 0; i < n; i++ {
		props = props.Configure(WithSenderMiddleware(middlewareFn))
	}

	system := NewActorSystem()
	parent := &actorContext{self: NewPID(localAddress, "foo"), props: props, actorSystem: system}

	for i := 0; i < b.N; i++ {
		parent.Spawn(props)
	}
}

func BenchmarkActorContext_SpawnWithMiddleware0(b *testing.B) {
	benchmarkactorcontextSpawnwithmiddlewaren(0, b)
}

func BenchmarkActorContext_SpawnWithMiddleware1(b *testing.B) {
	benchmarkactorcontextSpawnwithmiddlewaren(1, b)
}

func BenchmarkActorContext_SpawnWithMiddleware2(b *testing.B) {
	benchmarkactorcontextSpawnwithmiddlewaren(2, b)
}

func BenchmarkActorContext_SpawnWithMiddleware5(b *testing.B) {
	benchmarkactorcontextSpawnwithmiddlewaren(5, b)
}

func TestActorContinueFutureInActor(t *testing.T) {
	t.Parallel()

	pid := rootContext.Spawn(PropsFromFunc(func(ctx Context) {
		if ctx.Message() == "request" {
			ctx.Respond("done")
		}
		if ctx.Message() == "start" {
			f := ctx.RequestFuture(ctx.Self(), "request", 5*time.Second)
			ctx.ReenterAfter(f, func(res interface{}, err error) {
				ctx.Respond(res)
			})
		}
	}))
	res, err := rootContext.RequestFuture(pid, "start", time.Second).Result()
	assert.NoError(t, err)
	assert.Equal(t, "done", res)
}

type dummyAutoRespond struct{}

func (*dummyAutoRespond) GetAutoResponse(_ Context) interface{} {
	return &dummyResponse{}
}

func TestActorContextAutoRespondMessage(t *testing.T) {
	t.Parallel()

	pid := rootContext.Spawn(PropsFromFunc(func(ctx Context) {}))

	var msg AutoRespond = &dummyAutoRespond{}

	res, err := rootContext.RequestFuture(pid, msg, 1*time.Second).Result()
	assert.NoError(t, err)
	assert.IsType(t, &dummyResponse{}, res)
}

func TestActorContextAutoRespondTouchedMessage(t *testing.T) {
	t.Parallel()

	pid := rootContext.Spawn(PropsFromFunc(func(ctx Context) {}))

	var msg AutoRespond = &Touch{}

	res, err := rootContext.RequestFuture(pid, msg, 1*time.Second).Result()

	res2, _ := res.(*Touched)

	assert.NoError(t, err)
	assert.IsType(t, &Touched{}, res)
	assert.True(t, res2.Who.Equal(pid))
}
