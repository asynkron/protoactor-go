package actor

import (
	"testing"
	"time"
)

// deduplicator extracts a string key from a message. Messages producing the
// same key are considered duplicates.
type deduplicator func(interface{}) string

// deduplicationContext wraps an actor context with logic to ignore duplicate
// messages based on a deduplication function. A TTL controls how long entries
// are remembered before being purged.
func deduplicationContext(fn deduplicator, ttl time.Duration) ContextDecorator {
	return func(next ContextDecoratorFunc) ContextDecoratorFunc {
		return func(ctx Context) Context {
			return &dedupContext{
				Context: next(ctx),
				dedup:   fn,
				ttl:     ttl,
				seen:    make(map[string]time.Time),
			}
		}
	}
}

// dedupContext embeds an actor Context and tracks which messages have already
// been processed according to the deduplication key. If a duplicate is
// received, it is dropped before reaching the wrapped context.
type dedupContext struct {
	Context
	dedup deduplicator
	ttl   time.Duration
	seen  map[string]time.Time
}

var _ Context = (*dedupContext)(nil)

// Receive intercepts incoming messages and filters duplicates. Entries expire
// after the configured TTL, allowing messages with the same key to be processed
// again once the key is cleaned up.
func (d *dedupContext) Receive(envelope *MessageEnvelope) {
	now := time.Now()
	d.cleanup(now)

	key := d.dedup(envelope.Message)
	if key != "" {
		if last, exists := d.seen[key]; exists {
			if now.Sub(last) < d.ttl {
				d.seen[key] = now
				return
			}
		}
		d.seen[key] = now
	}
	d.Context.Receive(envelope)
}

func (d *dedupContext) cleanup(now time.Time) {
	for k, t := range d.seen {
		if now.Sub(t) >= d.ttl {
			delete(d.seen, k)
		}
	}
}

// collectingActor forwards string messages to the provided channel for
// verification in tests.
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
		WithContextDecorator(deduplicationContext(func(msg interface{}) string {
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
