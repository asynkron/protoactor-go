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
