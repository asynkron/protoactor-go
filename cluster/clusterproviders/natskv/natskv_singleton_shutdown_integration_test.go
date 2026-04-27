//go:build integration

package natskv

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
)

// recordingRoleListener captures the ordered sequence of role transitions a
// provider notifies it about, so tests can assert what was observed before
// Shutdown returned.
type recordingRoleListener struct {
	mu    sync.Mutex
	roles []cluster.RoleType
}

func (r *recordingRoleListener) OnRoleChanged(rt cluster.RoleType) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.roles = append(r.roles, rt)
}

func (r *recordingRoleListener) snapshot() []cluster.RoleType {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]cluster.RoleType, len(r.roles))
	copy(out, r.roles)
	return out
}

func containsRole(roles []cluster.RoleType, want cluster.RoleType) bool {
	for _, r := range roles {
		if r == want {
			return true
		}
	}
	return false
}

// TestIntegration_Shutdown_SingleNode_StepsDownAndPoisonsSingleton asserts that
// a graceful Shutdown on the only (and therefore leader) member transitions the
// provider's role to Follower and stops actors spawned by registered
// SingletonSchedulers — and that both happen before Shutdown returns.
func TestIntegration_Shutdown_SingleNode_StepsDownAndPoisonsSingleton(t *testing.T) {
	natsURL := startNATSContainer(t)

	p, c := setupClusterFromURL(t, natsURL, "integ-shutdown-single",
		WithMemberTTL(2*time.Second),
		WithRefreshInterval(500*time.Millisecond),
		WithLeaderTTL(3*time.Second),
	)

	var (
		started atomic.Bool
		stopped atomic.Bool
	)
	scheduler := cluster.NewSingletonScheduler(c.ActorSystem.Root)
	scheduler.FromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *actor.Started:
			started.Store(true)
		case *actor.Stopping:
			stopped.Store(true)
		}
	})
	p.RegisterSingletonScheduler(scheduler)

	listener := &recordingRoleListener{}
	p.RegisterSingletonScheduler(listener)

	require.NoError(t, p.StartMember(c))

	require.Eventually(t, func() bool {
		return p.isLeader.Load() && started.Load()
	}, 10*time.Second, 100*time.Millisecond, "single member should become leader and spawn singleton")

	require.Eventually(t, func() bool {
		return containsRole(listener.snapshot(), cluster.RoleLeader)
	}, 5*time.Second, 50*time.Millisecond, "listener should have observed RoleLeader before shutdown")

	require.NoError(t, p.Shutdown(true))

	roles := listener.snapshot()
	assert.True(t, containsRole(roles, cluster.RoleFollower),
		"listener should observe RoleFollower before Shutdown returns; got sequence: %v", roles)
	assert.True(t, stopped.Load(),
		"singleton actor should have stopped before Shutdown returns")
}

// TestIntegration_Shutdown_MultiNode_LeaderStepsDownAndFollowerTakesOver covers
// the failover path: the leader gracefully shuts down and must (a) demote
// itself synchronously, stopping its singleton actors, and (b) leave a clean
// KV state so a surviving follower wins the next election and spawns its own
// singleton.
func TestIntegration_Shutdown_MultiNode_LeaderStepsDownAndFollowerTakesOver(t *testing.T) {
	natsURL := startNATSContainer(t)

	memberTTL := 3 * time.Second
	refresh := 500 * time.Millisecond
	leaderTTL := 4 * time.Second

	p1, c1 := setupClusterFromURL(t, natsURL, "integ-shutdown-multi",
		WithMemberTTL(memberTTL),
		WithRefreshInterval(refresh),
		WithLeaderTTL(leaderTTL),
	)

	var (
		started1 atomic.Bool
		stopped1 atomic.Bool
	)
	sched1 := cluster.NewSingletonScheduler(c1.ActorSystem.Root)
	sched1.FromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *actor.Started:
			started1.Store(true)
		case *actor.Stopping:
			stopped1.Store(true)
		}
	})
	p1.RegisterSingletonScheduler(sched1)
	listener1 := &recordingRoleListener{}
	p1.RegisterSingletonScheduler(listener1)

	require.NoError(t, p1.StartMember(c1))

	require.Eventually(t, func() bool {
		return p1.isLeader.Load() && started1.Load()
	}, 10*time.Second, 100*time.Millisecond, "p1 should become leader and spawn singleton")

	p2, c2 := setupClusterFromURL(t, natsURL, "integ-shutdown-multi",
		WithMemberTTL(memberTTL),
		WithRefreshInterval(refresh),
		WithLeaderTTL(leaderTTL),
	)
	t.Cleanup(func() { _ = p2.Shutdown(true) })

	var (
		started2 atomic.Bool
		stopped2 atomic.Bool
	)
	sched2 := cluster.NewSingletonScheduler(c2.ActorSystem.Root)
	sched2.FromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *actor.Started:
			started2.Store(true)
		case *actor.Stopping:
			stopped2.Store(true)
		}
	})
	p2.RegisterSingletonScheduler(sched2)

	require.NoError(t, p2.StartMember(c2))

	require.Eventually(t, func() bool {
		p1.membersMu.RLock()
		_, ok := p1.members[p2.self.ID]
		p1.membersMu.RUnlock()
		return ok
	}, 10*time.Second, 200*time.Millisecond, "p1 should discover p2 before shutdown")

	require.False(t, started2.Load(), "p2 should not have spawned the singleton while p1 leads")

	require.NoError(t, p1.Shutdown(true))

	roles1 := listener1.snapshot()
	assert.True(t, containsRole(roles1, cluster.RoleFollower),
		"p1's listener should observe RoleFollower before Shutdown returns; got sequence: %v", roles1)
	assert.True(t, stopped1.Load(),
		"p1's singleton actor should have stopped before Shutdown returns")

	require.Eventually(t, func() bool {
		return p2.isLeader.Load()
	}, 15*time.Second, 200*time.Millisecond, "p2 should become leader after p1 shuts down")
	require.Eventually(t, func() bool {
		return started2.Load()
	}, 5*time.Second, 100*time.Millisecond, "p2 should spawn its singleton after becoming leader")
	assert.False(t, stopped2.Load(), "p2's singleton should still be running")
}
