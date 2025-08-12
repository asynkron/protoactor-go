package actor

import (
	"log/slog"
	"time"
)

// Deduplicator extracts a deduplication key from a message.
// Messages yielding the same key within the TTL are treated as duplicates.
type Deduplicator func(interface{}) string

// DeduplicationContext returns a context decorator that ignores duplicate messages
// as determined by the provided Deduplicator function. Keys expire after the
// given TTL, allowing messages to be processed again once the entry is purged.
func DeduplicationContext(fn Deduplicator, ttl time.Duration) ContextDecorator {
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

type dedupContext struct {
	Context
	dedup Deduplicator
	ttl   time.Duration
	seen  map[string]time.Time
}

// Receive intercepts incoming messages and drops duplicates. The entry
// timestamp is refreshed on each reception and purged when older than the TTL.
func (d *dedupContext) Receive(envelope *MessageEnvelope) {
	now := time.Now()
	d.cleanup(now)

	key := d.dedup(envelope.Message)
	if key != "" {
		if last, exists := d.seen[key]; exists {
			if now.Sub(last) < d.ttl {
				d.seen[key] = now
				d.Logger().Info("Request de-duplicated", slog.String("key", key))
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
