package actor

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestActorHandlesConcurrentMessageFlood verifies that an actor processes
// all messages sent concurrently without loss or duplication.
func TestActorHandlesConcurrentMessageFlood(t *testing.T) {
	system := NewActorSystem()

	var processed int32
	done := make(chan struct{})

	// simple actor increments counter for each int message
	props := PropsFromFunc(func(ctx Context) {
		if _, ok := ctx.Message().(int); ok {
			if atomic.AddInt32(&processed, 1) == 1000 {
				close(done)
			}
		}
	})

	pid := system.Root.Spawn(props)

	total := 1000
	workers := 10
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < total/workers; j++ {
				system.Root.Send(pid, j)
			}
		}()
	}
	wg.Wait()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout waiting for messages, processed %d", atomic.LoadInt32(&processed))
	}

	_ = system.Root.StopFuture(pid).Wait()
}

// TestActorDeadLettersAfterStop verifies that messages sent to a stopped
// actor are redirected to the dead letter mailbox.
func TestActorDeadLettersAfterStop(t *testing.T) {
	system := NewActorSystem()

	pid := system.Root.Spawn(PropsFromFunc(func(Context) {}))

	var deadletters int32
	sub := system.EventStream.Subscribe(func(msg interface{}) {
		if _, ok := msg.(*DeadLetterEvent); ok {
			atomic.AddInt32(&deadletters, 1)
		}
	})

	// stop the actor before sending any messages
	_ = system.Root.StopFuture(pid).Wait()

	const attempts = 10
	for i := 0; i < attempts; i++ {
		system.Root.Send(pid, i)
	}

	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&deadletters) != attempts {
		t.Fatalf("expected %d dead letters, got %d", attempts, atomic.LoadInt32(&deadletters))
	}

	system.EventStream.Unsubscribe(sub)
}

type forwardingParent struct {
	childProps *Props
	child      *PID
}

func (p *forwardingParent) Receive(ctx Context) {
	switch ctx.Message().(type) {
	case *Started:
		p.child = ctx.Spawn(p.childProps)
	case int, string:
		ctx.Send(p.child, ctx.Message())
	}
}

// TestSupervisorHandlesChildPanicUnderLoad verifies that a parent actor with a
// supervision strategy restarts a failing child under concurrent message load
// without losing messages or entering an inconsistent state.
func TestSupervisorHandlesChildPanicUnderLoad(t *testing.T) {
	system := NewActorSystem()

	const total = 100
	var processed int32
	done := make(chan struct{})

	childProps := PropsFromFunc(func(ctx Context) {
		switch ctx.Message().(type) {
		case int:
			if atomic.AddInt32(&processed, 1) == total {
				close(done)
			}
		case string:
			panic("boom")
		}
	})

	var restarts int32
	decider := func(reason interface{}) Directive {
		atomic.AddInt32(&restarts, 1)
		return RestartDirective
	}
	supervisor := NewOneForOneStrategy(10, time.Second, decider)

	parentProps := PropsFromProducer(func() Actor {
		return &forwardingParent{childProps: childProps}
	}, WithSupervisor(supervisor))

	pid := system.Root.Spawn(parentProps)

	workers := 10
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < total/workers; j++ {
				system.Root.Send(pid, j)
			}
		}()
	}

	// inject a failure while other messages are in flight
	system.Root.Send(pid, "panic")

	wg.Wait()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout waiting for messages, processed %d", atomic.LoadInt32(&processed))
	}

	if atomic.LoadInt32(&restarts) != 1 {
		t.Fatalf("expected 1 restart, got %d", atomic.LoadInt32(&restarts))
	}

	_ = system.Root.StopFuture(pid).Wait()
}
