package testkit

import (
	"errors"
	"fmt"
	"time"

	"github.com/asynkron/protoactor-go/actor"
)

// messageAndSender captures a message and its sender.
type messageAndSender struct {
	message interface{}
	sender  *actor.PID
}

// TestProbe is an actor used in tests to intercept and assert on messages.
// It stores every incoming message along with its sender.
type TestProbe struct {
	mailbox chan messageAndSender
	ctx     actor.Context
	sender  *actor.PID
}

// NewTestProbe creates a new TestProbe instance.
func NewTestProbe() *TestProbe {
	return &TestProbe{mailbox: make(chan messageAndSender, 100)}
}

// Receive implements actor.Actor. It records all messages after initialization.
func (tp *TestProbe) Receive(ctx actor.Context) {
	switch ctx.Message().(type) {
	case *actor.Started:
		tp.ctx = ctx
	default:
		tp.mailbox <- messageAndSender{message: ctx.Message(), sender: ctx.Sender()}
	}
}

// Context returns the probe's context. Panics if called before Start.
func (tp *TestProbe) Context() actor.Context {
	if tp.ctx == nil {
		panic("probe context is nil")
	}
	return tp.ctx
}

// Sender returns the sender of the last retrieved message.
func (tp *TestProbe) Sender() *actor.PID { return tp.sender }

// ExpectNoMessage verifies that no message arrives within the allowed time.
func (tp *TestProbe) ExpectNoMessage(timeAllowed time.Duration) error {
	if timeAllowed == 0 {
		timeAllowed = time.Second
	}
	select {
	case m := <-tp.mailbox:
		return fmt.Errorf("waited %v and received message of type %T", timeAllowed, m.message)
	case <-time.After(timeAllowed):
		return nil
	}
}

// GetNextMessage waits for the next message and returns it.
func (tp *TestProbe) GetNextMessage(timeAllowed time.Duration) (interface{}, error) {
	if timeAllowed == 0 {
		timeAllowed = time.Second
	}
	select {
	case m := <-tp.mailbox:
		tp.sender = m.sender
		return m.message, nil
	case <-time.After(timeAllowed):
		return nil, fmt.Errorf("waited %v but failed to receive a message", timeAllowed)
	}
}

// Send forwards a message to the target actor.
func (tp *TestProbe) Send(target *actor.PID, message interface{}) { tp.Context().Send(target, message) }

// Request sends a request message from the probe to the target.
func (tp *TestProbe) Request(target *actor.PID, message interface{}) {
	tp.Context().Request(target, message)
}

// Respond sends a response to the last sender if present.
func (tp *TestProbe) Respond(message interface{}) {
	if tp.sender != nil {
		tp.Send(tp.sender, message)
	}
}

// Helper functions -------------------------------------------------------

// GetNextMessageOf waits for the next message and asserts its type.
func GetNextMessageOf[T any](tp *TestProbe, timeAllowed time.Duration) (T, error) {
	var zero T
	msg, err := tp.GetNextMessage(timeAllowed)
	if err != nil {
		return zero, err
	}
	typed, ok := msg.(T)
	if !ok {
		return zero, fmt.Errorf("message expected type %T, actual type %T", zero, msg)
	}
	return typed, nil
}

// GetNextMessageWhere waits for the next message of type T matching the predicate.
func GetNextMessageWhere[T any](tp *TestProbe, when func(T) bool, timeAllowed time.Duration) (T, error) {
	msg, err := GetNextMessageOf[T](tp, timeAllowed)
	if err != nil {
		var zero T
		return zero, err
	}
	if !when(msg) {
		var zero T
		return zero, errors.New("condition not met")
	}
	return msg, nil
}

// FishForMessage waits until a message of type T arrives that satisfies when.
func FishForMessage[T any](tp *TestProbe, when func(T) bool, timeAllowed time.Duration) (T, error) {
	var zero T
	if timeAllowed == 0 {
		timeAllowed = time.Second
	}
	deadline := time.Now().Add(timeAllowed)
	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		select {
		case m := <-tp.mailbox:
			if typed, ok := m.message.(T); ok && when(typed) {
				tp.sender = m.sender
				return typed, nil
			}
		case <-time.After(remaining):
			return zero, errors.New("message not found")
		}
	}
	return zero, errors.New("message not found")
}

// FishForMessageOf waits for any message of type T.
func FishForMessageOf[T any](tp *TestProbe, timeAllowed time.Duration) (T, error) {
	return FishForMessage(tp, func(T) bool { return true }, timeAllowed)
}

// RequestFuture sends a request and waits for a typed response.
func RequestFuture[T any](tp *TestProbe, target *actor.PID, message interface{}, timeout time.Duration) (T, error) {
	var zero T
	res, err := tp.Context().RequestFuture(target, message, timeout).Result()
	if err != nil {
		return zero, err
	}
	typed, ok := res.(T)
	if !ok {
		return zero, fmt.Errorf("message expected type %T, actual type %T", zero, res)
	}
	return typed, nil
}

// implicit PID conversion is not available in Go; use tp.Context().Self() for the probe PID.
