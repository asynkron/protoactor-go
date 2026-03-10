package cluster

import (
	"sync"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// enumerableFakeIdentityLookup extends fakeIdentityLookup with GrainEnumerator support.
type enumerableFakeIdentityLookup struct {
	mu      sync.RWMutex
	entries map[string]*GrainInfo // keyed by "kind/identity"
	cluster *Cluster
}

func newEnumerableFakeIdentityLookup() *enumerableFakeIdentityLookup {
	return &enumerableFakeIdentityLookup{
		entries: make(map[string]*GrainInfo),
	}
}

func (l *enumerableFakeIdentityLookup) Get(identity *ClusterIdentity) *actor.PID {
	l.mu.RLock()
	key := identity.AsKey()
	if info, ok := l.entries[key]; ok {
		l.mu.RUnlock()
		return info.PID
	}
	l.mu.RUnlock()

	// If the kind is registered, spawn the actor on first lookup.
	if l.cluster != nil {
		if clusterKind := l.cluster.GetClusterKind(identity.Kind); clusterKind != nil {
			props := WithClusterIdentity(clusterKind.Props, identity)
			pid, err := l.cluster.ActorSystem.Root.SpawnNamed(props, identity.Identity)
			if err == nil {
				clusterKind.Inc()
				l.mu.Lock()
				l.entries[key] = &GrainInfo{
					Identity:    identity.Identity,
					Kind:        identity.Kind,
					PID:         pid,
					MemberID:    l.cluster.ActorSystem.ID,
					ActivatedAt: time.Now(),
				}
				l.mu.Unlock()
				return pid
			}
		}
	}
	return nil
}

func (l *enumerableFakeIdentityLookup) RemovePid(identity *ClusterIdentity, pid *actor.PID) {
	l.mu.Lock()
	defer l.mu.Unlock()
	key := identity.AsKey()
	if info, ok := l.entries[key]; ok && info.PID.Equal(pid) {
		delete(l.entries, key)
	}
}

func (l *enumerableFakeIdentityLookup) Setup(cluster *Cluster, kinds []string, isClient bool) {
	l.cluster = cluster
}

func (l *enumerableFakeIdentityLookup) Shutdown() {}

// GrainEnumerator implementation.

func (l *enumerableFakeIdentityLookup) ListGrains() ([]*GrainInfo, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]*GrainInfo, 0, len(l.entries))
	for _, info := range l.entries {
		cp := *info
		result = append(result, &cp)
	}
	return result, nil
}

func (l *enumerableFakeIdentityLookup) ListGrainsByKind(kind string) ([]*GrainInfo, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var result []*GrainInfo
	for _, info := range l.entries {
		if info.Kind == kind {
			cp := *info
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (l *enumerableFakeIdentityLookup) ListGrainsByMember(memberID string) ([]*GrainInfo, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var result []*GrainInfo
	for _, info := range l.entries {
		if info.MemberID == memberID {
			cp := *info
			result = append(result, &cp)
		}
	}
	return result, nil
}

// Compile-time check that enumerableFakeIdentityLookup implements both interfaces.
var _ IdentityLookup = (*enumerableFakeIdentityLookup)(nil)
var _ GrainEnumerator = (*enumerableFakeIdentityLookup)(nil)

func newClusterForIntegrationTest(name string, opts ...ConfigOption) *Cluster {
	system := actor.NewActorSystem()
	lookup := newEnumerableFakeIdentityLookup()
	cfg := Configure(name, newInmemoryProvider(), lookup, remote.Configure("127.0.0.1", 0), opts...)
	c := NewCluster(system, cfg)
	return c
}

func TestGrainRegistry_Integration_Enumeration(t *testing.T) {
	kindA := NewKind("kind-a", actor.PropsFromFunc(func(ctx actor.Context) {}))
	kindB := NewKind("kind-b", actor.PropsFromFunc(func(ctx actor.Context) {}))

	c := newClusterForIntegrationTest("test-registry-enum", WithKinds(kindA, kindB))
	err := c.StartMember()
	require.NoError(t, err)
	defer c.Shutdown(true)

	// Activate grains via Get.
	pid1 := c.Get("grain-1", "kind-a")
	pid2 := c.Get("grain-2", "kind-a")
	pid3 := c.Get("grain-3", "kind-b")
	require.NotNil(t, pid1)
	require.NotNil(t, pid2)
	require.NotNil(t, pid3)

	// Allow activations to settle.
	time.Sleep(200 * time.Millisecond)

	reg := c.GrainRegistry()

	// Count should reflect 3 activated grains.
	assert.Equal(t, 3, reg.Count())

	// CountByKind should break down correctly.
	byKind := reg.CountByKind()
	assert.Equal(t, 2, byKind["kind-a"])
	assert.Equal(t, 1, byKind["kind-b"])

	// All should return 3 grain infos.
	all, err := reg.All()
	require.NoError(t, err)
	assert.Len(t, all, 3)

	// ByKind should filter correctly.
	kindAGrains, err := reg.ByKind("kind-a")
	require.NoError(t, err)
	assert.Len(t, kindAGrains, 2)
	for _, g := range kindAGrains {
		assert.Equal(t, "kind-a", g.Kind)
	}

	kindBGrains, err := reg.ByKind("kind-b")
	require.NoError(t, err)
	assert.Len(t, kindBGrains, 1)
	assert.Equal(t, "grain-3", kindBGrains[0].Identity)

	// Get should return a specific grain.
	info, found, err := reg.Get("grain-3", "kind-b")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "grain-3", info.Identity)
	assert.Equal(t, "kind-b", info.Kind)
	assert.NotNil(t, info.PID)

	// Get non-existent grain should return not found.
	_, found, err = reg.Get("nonexistent", "kind-a")
	require.NoError(t, err)
	assert.False(t, found)

	// ByMember should return all grains for the local member.
	memberGrains, err := reg.ByMember(c.ActorSystem.ID)
	require.NoError(t, err)
	assert.Len(t, memberGrains, 3)

	// ByMember for unknown member should return empty.
	unknownGrains, err := reg.ByMember("unknown-member")
	require.NoError(t, err)
	assert.Empty(t, unknownGrains)
}

func TestGrainRegistry_Integration_WithGrainMetrics(t *testing.T) {
	echoKind := NewKind("echo", actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *actor.Started:
		case *actor.Stopping:
		case *actor.Stopped:
		case *ClusterInit:
		default:
			if ctx.Sender() != nil {
				ctx.Respond(ctx.Message())
			}
		}
	}))

	c := newClusterForIntegrationTest("test-registry-metrics",
		WithKinds(echoKind),
		WithGrainMetrics(),
	)
	err := c.StartMember()
	require.NoError(t, err)
	defer c.Shutdown(true)

	// Activate and send messages via Request.
	resp, err := c.Request("my-echo", "echo", "hello")
	require.NoError(t, err)
	assert.Equal(t, "hello", resp)

	resp, err = c.Request("my-echo", "echo", "world")
	require.NoError(t, err)
	assert.Equal(t, "world", resp)

	// Allow metrics to settle.
	time.Sleep(200 * time.Millisecond)

	reg := c.GrainRegistry()
	info, found, err := reg.Get("my-echo", "echo")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "my-echo", info.Identity)
	assert.Equal(t, "echo", info.Kind)

	// With grain metrics enabled, message count should be > 0.
	// The middleware records metrics for every message including Started and ClusterInit.
	assert.Greater(t, info.MessageCount, int64(0))
	assert.False(t, info.LastMessageAt.IsZero())
}

func TestGrainRegistry_Integration_NonEnumerableLookup(t *testing.T) {
	// Use a lookup that does NOT implement GrainEnumerator.
	c := newClusterForTest("test-registry-noenum", newInmemoryProvider())
	c.grainReg = &GrainRegistry{cluster: c}
	c.IdentityLookup = &mockIdentityLookup{}

	// Count and CountByKind should still work (they don't need enumeration).
	assert.Equal(t, 0, c.GrainRegistry().Count())
	assert.NotNil(t, c.GrainRegistry().CountByKind())

	// Enumeration methods should return ErrEnumerationNotSupported.
	_, err := c.GrainRegistry().All()
	assert.ErrorIs(t, err, ErrEnumerationNotSupported)

	_, err = c.GrainRegistry().ByKind("any")
	assert.ErrorIs(t, err, ErrEnumerationNotSupported)

	_, err = c.GrainRegistry().ByMember("any")
	assert.ErrorIs(t, err, ErrEnumerationNotSupported)

	_, _, err = c.GrainRegistry().Get("id", "kind")
	assert.ErrorIs(t, err, ErrEnumerationNotSupported)
}
