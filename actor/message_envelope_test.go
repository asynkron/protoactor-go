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
