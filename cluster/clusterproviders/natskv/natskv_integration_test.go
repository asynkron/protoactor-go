//go:build integration

package natskv

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/remote"
)

// startNATSContainer starts a real NATS server in a Docker container with JetStream
// enabled and returns the connection URL. The container is terminated when the test
// completes.
func startNATSContainer(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "nats:latest",
		ExposedPorts: []string{"4222/tcp"},
		Cmd:          []string{"-js"},
		WaitingFor:   wait.ForListeningPort("4222/tcp").WithStartupTimeout(30 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })
	endpoint, err := container.Endpoint(ctx, "")
	require.NoError(t, err)
	return "nats://" + endpoint
}

// setupClusterFromURL creates a provider, actor system, and cluster for testing
// using an external NATS URL (such as one from a testcontainer) instead of the
// embedded NATS server. It returns the provider and cluster.
func setupClusterFromURL(t *testing.T, natsURL, clusterName string, opts ...Option) (*Provider, *cluster.Cluster) {
	t.Helper()
	nc, err := nats.Connect(natsURL)
	require.NoError(t, err)
	t.Cleanup(func() { nc.Close() })

	p, err := New(nc, opts...)
	require.NoError(t, err)

	system := actor.NewActorSystem()
	remoteCfg := remote.Configure("127.0.0.1", 0)
	clusterCfg := cluster.Configure(clusterName, p, p.IdentityLookup(), remoteCfg)
	c := cluster.NewCluster(system, clusterCfg)

	// Initialize the remote so that ActorSystem.Address() returns a proper host:port.
	c.Remote = remote.NewRemote(system, remoteCfg)

	return p, c
}

// TestIntegration_TwoMemberCluster starts two members on the same NATS container
// and verifies that they discover each other through topology convergence.
func TestIntegration_TwoMemberCluster(t *testing.T) {
	natsURL := startNATSContainer(t)

	p1, c1 := setupClusterFromURL(t, natsURL, "integ-two-member")
	err := p1.StartMember(c1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p1.Shutdown(true) })

	// Allow first member to register and settle.
	time.Sleep(500 * time.Millisecond)

	p2, c2 := setupClusterFromURL(t, natsURL, "integ-two-member")
	err = p2.StartMember(c2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	// Wait for topology convergence: each member should see the other.
	require.Eventually(t, func() bool {
		p1.membersMu.RLock()
		_, p1SeesP2 := p1.members[p2.self.ID]
		p1.membersMu.RUnlock()
		return p1SeesP2
	}, 10*time.Second, 200*time.Millisecond, "member 1 should discover member 2")

	require.Eventually(t, func() bool {
		p2.membersMu.RLock()
		_, p2SeesP1 := p2.members[p1.self.ID]
		p2.membersMu.RUnlock()
		return p2SeesP1
	}, 10*time.Second, 200*time.Millisecond, "member 2 should discover member 1")

	// Verify each member's own self is set correctly.
	assert.NotEmpty(t, p1.self.ID, "member 1 should have a non-empty ID")
	assert.NotEmpty(t, p2.self.ID, "member 2 should have a non-empty ID")
	assert.NotEqual(t, p1.self.ID, p2.self.ID, "members should have distinct IDs")
}

// TestIntegration_MemberJoinLeave starts two members, verifies mutual discovery,
// then gracefully shuts down member 2 and verifies that member 1 detects the departure.
func TestIntegration_MemberJoinLeave(t *testing.T) {
	natsURL := startNATSContainer(t)

	// Use short TTL so departure is detected quickly.
	ttl := 3 * time.Second
	refresh := 1 * time.Second

	p1, c1 := setupClusterFromURL(t, natsURL, "integ-join-leave",
		WithMemberTTL(ttl),
		WithRefreshInterval(refresh),
	)
	err := p1.StartMember(c1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p1.Shutdown(true) })

	time.Sleep(500 * time.Millisecond)

	p2, c2 := setupClusterFromURL(t, natsURL, "integ-join-leave",
		WithMemberTTL(ttl),
		WithRefreshInterval(refresh),
	)
	err = p2.StartMember(c2)
	require.NoError(t, err)

	// Wait for member 1 to see member 2.
	p2ID := p2.self.ID
	require.Eventually(t, func() bool {
		p1.membersMu.RLock()
		_, found := p1.members[p2ID]
		p1.membersMu.RUnlock()
		return found
	}, 10*time.Second, 200*time.Millisecond, "member 1 should discover member 2")

	// Gracefully shut down member 2.
	err = p2.Shutdown(true)
	require.NoError(t, err)

	// Verify member 1 detects the departure.
	require.Eventually(t, func() bool {
		p1.membersMu.RLock()
		_, stillFound := p1.members[p2ID]
		p1.membersMu.RUnlock()
		return !stillFound
	}, 10*time.Second, 200*time.Millisecond, "member 1 should detect member 2 departure")
}

// TestIntegration_LeaderElection_ThreeNodes starts three members and verifies
// that exactly one of them becomes the leader.
func TestIntegration_LeaderElection_ThreeNodes(t *testing.T) {
	natsURL := startNATSContainer(t)

	providers := make([]*Provider, 3)
	clusters := make([]*cluster.Cluster, 3)

	for i := 0; i < 3; i++ {
		p, c := setupClusterFromURL(t, natsURL, "integ-leader-3")
		err := p.StartMember(c)
		require.NoError(t, err)
		t.Cleanup(func() { _ = p.Shutdown(true) })

		providers[i] = p
		clusters[i] = c

		// Stagger startup slightly so leader election ordering is predictable.
		time.Sleep(300 * time.Millisecond)
	}

	// Wait for leader election to settle.
	require.Eventually(t, func() bool {
		leaderCount := 0
		for _, p := range providers {
			if p.isLeader.Load() {
				leaderCount++
			}
		}
		return leaderCount == 1
	}, 10*time.Second, 200*time.Millisecond, "exactly one provider should be leader")

	// Count leaders and followers for final assertion.
	leaderCount := 0
	followerCount := 0
	for _, p := range providers {
		if p.isLeader.Load() {
			leaderCount++
		} else {
			followerCount++
		}
	}

	assert.Equal(t, 1, leaderCount, "exactly one node should be leader")
	assert.Equal(t, 2, followerCount, "exactly two nodes should be followers")
}
