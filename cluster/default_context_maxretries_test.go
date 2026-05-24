package cluster

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixedDeadIdentityLookup always returns the same (dead) PID from Get and treats
// RemovePid as a no-op, so the dead PID keeps being returned on every retry
// attempt. This makes dead-letter retry exhaustion deterministic.
type fixedDeadIdentityLookup struct {
	mu  sync.Mutex
	pid *actor.PID
}

func (l *fixedDeadIdentityLookup) Get(identity *ClusterIdentity) *actor.PID {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.pid
}

// RemovePid is intentionally a no-op so the dead PID is never cleared.
func (l *fixedDeadIdentityLookup) RemovePid(identity *ClusterIdentity, pid *actor.PID) {}

func (l *fixedDeadIdentityLookup) Setup(cluster *Cluster, kinds []string, isClient bool) {}

func (l *fixedDeadIdentityLookup) Shutdown() {}

func (l *fixedDeadIdentityLookup) Peek(identity *ClusterIdentity) (*PeekResult, error) {
	return &PeekResult{
		GrainInfo: &GrainInfo{Identity: identity.Identity, Kind: identity.Kind},
		Status:    PeekStatusNotFound,
	}, nil
}

// newClusterWithLookup builds a started cluster member using the supplied
// IdentityLookup, mirroring the removepid test harness.
func newClusterWithLookup(t *testing.T, name string, lookup IdentityLookup, opts ...ConfigOption) *Cluster {
	t.Helper()
	system := actor.NewActorSystem()
	cp := newInmemoryProvider()
	cfg := Configure(name, cp, lookup, remote.Configure("127.0.0.1", 0), opts...)
	c := New(system, cfg)
	c.Remote = remote.NewRemote(system, c.Config.RemoteConfig)
	require.NoError(t, c.StartMember())
	t.Cleanup(func() { c.Shutdown(true) })
	return c
}

// T1: dead-letter exhaustion. Drive c.Request so every attempt resolves a dead
// PID and gets a dead-letter error, exhausting the retry count. The returned
// error must match BOTH ErrMaxRetriesExceeded and the dead-letter sentinel.
func TestRequest_DeadLetterExhaustion_WrapsBothSentinels(t *testing.T) {
	lookup := &fixedDeadIdentityLookup{}
	c := newClusterWithLookup(t, "test-maxretries-deadletter", lookup,
		WithKinds(NewKind("echo", actor.PropsFromFunc(func(ctx actor.Context) {
			if _, ok := ctx.Message().(*PingMessage); ok {
				ctx.Respond(&PingMessage{})
			}
		}))),
	)

	// Spawn a real actor, then poison it so it is dead.
	kind := c.GetClusterKind("echo")
	require.NotNil(t, kind)
	ci := NewClusterIdentity("dead-grain", "echo")
	props := WithClusterIdentity(kind.Props, ci)
	deadPid, err := c.ActorSystem.Root.SpawnNamed(props, "echo-dead-grain")
	require.NoError(t, err)
	c.ActorSystem.Root.Poison(deadPid)
	time.Sleep(200 * time.Millisecond)

	lookup.mu.Lock()
	lookup.pid = deadPid
	lookup.mu.Unlock()

	// Generous overall timeout so the ~700ms backoff for RetryCount=3 does not
	// trip the ctx.Done() branch before the exhaustion branch.
	resp, err := c.Request("dead-grain", "echo", &PingMessage{},
		WithTimeout(5*time.Second),
		WithRetryCount(3),
	)
	assert.Nil(t, resp)
	require.Error(t, err)

	t.Logf("T1 returned err: %v", err)
	t.Logf("T1 errors.Is(err, actor.ErrDeadLetter)  = %v", errors.Is(err, actor.ErrDeadLetter))
	t.Logf("T1 errors.Is(err, remote.ErrDeadLetter) = %v", errors.Is(err, remote.ErrDeadLetter))

	assert.True(t, errors.Is(err, ErrMaxRetriesExceeded),
		"exhaustion error must match ErrMaxRetriesExceeded")
	// The dead-letter sentinel actually on this path must remain matchable.
	assert.True(t, errors.Is(err, actor.ErrDeadLetter),
		"exhaustion error must also preserve the dead-letter sentinel")
}

// T2: Request pid-nil exhaustion. Requesting an unregistered kind means getPid
// is always nil, so exhaustion happens with no inner sentinel.
func TestRequest_PidNilExhaustion_MatchesSentinel(t *testing.T) {
	c := newClusterForTest("test-maxretries-pidnil", newInmemoryProvider())

	resp, err := c.Request("name", "nonkind", &PingMessage{},
		WithTimeout(5*time.Second),
		WithRetryCount(3),
	)
	assert.Nil(t, resp)
	require.Error(t, err)
	t.Logf("T2 returned err: %v", err)

	assert.True(t, errors.Is(err, ErrMaxRetriesExceeded),
		"pid-nil exhaustion error must match ErrMaxRetriesExceeded")
	assert.ErrorContains(t, err, "max retries")
}

// T3: RequestFuture pid-nil exhaustion.
func TestRequestFuture_PidNilExhaustion_MatchesSentinel(t *testing.T) {
	c := newClusterForTest("test-maxretries-future-pidnil", newInmemoryProvider())

	f, err := c.RequestFuture("name", "nonkind", &PingMessage{},
		WithTimeout(5*time.Second),
		WithRetryCount(3),
	)
	assert.Nil(t, f)
	require.Error(t, err)
	t.Logf("T3 returned err: %v", err)

	assert.True(t, errors.Is(err, ErrMaxRetriesExceeded),
		"RequestFuture pid-nil exhaustion error must match ErrMaxRetriesExceeded")
}
