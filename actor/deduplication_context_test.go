package actor

import (
	"testing"
	"time"
)

// collectingActor forwards string messages to a channel for verification.
type collectingActor struct {
	ch chan string
}

func (a *collectingActor) Receive(ctx Context) {
	if msg, ok := ctx.Message().(string); ok {
		a.ch <- msg
	}
}

func TestDeduplicationContext(t *testing.T) {
	system := NewActorSystem()
	ttl := 50 * time.Millisecond
	ch := make(chan string, 6)
	props := PropsFromProducer(func() Actor { return &collectingActor{ch: ch} },
		WithContextDecorator(DeduplicationContext(func(msg interface{}) string {
			if s, ok := msg.(string); ok {
				return s
			}
			return ""
		}, ttl)))

	pid := system.Root.Spawn(props)
	system.Root.Send(pid, "one")
	system.Root.Send(pid, "two")
	system.Root.Send(pid, "one") // duplicate
	system.Root.Send(pid, "three")
	system.Root.Send(pid, "two") // duplicate
	time.Sleep(ttl + 10*time.Millisecond)
	system.Root.Send(pid, "one") // after ttl, should be processed again

	expected := []string{"one", "two", "three", "one"}
	var received []string
	for i := 0; i < len(expected); i++ {
		select {
		case msg := <-ch:
			received = append(received, msg)
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for message %d", i)
		}
	}

	// ensure no further messages are received
	select {
	case msg := <-ch:
		t.Fatalf("unexpected message: %s", msg)
	case <-time.After(50 * time.Millisecond):
	}

	system.Root.Stop(pid)

	if len(received) != len(expected) {
		t.Fatalf("expected %d messages, got %d: %v", len(expected), len(received), received)
	}
	for i, msg := range expected {
		if received[i] != msg {
			t.Fatalf("expected message %d to be %q, got %q", i, msg, received[i])
		}
	}
}
