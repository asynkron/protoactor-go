package cluster

import (
	"testing"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// selfCachingFakeLookup is a stub IdentityLookup that also implements
// selfCachingLookup (SetsOwnPidCache returns true).
type selfCachingFakeLookup struct {
	fakeIdentityLookup
}

func (l *selfCachingFakeLookup) SetsOwnPidCache() bool { return true }

// nonSelfCachingFakeLookup is a stub IdentityLookup that does NOT implement
// the selfCachingLookup interface. It embeds fakeIdentityLookup but does not
// add SetsOwnPidCache, so the interface assertion in DefaultContext returns false.
type nonSelfCachingFakeLookup struct {
	fakeIdentityLookup
}

// TestDefaultContextSkipsCachingForSelfCachingLookup verifies:
//  1. When the IdentityLookup implements SetsOwnPidCache()=true, DefaultContext
//     must NOT call PidCache.Set on the PID returned by Get().
//  2. When the IdentityLookup does NOT implement the interface, DefaultContext
//     DOES call PidCache.Set as before.
func TestDefaultContextSkipsCachingForSelfCachingLookup(t *testing.T) {
	t.Parallel()

	staticPID := actor.NewPID("127.0.0.1:9999", "TestKind/testid")

	t.Run("self-caching lookup: PidCache.Set must be skipped", func(t *testing.T) {
		cp := newInmemoryProvider()
		scl := &selfCachingFakeLookup{}
		system := actor.NewActorSystem()
		cfg := Configure("test-selfcache", cp, scl, remote.Configure("127.0.0.1", 0))
		c := NewCluster(system, cfg)
		c.MemberList = NewMemberList(c)
		c.Remote = remote.NewRemote(system, cfg.RemoteConfig)
		// Wire the lookup so selfCachingSkip() can see it.
		c.IdentityLookup = scl
		// Inject the PID into the fake lookup so Get() returns it.
		scl.m.Store("testid", staticPID)

		ctx := newDefaultClusterContext(c)
		dcc := ctx.(*DefaultContext)

		pid, fromCache := dcc.getPid(t.Context(), "testid", "TestKind")
		require.NotNil(t, pid)
		assert.False(t, fromCache, "new resolution should not come from cache")

		// After getPid, the PidCache must NOT contain the PID.
		_, inCache := c.PidCache.Get("testid", "TestKind")
		assert.False(t, inCache,
			"self-caching lookup: DefaultContext must NOT add PID to PidCache")
	})

	t.Run("non-self-caching lookup: PidCache.Set must happen normally", func(t *testing.T) {
		cp := newInmemoryProvider()
		nscl := &nonSelfCachingFakeLookup{}
		system := actor.NewActorSystem()
		cfg := Configure("test-nonselfcache", cp, nscl, remote.Configure("127.0.0.1", 0))
		c := NewCluster(system, cfg)
		c.MemberList = NewMemberList(c)
		c.Remote = remote.NewRemote(system, cfg.RemoteConfig)
		c.IdentityLookup = nscl
		nscl.m.Store("testid", staticPID)

		ctx := newDefaultClusterContext(c)
		dcc := ctx.(*DefaultContext)

		pid, fromCache := dcc.getPid(t.Context(), "testid", "TestKind")
		require.NotNil(t, pid)
		assert.False(t, fromCache)

		// After getPid, the PidCache MUST contain the PID.
		cached, inCache := c.PidCache.Get("testid", "TestKind")
		assert.True(t, inCache,
			"non-self-caching lookup: DefaultContext must add PID to PidCache")
		assert.True(t, staticPID.Equal(cached))
	})
}
