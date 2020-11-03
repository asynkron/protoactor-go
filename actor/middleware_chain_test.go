package actor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func middleware(called *int) ReceiverMiddleware {
	return func(next ReceiverFunc) ReceiverFunc {
		fn := func(ctx ReceiverContext, env *MessageEnvelope) {
			env.Message = env.Message.(int) + 1
			*called = env.Message.(int)

			next(ctx, env)
		}
		return fn
	}
}

func TestMakeReceiverMiddleware_CallsInCorrectOrder(t *testing.T) {
	var c [3]int

	r := []ReceiverMiddleware{
		middleware(&c[0]),
		middleware(&c[1]),
		middleware(&c[2]),
	}

	mc := &mockContext{}

	env := &MessageEnvelope{
		Message: 0,
	}

	chain := makeReceiverMiddlewareChain(r, func(receiver ReceiverContext, env *MessageEnvelope) {})
	chain(mc, env)

	assert.Equal(t, 1, c[0])
	assert.Equal(t, 2, c[1])
	assert.Equal(t, 3, c[2])
}

func TestMakeInboundMiddleware_ReturnsNil(t *testing.T) {
	assert.Nil(t, makeReceiverMiddlewareChain([]ReceiverMiddleware{}, func(_ ReceiverContext, _ *MessageEnvelope) {}))
}
