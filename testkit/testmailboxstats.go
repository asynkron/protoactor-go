package testkit

import (
	"sync"

	"github.com/asynkron/protoactor-go/actor"
)

// TestMailboxStats collects mailbox events for tests.
type TestMailboxStats struct {
	waitForReceived func(interface{}) bool
	Reset           chan struct{}
	Stats           []interface{}
	Posted          []interface{}
	Received        []interface{}

	mu sync.Mutex
}

// NewTestMailboxStats creates a new stats collector.
func NewTestMailboxStats(wait func(interface{}) bool) *TestMailboxStats {
	return &TestMailboxStats{
		waitForReceived: wait,
		Reset:           make(chan struct{}, 1),
	}
}

// MailboxStarted records the start event.
func (t *TestMailboxStats) MailboxStarted() {
	t.mu.Lock()
	t.Stats = append(t.Stats, "Started")
	t.mu.Unlock()
}

// MessagePosted records a posted message.
func (t *TestMailboxStats) MessagePosted(message interface{}) {
	t.mu.Lock()
	t.Stats = append(t.Stats, message)
	t.Posted = append(t.Posted, message)
	t.mu.Unlock()
}

// MessageReceived records a received message and signals if predicate matches.
func (t *TestMailboxStats) MessageReceived(message interface{}) {
	t.mu.Lock()
	t.Stats = append(t.Stats, message)
	t.Received = append(t.Received, message)
	t.mu.Unlock()
	if t.waitForReceived != nil && t.waitForReceived(message) {
		select {
		case t.Reset <- struct{}{}:
		default:
		}
	}
}

// MailboxEmpty records an empty mailbox event.
func (t *TestMailboxStats) MailboxEmpty() {
	t.mu.Lock()
	t.Stats = append(t.Stats, "Empty")
	t.mu.Unlock()
}

var _ actor.MailboxMiddleware = (*TestMailboxStats)(nil)

// ReceiverMiddleware converts the stats collector into a receiver middleware.
// Every received message is recorded before being passed on.
func (t *TestMailboxStats) ReceiverMiddleware() actor.ReceiverMiddleware {
	return func(next actor.ReceiverFunc) actor.ReceiverFunc {
		return func(ctx actor.ReceiverContext, envelope *actor.MessageEnvelope) {
			if envelope != nil {
				t.MessageReceived(envelope.Message)
			}
			next(ctx, envelope)
		}
	}
}

// SenderMiddleware converts the stats collector into a sender middleware.
// Each sent message is recorded before being forwarded.
func (t *TestMailboxStats) SenderMiddleware() actor.SenderMiddleware {
	return func(next actor.SenderFunc) actor.SenderFunc {
		return func(ctx actor.SenderContext, target *actor.PID, env *actor.MessageEnvelope) {
			if env != nil {
				t.MessagePosted(env.Message)
			}
			next(ctx, target, env)
		}
	}
}

// WithMailboxStats configures props with a mailbox that records stats.
func WithMailboxStats(stats *TestMailboxStats) actor.PropsOption {
	return actor.WithMailbox(actor.Unbounded(stats))
}

// WithReceiveStats applies the stats collector as a receive middleware.
func WithReceiveStats(stats *TestMailboxStats) actor.PropsOption {
	return actor.WithReceiverMiddleware(stats.ReceiverMiddleware())
}

// WithSendStats applies the stats collector as a send middleware.
func WithSendStats(stats *TestMailboxStats) actor.PropsOption {
	return actor.WithSenderMiddleware(stats.SenderMiddleware())
}
