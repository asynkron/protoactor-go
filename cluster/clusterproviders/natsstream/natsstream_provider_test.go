package natsstream

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time check that Provider implements KindUpdater.
var _ cluster.KindUpdater = (*Provider)(nil)

func TestNew_ReturnsProvider(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc, _ := connectNATS(t, srv)

	p, err := New(nc)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestNewFromJetStream_ReturnsProvider(t *testing.T) {
	srv := startEmbeddedNATS(t)
	_, js := connectNATS(t, srv)

	p, err := NewFromJetStream(js)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestNew_WithOptions(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc, _ := connectNATS(t, srv)

	p, err := New(nc,
		WithStreamName("MY_STREAM"),
		WithSubjectPrefix("myprefix"),
	)
	require.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, "MY_STREAM", p.config.StreamName)
	assert.Equal(t, "myprefix", p.config.SubjectPrefix)
}

func TestProvider_GetHealthStatus_NilByDefault(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc, _ := connectNATS(t, srv)

	p, err := New(nc)
	require.NoError(t, err)

	assert.NoError(t, p.GetHealthStatus())
}

func TestProvider_IdentityLookup_NotNil(t *testing.T) {
	srv := startEmbeddedNATS(t)
	nc, _ := connectNATS(t, srv)

	p, err := New(nc)
	require.NoError(t, err)

	assert.NotNil(t, p.IdentityLookup())
}

// mockRoleListener records role changes via a callback.
type mockRoleListener struct {
	callback func(cluster.RoleType)
}

func (m *mockRoleListener) OnRoleChanged(r cluster.RoleType) { m.callback(r) }

func TestStartMember_PublishesHeartbeat(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-heartbeat")

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	// Verify the heartbeat message exists on the stream.
	require.NotNil(t, p.self)
	subject := p.prefix + ".members." + p.self.ID

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msg, err := p.stream.GetLastMsgForSubject(ctx, subject)
	require.NoError(t, err, "heartbeat message should exist on stream")

	var node Node
	require.NoError(t, json.Unmarshal(msg.Data, &node))
	assert.Equal(t, p.self.ID, node.ID)
	assert.True(t, node.Alive)
}

func TestStartMember_DiscoversExistingMembers(t *testing.T) {
	srv := startEmbeddedNATS(t)

	// Start first member.
	p1, c1 := setupCluster(t, srv, "test-discover")
	err := p1.StartMember(c1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p1.Shutdown(true) })

	time.Sleep(500 * time.Millisecond)

	// Start second member.
	p2, c2 := setupCluster(t, srv, "test-discover")
	err = p2.StartMember(c2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	// Second should discover first.
	require.Eventually(t, func() bool {
		p2.membersMu.RLock()
		defer p2.membersMu.RUnlock()
		_, found := p2.members[p1.self.ID]
		return found
	}, 10*time.Second, 200*time.Millisecond, "second provider should discover the first member")
}

func TestStartClient_WatchOnly(t *testing.T) {
	srv := startEmbeddedNATS(t)

	// Start a member first.
	pMember, cMember := setupCluster(t, srv, "test-client")
	err := pMember.StartMember(cMember)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pMember.Shutdown(true) })

	time.Sleep(500 * time.Millisecond)

	// Start a client.
	pClient, cClient := setupCluster(t, srv, "test-client")
	err = pClient.StartClient(cClient)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pClient.Shutdown(true) })

	// Client should NOT have published a heartbeat.
	subject := pClient.prefix + ".members." + pClient.self.ID
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = pClient.stream.GetLastMsgForSubject(ctx, subject)
	assert.Error(t, err, "client should NOT publish heartbeat")

	// Client should discover the member.
	require.Eventually(t, func() bool {
		pClient.membersMu.RLock()
		defer pClient.membersMu.RUnlock()
		_, found := pClient.members[pMember.self.ID]
		return found
	}, 10*time.Second, 200*time.Millisecond, "client should discover the member")
}

func TestShutdown_PublishesLeave(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-leave")

	err := p.StartMember(c)
	require.NoError(t, err)

	require.NotNil(t, p.self)

	// Save self ID before shutdown since we need it after.
	selfID := p.self.ID

	// Shutdown gracefully.
	err = p.Shutdown(true)
	require.NoError(t, err)

	// Verify leave message was published.
	subject := p.prefix + ".leave." + selfID
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	msg, err := p.stream.GetLastMsgForSubject(ctx, subject)
	require.NoError(t, err, "leave event should be published on shutdown")
	assert.Contains(t, string(msg.Data), selfID)
}

func TestShutdown_Idempotent(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-shutdown-idem")

	err := p.StartMember(c)
	require.NoError(t, err)

	err = p.Shutdown(true)
	assert.NoError(t, err)

	err = p.Shutdown(true)
	assert.NoError(t, err, "second shutdown should not fail or panic")
}

func TestMemberCrash_TimeoutDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timeout detection test in short mode")
	}

	srv := startEmbeddedNATS(t)

	// Use short timeouts for fast tests.
	// HeartbeatTTL must be >= 1s due to NATS subject delete marker TTL minimum.
	opts := []Option{
		WithHeartbeatInterval(200 * time.Millisecond),
		WithHeartbeatTTL(1 * time.Second),
		WithMemberTimeout(1500 * time.Millisecond),
		WithCheckInterval(200 * time.Millisecond),
	}

	p1, c1 := setupCluster(t, srv, "test-crash", opts...)
	err := p1.StartMember(c1)
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	p2, c2 := setupCluster(t, srv, "test-crash", opts...)
	err = p2.StartMember(c2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	// Verify p2 sees p1.
	require.Eventually(t, func() bool {
		p2.membersMu.RLock()
		defer p2.membersMu.RUnlock()
		_, found := p2.members[p1.self.ID]
		return found
	}, 5*time.Second, 100*time.Millisecond, "p2 should see p1")

	// "Crash" p1: cancel context and set shutdown to stop heartbeats.
	p1.shutdown.Store(true)
	if p1.cancel != nil {
		p1.cancel()
	}
	p1.wg.Wait()

	// After timeout, p2 should remove p1.
	require.Eventually(t, func() bool {
		p2.membersMu.RLock()
		defer p2.membersMu.RUnlock()
		_, found := p2.members[p1.self.ID]
		return !found
	}, 5*time.Second, 100*time.Millisecond, "p2 should detect p1 crash via timeout")
}

func TestLeaderElection_FirstWins(t *testing.T) {
	srv := startEmbeddedNATS(t)

	p1, c1 := setupCluster(t, srv, "test-leader")
	err := p1.StartMember(c1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p1.Shutdown(true) })

	time.Sleep(500 * time.Millisecond)

	p2, c2 := setupCluster(t, srv, "test-leader")
	err = p2.StartMember(c2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	time.Sleep(500 * time.Millisecond)

	p1IsLeader := p1.isLeader.Load()
	p2IsLeader := p2.isLeader.Load()

	assert.True(t, p1IsLeader || p2IsLeader, "at least one should be leader")
	assert.False(t, p1IsLeader && p2IsLeader, "only one should be leader at a time")
	assert.True(t, p1IsLeader, "first member should win leader election")
}

func TestLeaderElection_Failover(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping leader failover test in short mode")
	}

	srv := startEmbeddedNATS(t)

	leaderTTL := 3 * time.Second
	// HeartbeatTTL must be >= 1s due to NATS subject delete marker TTL minimum.
	opts := []Option{
		WithLeaderTTL(leaderTTL),
		WithHeartbeatInterval(200 * time.Millisecond),
		WithHeartbeatTTL(1 * time.Second),
		WithMemberTimeout(2 * time.Second),
		WithCheckInterval(200 * time.Millisecond),
	}

	p1, c1 := setupCluster(t, srv, "test-failover", opts...)
	err := p1.StartMember(c1)
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	p2, c2 := setupCluster(t, srv, "test-failover", opts...)
	err = p2.StartMember(c2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	// Wait for p1 to become leader.
	require.Eventually(t, func() bool {
		return p1.isLeader.Load()
	}, 5*time.Second, 100*time.Millisecond, "p1 should be leader")

	// Crash p1.
	p1.shutdown.Store(true)
	if p1.cancel != nil {
		p1.cancel()
	}
	p1.wg.Wait()

	// Wait for p2 to become leader after TTL expires.
	require.Eventually(t, func() bool {
		return p2.isLeader.Load()
	}, leaderTTL+5*time.Second, 200*time.Millisecond, "p2 should become leader after failover")
}

func TestRoleChangedListener_Called(t *testing.T) {
	srv := startEmbeddedNATS(t)

	var gotRole atomic.Int32
	gotRole.Store(-1)

	listener := &mockRoleListener{
		callback: func(r cluster.RoleType) {
			gotRole.Store(int32(r))
		},
	}

	p, c := setupCluster(t, srv, "test-rolechange", WithRoleChangedListener(listener))
	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	require.Eventually(t, func() bool {
		return gotRole.Load() == int32(cluster.RoleLeader)
	}, 5*time.Second, 100*time.Millisecond, "role changed listener should be called with Leader")
}

func TestSingletonScheduler_SpawnOnLeader(t *testing.T) {
	srv := startEmbeddedNATS(t)

	var spawned atomic.Bool

	p, c := setupCluster(t, srv, "test-singleton")

	scheduler := cluster.NewSingletonScheduler(c.ActorSystem.Root)
	scheduler.FromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *actor.Started:
			spawned.Store(true)
		}
	})
	p.RegisterSingletonScheduler(scheduler)

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	require.Eventually(t, func() bool {
		return spawned.Load()
	}, 5*time.Second, 100*time.Millisecond, "singleton actor should be spawned on leader")
}

func TestProvider_UpdateKinds(t *testing.T) {
	srv := startEmbeddedNATS(t)
	p, c := setupCluster(t, srv, "test-updatekinds")

	err := p.StartMember(c)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(true) })

	originalKinds := p.self.Kinds

	newKinds := append([]string{}, originalKinds...)
	newKinds = append(newKinds, "dynamicKind")
	err = p.UpdateKinds(newKinds)
	require.NoError(t, err)

	// Verify self.Kinds is updated
	p.membersMu.RLock()
	assert.ElementsMatch(t, newKinds, p.self.Kinds)
	p.membersMu.RUnlock()
}
