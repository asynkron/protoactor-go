package actor

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootContext_TrySpawn(t *testing.T) {
	props := PropsFromProducer(func() Actor { return nullReceive })
	pid, err := rootContext.TrySpawn(props)
	require.NoError(t, err)
	require.NotNil(t, pid)
	defer rootContext.Stop(pid)

	assert.NotEmpty(t, pid.Id)
}

func TestRootContext_TrySpawn_Error(t *testing.T) {
	spawnErr := errors.New("test spawn error")
	props := PropsFromProducer(func() Actor { return nullReceive },
		WithSpawnFunc(func(actorSystem *ActorSystem, id string, props *Props, parentContext SpawnerContext) (*PID, error) {
			return nil, spawnErr
		}),
	)

	pid, err := rootContext.TrySpawn(props)
	assert.Nil(t, pid)
	assert.ErrorIs(t, err, spawnErr)
}

func TestRootContext_TrySpawnPrefix(t *testing.T) {
	props := PropsFromProducer(func() Actor { return nullReceive })
	pid, err := rootContext.TrySpawnPrefix(props, "myprefix")
	require.NoError(t, err)
	require.NotNil(t, pid)
	defer rootContext.Stop(pid)

	assert.True(t, strings.HasPrefix(pid.Id, "myprefix"), "expected PID Id to start with 'myprefix', got: %s", pid.Id)
}

// Asserts pre-wrapped envelope headers survive RequestFuture via RootContext.
func TestRootContext_RequestFuture_PreservesEnvelopeHeaders(t *testing.T) {
	t.Parallel()

	type gotHeaders struct {
		traceID string
		msg     any
		hdrLen  int
	}
	result := make(chan gotHeaders, 1)

	props := PropsFromFunc(func(ctx Context) {
		if _, ok := ctx.Message().(string); ok {
			result <- gotHeaders{
				traceID: ctx.MessageHeader().Get("trace-id"),
				msg:     ctx.Message(),
				hdrLen:  ctx.MessageHeader().Length(),
			}
			ctx.Respond("ok")
		}
	})
	a := rootContext.Spawn(props)
	defer func() { _ = rootContext.StopFuture(a).Wait() }()

	env := WrapEnvelope("hello")
	env.SetHeader("trace-id", "abc")

	f := rootContext.RequestFuture(a, env, testTimeout)
	_, err := f.Result()
	assert.NoError(t, err)

	got := <-result
	assert.Equal(t, "abc", got.traceID, "receiver must see envelope header")
	assert.Equal(t, "hello", got.msg, "receiver's ctx.Message() must be the inner payload, not the envelope")
	assert.Equal(t, 1, got.hdrLen)
}

// Asserts raw-message RequestFuture still produces empty headers (regression).
func TestRootContext_RequestFuture_RawMessage_EmptyHeaders(t *testing.T) {
	t.Parallel()

	hdrLen := make(chan int, 1)

	props := PropsFromFunc(func(ctx Context) {
		if _, ok := ctx.Message().(string); ok {
			hdrLen <- ctx.MessageHeader().Length()
			ctx.Respond("ok")
		}
	})
	a := rootContext.Spawn(props)
	defer func() { _ = rootContext.StopFuture(a).Wait() }()

	f := rootContext.RequestFuture(a, "hello", testTimeout)
	_, err := f.Result()
	assert.NoError(t, err)

	assert.Equal(t, 0, <-hdrLen)
}

// Asserts RequestWithCustomSender preserves envelope headers and overrides sender.
func TestRootContext_RequestWithCustomSender_PreservesHeadersOverridesSender(t *testing.T) {
	t.Parallel()

	type received struct {
		sender  *PID
		traceID string
		msg     any
	}
	got := make(chan received, 1)

	props := PropsFromFunc(func(ctx Context) {
		if _, ok := ctx.Message().(string); ok {
			got <- received{
				sender:  ctx.Sender(),
				traceID: ctx.MessageHeader().Get("trace-id"),
				msg:     ctx.Message(),
			}
		}
	})
	a := rootContext.Spawn(props)
	defer func() { _ = rootContext.StopFuture(a).Wait() }()

	customSender := NewPID("custom-addr", "custom-id")
	env := WrapEnvelope("hello")
	env.SetHeader("trace-id", "xyz")
	env.Sender = NewPID("other", "other") // should be overridden

	rootContext.RequestWithCustomSender(a, env, customSender)

	r := <-got
	assert.Equal(t, "xyz", r.traceID)
	assert.Equal(t, "hello", r.msg)
	assert.Equal(t, customSender.Address, r.sender.Address)
	assert.Equal(t, customSender.Id, r.sender.Id)
}
