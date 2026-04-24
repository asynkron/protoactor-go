//go:build integration

package natsstream

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

// startNATSContainer starts a NATS server in a Docker container with JetStream enabled.
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

// setupClusterFromURL creates a provider and cluster using an external NATS URL.
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
	c.Remote = remote.NewRemote(system, remoteCfg)

	return p, c
}

func TestIntegration_TwoMemberCluster(t *testing.T) {
	natsURL := startNATSContainer(t)

	p1, c1 := setupClusterFromURL(t, natsURL, "integ-two-member")
	err := p1.StartMember(c1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p1.Shutdown(true) })

	time.Sleep(500 * time.Millisecond)

	p2, c2 := setupClusterFromURL(t, natsURL, "integ-two-member")
	err = p2.StartMember(c2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	require.Eventually(t, func() bool {
		p1.membersMu.RLock()
		_, found := p1.members[p2.self.ID]
		p1.membersMu.RUnlock()
		return found
	}, 10*time.Second, 200*time.Millisecond, "member 1 should discover member 2")

	require.Eventually(t, func() bool {
		p2.membersMu.RLock()
		_, found := p2.members[p1.self.ID]
		p2.membersMu.RUnlock()
		return found
	}, 10*time.Second, 200*time.Millisecond, "member 2 should discover member 1")

	assert.NotEmpty(t, p1.self.ID)
	assert.NotEmpty(t, p2.self.ID)
	assert.NotEqual(t, p1.self.ID, p2.self.ID)
}

func TestIntegration_MemberJoinLeave(t *testing.T) {
	natsURL := startNATSContainer(t)

	opts := []Option{
		WithHeartbeatInterval(500 * time.Millisecond),
		WithHeartbeatTTL(2 * time.Second),
		WithMemberTimeout(3 * time.Second),
		WithCheckInterval(500 * time.Millisecond),
	}

	p1, c1 := setupClusterFromURL(t, natsURL, "integ-join-leave", opts...)
	err := p1.StartMember(c1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p1.Shutdown(true) })

	time.Sleep(500 * time.Millisecond)

	p2, c2 := setupClusterFromURL(t, natsURL, "integ-join-leave", opts...)
	err = p2.StartMember(c2)
	require.NoError(t, err)

	p2ID := p2.self.ID
	require.Eventually(t, func() bool {
		p1.membersMu.RLock()
		_, found := p1.members[p2ID]
		p1.membersMu.RUnlock()
		return found
	}, 10*time.Second, 200*time.Millisecond, "member 1 should discover member 2")

	err = p2.Shutdown(true)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		p1.membersMu.RLock()
		_, found := p1.members[p2ID]
		p1.membersMu.RUnlock()
		return !found
	}, 10*time.Second, 200*time.Millisecond, "member 1 should detect member 2 departure")
}

func TestIntegration_MemberCrash(t *testing.T) {
	natsURL := startNATSContainer(t)

	opts := []Option{
		WithHeartbeatInterval(200 * time.Millisecond),
		WithHeartbeatTTL(1 * time.Second),
		WithMemberTimeout(2 * time.Second),
		WithCheckInterval(200 * time.Millisecond),
	}

	p1, c1 := setupClusterFromURL(t, natsURL, "integ-crash", opts...)
	err := p1.StartMember(c1)
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	p2, c2 := setupClusterFromURL(t, natsURL, "integ-crash", opts...)
	err = p2.StartMember(c2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	p1ID := p1.self.ID
	require.Eventually(t, func() bool {
		p2.membersMu.RLock()
		_, found := p2.members[p1ID]
		p2.membersMu.RUnlock()
		return found
	}, 10*time.Second, 200*time.Millisecond, "p2 should discover p1")

	// Crash p1 (no graceful leave).
	p1.shutdown.Store(true)
	if p1.cancel != nil {
		p1.cancel()
	}
	p1.wg.Wait()

	// p2 should detect crash via local timeout.
	require.Eventually(t, func() bool {
		p2.membersMu.RLock()
		_, found := p2.members[p1ID]
		p2.membersMu.RUnlock()
		return !found
	}, 10*time.Second, 200*time.Millisecond, "p2 should detect p1 crash")
}

func TestIntegration_LeaderElection_ThreeNodes(t *testing.T) {
	natsURL := startNATSContainer(t)

	providers := make([]*Provider, 3)
	for i := 0; i < 3; i++ {
		p, c := setupClusterFromURL(t, natsURL, "integ-leader-3")
		err := p.StartMember(c)
		require.NoError(t, err)
		t.Cleanup(func() { _ = p.Shutdown(true) })
		providers[i] = p
		time.Sleep(300 * time.Millisecond)
	}

	require.Eventually(t, func() bool {
		leaderCount := 0
		for _, p := range providers {
			if p.isLeader.Load() {
				leaderCount++
			}
		}
		return leaderCount == 1
	}, 10*time.Second, 200*time.Millisecond, "exactly one should be leader")

	leaderCount := 0
	for _, p := range providers {
		if p.isLeader.Load() {
			leaderCount++
		}
	}
	assert.Equal(t, 1, leaderCount)
}

func TestIntegration_LeaderFailover(t *testing.T) {
	natsURL := startNATSContainer(t)

	leaderTTL := 3 * time.Second
	opts := []Option{
		WithLeaderTTL(leaderTTL),
		WithHeartbeatInterval(200 * time.Millisecond),
		WithHeartbeatTTL(1 * time.Second),
		WithMemberTimeout(2 * time.Second),
		WithCheckInterval(200 * time.Millisecond),
	}

	p1, c1 := setupClusterFromURL(t, natsURL, "integ-failover", opts...)
	err := p1.StartMember(c1)
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	p2, c2 := setupClusterFromURL(t, natsURL, "integ-failover", opts...)
	err = p2.StartMember(c2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	require.Eventually(t, func() bool {
		return p1.isLeader.Load()
	}, 5*time.Second, 100*time.Millisecond, "p1 should be leader")

	// Crash p1.
	p1.shutdown.Store(true)
	if p1.cancel != nil {
		p1.cancel()
	}
	p1.wg.Wait()

	// p2 should become leader after TTL expires.
	require.Eventually(t, func() bool {
		return p2.isLeader.Load()
	}, leaderTTL+5*time.Second, 200*time.Millisecond, "p2 should become leader after failover")
}
