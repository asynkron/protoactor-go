package cluster

import (
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

func TestDefaultContext_timeoutLogThrottle(t *testing.T) {
	t.Parallel()

	cp := newInmemoryProvider()
	c := newClusterForTest("test-throttle", cp)

	ctx := newDefaultClusterContext(c)
	dcc, ok := ctx.(*DefaultContext)
	assert.True(t, ok)
	assert.NotNil(t, dcc.requestTimeoutLogThrottle)
	assert.NotNil(t, dcc.futureTimeoutLogThrottle)

	// Exhaust request throttle
	for i := 0; i < 4; i++ {
		valve := dcc.requestTimeoutLogThrottle()
		assert.Equal(t, actor.Open, valve, "request call %d should be Open", i)
	}
	valve := dcc.requestTimeoutLogThrottle()
	assert.Equal(t, actor.Closing, valve)
	valve = dcc.requestTimeoutLogThrottle()
	assert.Equal(t, actor.Closed, valve)

	// Future throttle should still be fully Open (independent)
	valve = dcc.futureTimeoutLogThrottle()
	assert.Equal(t, actor.Open, valve, "future throttle should be independent")
}

func TestDefaultContext_timeoutLogThrottle_resetsAfterPeriod(t *testing.T) {
	t.Parallel()

	cp := newInmemoryProvider()
	c := newClusterForTest("test-throttle-reset", cp)

	// Create a custom context with a short throttle period for testing reset
	dcc := &DefaultContext{
		cluster: c,
		requestTimeoutLogThrottle: actor.NewThrottle(2, 100*time.Millisecond, func(count int32) {}),
		futureTimeoutLogThrottle:  actor.NewThrottle(2, 100*time.Millisecond, func(count int32) {}),
	}

	// Exhaust the throttle
	assert.Equal(t, actor.Open, dcc.requestTimeoutLogThrottle())
	assert.Equal(t, actor.Closing, dcc.requestTimeoutLogThrottle())
	assert.Equal(t, actor.Closed, dcc.requestTimeoutLogThrottle())

	// Wait for the throttle period to reset
	time.Sleep(200 * time.Millisecond)

	// After reset, should be Open again
	assert.Equal(t, actor.Open, dcc.requestTimeoutLogThrottle())
}
