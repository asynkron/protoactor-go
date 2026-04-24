package actor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalMessageGivesEmptyMessageHeaders(t *testing.T) {
	t.Parallel()

	props := PropsFromFunc(func(ctx Context) {
		if _, ok := ctx.Message().(string); ok {
			l := len(ctx.MessageHeader().Keys())
			ctx.Respond(l)
		}
	})
	a := rootContext.Spawn(props)

	defer func() {
		_ = rootContext.StopFuture(a).Wait()
	}()

	f := rootContext.RequestFuture(a, "hello", testTimeout)

	res, _ := assertFutureSuccess(f, t).(int)
	assert.Equal(t, 0, res)
}

func TestEnvelopeWithSender_RawMessage_NilSender(t *testing.T) {
	t.Parallel()

	out := envelopeWithSender("hello", nil)
	assert.NotNil(t, out)
	assert.Nil(t, out.Header)
	assert.Equal(t, "hello", out.Message)
	assert.Nil(t, out.Sender)
}

func TestEnvelopeWithSender_RawMessage_WithSender(t *testing.T) {
	t.Parallel()

	sender := NewPID("addr", "id")
	out := envelopeWithSender("hello", sender)
	assert.NotNil(t, out)
	assert.Nil(t, out.Header)
	assert.Equal(t, "hello", out.Message)
	assert.Same(t, sender, out.Sender)
}

func TestEnvelopeWithSender_Envelope_NilSender_ReturnsSame(t *testing.T) {
	t.Parallel()

	in := &MessageEnvelope{
		Header:  messageHeader{"k": "v"},
		Message: "hello",
		Sender:  NewPID("a", "1"),
	}
	out := envelopeWithSender(in, nil)
	assert.Same(t, in, out, "with nil sender, envelope should pass through unchanged")
}

func TestEnvelopeWithSender_Envelope_WithSender_ClonesAndOverrides(t *testing.T) {
	t.Parallel()

	originalSender := NewPID("orig", "1")
	in := &MessageEnvelope{
		Header:  messageHeader{"trace": "abc"},
		Message: "hello",
		Sender:  originalSender,
	}

	newSender := NewPID("new", "2")
	out := envelopeWithSender(in, newSender)

	// Clone returned, caller's envelope untouched.
	assert.NotSame(t, in, out)
	assert.Same(t, originalSender, in.Sender, "caller's envelope must not be mutated")

	// Clone has header preserved and sender replaced.
	assert.Equal(t, "abc", out.Header.Get("trace"))
	assert.Equal(t, "hello", out.Message)
	assert.Same(t, newSender, out.Sender)
}

func TestEnvelopeWithHeaders_RawMessage_EmptyHeaders(t *testing.T) {
	t.Parallel()

	out := EnvelopeWithHeaders("hello", nil)
	assert.NotNil(t, out)
	assert.Nil(t, out.Header)
	assert.Equal(t, "hello", out.Message)
	assert.Nil(t, out.Sender)
}

func TestEnvelopeWithHeaders_RawMessage_WithHeaders(t *testing.T) {
	t.Parallel()

	out := EnvelopeWithHeaders("hello", map[string]string{"k": "v"})
	assert.NotNil(t, out)
	assert.Equal(t, 1, out.Header.Length())
	assert.Equal(t, "v", out.Header.Get("k"))
	assert.Equal(t, "hello", out.Message)
}

func TestEnvelopeWithHeaders_Envelope_EmptyHeaders_ReturnsSame(t *testing.T) {
	t.Parallel()

	in := &MessageEnvelope{
		Header:  messageHeader{"k": "v"},
		Message: "hello",
	}
	out := EnvelopeWithHeaders(in, nil)
	assert.Same(t, in, out)
}

func TestEnvelopeWithHeaders_Envelope_WithHeaders_ClonesAndMerges(t *testing.T) {
	t.Parallel()

	in := &MessageEnvelope{
		Header:  messageHeader{"k": "envelope-value"},
		Message: "hello",
	}

	out := EnvelopeWithHeaders(in, map[string]string{
		"k": "option-value", // should lose to envelope
		"m": "n",            // should be added
	})

	assert.NotSame(t, in, out, "must clone to avoid mutating caller's envelope")
	assert.Equal(t, 1, in.Header.Length(), "caller's envelope must not be mutated")
	assert.Equal(t, "envelope-value", in.Header.Get("k"))

	assert.Equal(t, 2, out.Header.Length())
	assert.Equal(t, "envelope-value", out.Header.Get("k"), "envelope wins on conflict")
	assert.Equal(t, "n", out.Header.Get("m"), "non-overlapping option key added")
	assert.Equal(t, "hello", out.Message)
}

func TestEnvelopeWithHeaders_Envelope_NilHeader_AddsHeaders(t *testing.T) {
	t.Parallel()

	in := &MessageEnvelope{
		Header:  nil,
		Message: "hello",
	}

	out := EnvelopeWithHeaders(in, map[string]string{"k": "v"})

	assert.NotSame(t, in, out)
	assert.Nil(t, in.Header, "caller's envelope header must remain nil")
	assert.Equal(t, 1, out.Header.Length())
	assert.Equal(t, "v", out.Header.Get("k"))
}

// Guards the glue between the cluster-side envelope construction and the
// remote outbound path: remote/remote_process.go:22 calls UnwrapEnvelope on
// the outgoing envelope and hands header/message/sender to SendMessage. Any
// envelope built by EnvelopeWithHeaders must round-trip through UnwrapEnvelope
// with headers intact.
func TestEnvelopeWithHeaders_UnwrapsCleanlyForRemote(t *testing.T) {
	t.Parallel()

	env := EnvelopeWithHeaders("hello", map[string]string{"trace-id": "x"})

	header, msg, sender := UnwrapEnvelope(env)
	assert.Equal(t, "x", header.Get("trace-id"))
	assert.Equal(t, "hello", msg, "payload must be the inner message, not the envelope")
	assert.Nil(t, sender)
}

// Same guard when the cluster layer merges option headers with a caller's
// pre-wrapped envelope: UnwrapEnvelope must see the merged header map.
func TestEnvelopeWithHeaders_PrewrappedMergeUnwrapsCleanly(t *testing.T) {
	t.Parallel()

	in := WrapEnvelope("hello")
	in.SetHeader("trace-id", "envelope-wins")

	env := EnvelopeWithHeaders(in, map[string]string{
		"trace-id": "should-lose",
		"tenant":   "acme",
	})

	header, msg, _ := UnwrapEnvelope(env)
	assert.Equal(t, "envelope-wins", header.Get("trace-id"))
	assert.Equal(t, "acme", header.Get("tenant"))
	assert.Equal(t, "hello", msg)
}
