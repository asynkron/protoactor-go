package cluster

import (
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// spawns an echo kind that captures the received headers and payload
// and responds with an ack.
func newHeaderEchoCluster(t *testing.T, name string, seen chan<- headerObservation) *Cluster {
	t.Helper()
	props := actor.PropsFromFunc(func(ctx actor.Context) {
		if msg, ok := ctx.Message().(string); ok {
			seen <- headerObservation{
				traceID: ctx.MessageHeader().Get("trace-id"),
				tenant:  ctx.MessageHeader().Get("tenant"),
				msg:     msg,
				hdrLen:  ctx.MessageHeader().Length(),
			}
			ctx.Respond("ack")
		}
	})
	kind := NewKind("echo", props)
	c := newClusterForIntegrationTest(name, WithKinds(kind))
	require.NoError(t, c.StartMember())
	t.Cleanup(func() { c.Shutdown(true) })
	return c
}

type headerObservation struct {
	traceID string
	tenant  string
	msg     string
	hdrLen  int
}

func TestCluster_Request_WithHeadersOption_ReceiverSeesHeaders(t *testing.T) {
	t.Parallel()

	seen := make(chan headerObservation, 1)
	c := newHeaderEchoCluster(t, "test-headers-option", seen)

	resp, err := c.Request("id-1", "echo", "hello", WithHeaders(map[string]string{
		"trace-id": "abc",
		"tenant":   "acme",
	}))
	assert.NoError(t, err)
	assert.Equal(t, "ack", resp)

	obs := <-seen
	assert.Equal(t, "abc", obs.traceID)
	assert.Equal(t, "acme", obs.tenant)
	assert.Equal(t, "hello", obs.msg)
	assert.Equal(t, 2, obs.hdrLen)
}

func TestCluster_Request_NoHeaders_EmptyOnReceiver(t *testing.T) {
	t.Parallel()

	seen := make(chan headerObservation, 1)
	c := newHeaderEchoCluster(t, "test-headers-empty", seen)

	_, err := c.Request("id-1", "echo", "hello")
	assert.NoError(t, err)

	obs := <-seen
	assert.Equal(t, 0, obs.hdrLen)
}

func TestCluster_Request_PrewrappedEnvelopeHeadersWinOverOption(t *testing.T) {
	t.Parallel()

	seen := make(chan headerObservation, 1)
	c := newHeaderEchoCluster(t, "test-headers-merge", seen)

	env := actor.WrapEnvelope("hello")
	env.SetHeader("trace-id", "from-envelope")
	// "tenant" is unique to the option.

	resp, err := c.Request("id-1", "echo", env, WithHeaders(map[string]string{
		"trace-id": "from-option", // should lose
		"tenant":   "acme",        // should be added
	}))
	assert.NoError(t, err)
	assert.Equal(t, "ack", resp)

	obs := <-seen
	assert.Equal(t, "from-envelope", obs.traceID, "envelope wins on conflict")
	assert.Equal(t, "acme", obs.tenant, "non-overlapping option key is added")
	assert.Equal(t, 2, obs.hdrLen)
}
