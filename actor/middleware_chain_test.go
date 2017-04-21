package actor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func middleware(called *int) InboundMiddleware {
	return func(next ActorFunc) ActorFunc {
		fn := func(context Context) {
			*called = context.Message().(int)

			next(context)
		}
		return fn
	}
}

func TestMakeInboundMiddleware_CallsInCorrectOrder(t *testing.T) {
	var c [3]int

	r := []InboundMiddleware{
		middleware(&c[0]),
		middleware(&c[1]),
		middleware(&c[2]),
	}

	mc := &mockContext{}
	mc.On("Message").Return(1).Once()
	mc.On("Message").Return(2).Once()
	mc.On("Message").Return(3).Once()

	chain := makeInboundMiddlewareChain(r, func(_ Context) {})
	chain(mc)

	assert.Equal(t, 1, c[0])
	assert.Equal(t, 2, c[1])
	assert.Equal(t, 3, c[2])
	mc.AssertExpectations(t)
}

func TestMakeInboundMiddleware_ReturnsNil(t *testing.T) {
	assert.Nil(t, makeInboundMiddlewareChain([]InboundMiddleware{}, func(_ Context) {}))
}
