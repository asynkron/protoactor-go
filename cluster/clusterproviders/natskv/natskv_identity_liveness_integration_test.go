//go:build integration

package natskv

import (
	"context"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// slowKind creates a Kind whose actor sleeps for 2 seconds before responding.
// This simulates a slow (but alive) actor that causes request timeouts.
func slowKind() *cluster.Kind {
	return cluster.NewKind("SlowActor", actor.PropsFromFunc(func(ctx actor.Context) {
		if _, ok := ctx.Message().(slowPingMessage); ok {
			// Simulate slow processing — takes 2 seconds.
			time.Sleep(2 * time.Second)
			ctx.Respond(slowPingResponse{From: ctx.Self().Id})
		}
	}))
}

// slowPingMessage is a request that causes a delayed response.
type slowPingMessage struct{}

// slowPingResponse is the response to a slowPingMessage.
type slowPingResponse struct{ From string }

// TestIntegration_SlowActor_IdentityPreserved verifies the end-to-end fix:
// when a cluster request times out because the actor is slow (not dead),
// the identity record must NOT be deleted. After retry with a longer timeout,
// the same actor should respond successfully.
func TestIntegration_SlowActor_IdentityPreserved(t *testing.T) {
	natsURL := startNATSContainer(t)

	slow := slowKind()

	_, c := startFullCluster(t, natsURL, "integ-slow-actor", []*cluster.Kind{slow}, nil)
	t.Cleanup(func() { c.Shutdown(true) })

	// Allow cluster to settle.
	time.Sleep(1 * time.Second)

	// First request: short timeout → should fail with timeout.
	// The actor is alive but takes 2s; we only wait 500ms.
	_, err := c.Request("slow-grain-1", "SlowActor", slowPingMessage{},
		cluster.WithTimeout(500*time.Millisecond),
		cluster.WithRetryCount(1),
	)
	// Expect timeout error.
	require.Error(t, err, "request with short timeout should fail")

	// The identity record should still exist — the actor is alive, just slow.
	il := c.IdentityLookup.(*IdentityLookup)
	ci := cluster.NewClusterIdentity("slow-grain-1", "SlowActor")
	ctx := context.Background()
	rec := il.getExistingActivation(ctx, ci)
	assert.NotNil(t, rec,
		"identity record must survive a timeout — the actor is alive, just slow")

	// Second request: long timeout → should succeed, same actor responds.
	resp, err := c.Request("slow-grain-1", "SlowActor", slowPingMessage{},
		cluster.WithTimeout(10*time.Second),
		cluster.WithRetryCount(3),
	)
	require.NoError(t, err, "request with long timeout should succeed")
	require.NotNil(t, resp)

	slowResp, ok := resp.(slowPingResponse)
	require.True(t, ok, "response should be slowPingResponse")
	assert.Contains(t, slowResp.From, "SlowActor/slow-grain-1",
		"response should come from the original actor, not a re-spawned one")
}
