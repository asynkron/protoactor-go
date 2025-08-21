package testkit

import "github.com/asynkron/protoactor-go/actor"

// TestMailboxStats collects mailbox events for tests.
type TestMailboxStats struct {
	waitForReceived func(interface{}) bool
	Reset           chan struct{}
	Stats           []interface{}
	Posted          []interface{}
	Received        []interface{}
}

// NewTestMailboxStats creates a new stats collector.
func NewTestMailboxStats(wait func(interface{}) bool) *TestMailboxStats {
	return &TestMailboxStats{
		waitForReceived: wait,
		Reset:           make(chan struct{}, 1),
	}
}

// MailboxStarted records the start event.
func (t *TestMailboxStats) MailboxStarted() { t.Stats = append(t.Stats, "Started") }

// MessagePosted records a posted message.
func (t *TestMailboxStats) MessagePosted(message interface{}) {
	t.Stats = append(t.Stats, message)
	t.Posted = append(t.Posted, message)
}

// MessageReceived records a received message and signals if predicate matches.
func (t *TestMailboxStats) MessageReceived(message interface{}) {
	t.Stats = append(t.Stats, message)
	t.Received = append(t.Received, message)
	if t.waitForReceived != nil && t.waitForReceived(message) {
		select {
		case t.Reset <- struct{}{}:
		default:
		}
	}
}

// MailboxEmpty records an empty mailbox event.
func (t *TestMailboxStats) MailboxEmpty() { t.Stats = append(t.Stats, "Empty") }

var _ actor.MailboxMiddleware = (*TestMailboxStats)(nil)
