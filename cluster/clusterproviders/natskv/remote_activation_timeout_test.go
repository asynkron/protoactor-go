package natskv

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/remote"
)

// TestRemoteActivationTimeoutHonored verifies that requestRemoteActivation
// bounds its round-trip by RemoteActivationTimeout, not LockTTL. A fake member
// responds after a delay that EXCEEDS LockTTL but is UNDER
// RemoteActivationTimeout: with the old LockTTL bound the client would time
// out; with the fix it succeeds.
func TestRemoteActivationTimeoutHonored(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc, _ := connectNATS(t, srv)

	// LockTTL deliberately short; RemoteActivationTimeout comfortably longer.
	p, err := New(nc,
		WithLockTTL(200*time.Millisecond),
		WithRemoteActivationTimeout(3*time.Second),
	)
	require.NoError(t, err)

	system := actor.NewActorSystem()
	remoteConfig := remote.Configure("127.0.0.1", 0)
	clusterConfig := cluster.Configure("test-remote-activation-timeout", p, p.IdentityLookup(), remoteConfig)
	c := cluster.NewCluster(system, clusterConfig)
	c.Remote = remote.NewRemote(system, remoteConfig)
	require.NoError(t, c.Remote.Start())
	t.Cleanup(func() { c.Remote.Shutdown(true) })

	il := p.IdentityLookup()
	il.cluster = c
	il.config = p.config

	// Fake member: subscribe to the activate subject and respond after a delay
	// that exceeds LockTTL (200ms) but is under RemoteActivationTimeout (3s).
	const memberDelay = 600 * time.Millisecond
	subject := activateSubject(c.Config.Name)
	sub, err := nc.Subscribe(subject, func(msg *nats.Msg) {
		time.Sleep(memberDelay)
		resp, _ := json.Marshal(activationResp{
			PidID:      "TestKind/remote-grain",
			PidAddress: "127.0.0.1:9999",
		})
		_ = msg.Respond(resp)
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	ci := testCI("TestKind", "remote-grain")
	start := time.Now()
	rec := il.requestRemoteActivation(context.Background(), ci)
	elapsed := time.Since(start)

	require.NotNil(t, rec, "remote activation must succeed: the client must wait "+
		"RemoteActivationTimeout, not the shorter LockTTL")
	require.Equal(t, "TestKind/remote-grain", rec.PidID)
	require.GreaterOrEqual(t, elapsed, memberDelay,
		"client must have waited for the slow member (past LockTTL)")
	require.Less(t, elapsed, 3*time.Second,
		"client must return well before RemoteActivationTimeout expires")
}
