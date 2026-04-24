package cluster

import (
	"sync"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// removePidCall records a single invocation of RemovePid.
type removePidCall struct {
	Identity *ClusterIdentity
	Pid      *actor.PID
}

// trackingIdentityLookup wraps fakeIdentityLookup and records RemovePid calls.
type trackingIdentityLookup struct {
	fakeIdentityLookup
	mu             sync.Mutex
	removePidCalls []removePidCall
}

func (l *trackingIdentityLookup) RemovePid(identity *ClusterIdentity, pid *actor.PID) {
	l.mu.Lock()
	l.removePidCalls = append(l.removePidCalls, removePidCall{Identity: identity, Pid: pid})
	l.mu.Unlock()
	l.fakeIdentityLookup.RemovePid(identity, pid)
}

func (l *trackingIdentityLookup) getRemovePidCalls() []removePidCall {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]removePidCall, len(l.removePidCalls))
	copy(out, l.removePidCalls)
	return out
}

// TestDefaultContext_Request_CallsRemovePid_OnDeadLetter verifies that
// DefaultContext.Request() calls IdentityLookup.RemovePid() when a request
// to a grain returns a dead letter error. Without this call, external identity
// stores (NATS KV, NATS Stream, Redis, Postgres) retain stale activation
// records that block reactivation on surviving nodes.
func TestDefaultContext_Request_CallsRemovePid_OnDeadLetter(t *testing.T) {
	system := actor.NewActorSystem()

	lookup := &trackingIdentityLookup{}
	cp := newInmemoryProvider()

	cfg := Configure("test-removepid", cp, lookup,
		remote.Configure("127.0.0.1", 0),
		WithKinds(NewKind("echo", actor.PropsFromFunc(func(ctx actor.Context) {
			if _, ok := ctx.Message().(*PingMessage); ok {
				ctx.Respond(&PingMessage{})
			}
		}))),
	)

	c := New(system, cfg)
	c.Remote = remote.NewRemote(system, c.Config.RemoteConfig)

	err := c.StartMember()
	require.NoError(t, err)
	t.Cleanup(func() { c.Shutdown(true) })

	// Spawn a real actor and verify normal operation.
	kind := c.GetClusterKind("echo")
	require.NotNil(t, kind)
	ci := NewClusterIdentity("test-grain-1", "echo")
	props := WithClusterIdentity(kind.Props, ci)
	pid, err := system.Root.SpawnNamed(props, "echo-test-grain-1")
	require.NoError(t, err)

	// Register the PID in the identity lookup.
	lookup.m.Store("test-grain-1", pid)

	// Verify the grain works.
	resp, err := c.Request("test-grain-1", "echo", &PingMessage{}, WithTimeout(2*time.Second))
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Now poison the actor to make it dead.
	system.Root.Poison(pid)
	time.Sleep(200 * time.Millisecond) // wait for actor to stop

	// The identity lookup still returns the dead PID (simulating stale activation).
	// Request should fail, but it must call RemovePid so external stores can clean up.
	resp, err = c.Request("test-grain-1", "echo", &PingMessage{},
		WithTimeout(2*time.Second),
		WithRetryCount(3),
	)
	// Request will fail (dead actor), that's expected.
	_ = resp
	_ = err

	// The critical assertion: RemovePid must have been called with the
	// correct identity and the stale PID.
	calls := lookup.getRemovePidCalls()
	assert.NotEmpty(t, calls,
		"DefaultContext.Request() must call IdentityLookup.RemovePid() "+
			"when it encounters a dead letter, so that external identity "+
			"stores can clean up stale activation records")

	if len(calls) > 0 {
		assert.Equal(t, "test-grain-1", calls[0].Identity.Identity)
		assert.Equal(t, "echo", calls[0].Identity.Kind)
	}
}

// TestDefaultContext_RequestFuture_CallsRemovePid_OnDeadLetter verifies the
// same behavior for RequestFuture (the non-blocking variant).
// Note: RequestFuture returns after getting a PID, so RemovePid would be
// called if the returned future detects a dead letter. This test verifies
// the getPid path calls RemovePid when the cached PID is stale.
func TestDefaultContext_Request_RemovePid_ClearsStaleActivation(t *testing.T) {
	system := actor.NewActorSystem()

	lookup := &trackingIdentityLookup{}
	cp := newInmemoryProvider()

	cfg := Configure("test-removepid-clear", cp, lookup,
		remote.Configure("127.0.0.1", 0),
		WithKinds(NewKind("echo", actor.PropsFromFunc(func(ctx actor.Context) {
			if _, ok := ctx.Message().(*PingMessage); ok {
				ctx.Respond(&PingMessage{})
			}
		}))),
	)

	c := New(system, cfg)
	c.Remote = remote.NewRemote(system, c.Config.RemoteConfig)

	err := c.StartMember()
	require.NoError(t, err)
	t.Cleanup(func() { c.Shutdown(true) })

	kind := c.GetClusterKind("echo")
	require.NotNil(t, kind)

	// Spawn and immediately poison to create a dead PID.
	ci := NewClusterIdentity("grain-stale", "echo")
	props := WithClusterIdentity(kind.Props, ci)
	deadPid, err := system.Root.SpawnNamed(props, "echo-grain-stale")
	require.NoError(t, err)
	system.Root.Poison(deadPid)
	time.Sleep(200 * time.Millisecond)

	// Identity lookup returns the dead PID on first call.
	// After RemovePid is called, it should stop returning it.
	lookup.m.Store("grain-stale", deadPid)

	// First request hits the dead PID, gets a dead letter, and retries.
	// RemovePid clears the stale entry, the retry spawns a new actor,
	// and the request succeeds.
	resp, err := c.Request("grain-stale", "echo", &PingMessage{},
		WithTimeout(5*time.Second),
		WithRetryCount(5),
	)

	// Verify that RemovePid was called to clear the stale entry.
	calls := lookup.getRemovePidCalls()
	require.NotEmpty(t, calls, "RemovePid must be called to clear stale activation")

	// The request should ultimately succeed because RemovePid cleared
	// the stale entry, allowing the retry to spawn a fresh actor.
	require.NoError(t, err,
		"request should succeed after RemovePid clears the stale activation "+
			"and the retry spawns a new actor")
	require.NotNil(t, resp, "response must not be nil after recovery")
}

// TestDefaultContext_Request_Timeout_DoesNotCallRemovePid verifies that
// DefaultContext.Request() does NOT call IdentityLookup.RemovePid() when a
// request times out. Timeouts mean the actor may still be alive (just slow),
// so deleting its identity record would create an orphaned actor with no
// KV/stream record, breaking future lookups.
func TestDefaultContext_Request_Timeout_DoesNotCallRemovePid(t *testing.T) {
	system := actor.NewActorSystem()

	lookup := &trackingIdentityLookup{}
	cp := newInmemoryProvider()

	cfg := Configure("test-timeout-no-removepid", cp, lookup,
		remote.Configure("127.0.0.1", 0),
		WithKinds(NewKind("slow", actor.PropsFromFunc(func(ctx actor.Context) {
			if _, ok := ctx.Message().(*PingMessage); ok {
				// Deliberately slow — will exceed the request timeout.
				time.Sleep(1 * time.Second)
				ctx.Respond(&PingMessage{})
			}
		}))),
	)

	c := New(system, cfg)
	c.Remote = remote.NewRemote(system, c.Config.RemoteConfig)

	err := c.StartMember()
	require.NoError(t, err)
	t.Cleanup(func() { c.Shutdown(true) })

	// Spawn a slow actor and register it in the identity lookup.
	kind := c.GetClusterKind("slow")
	require.NotNil(t, kind)
	ci := NewClusterIdentity("slow-grain-1", "slow")
	props := WithClusterIdentity(kind.Props, ci)
	pid, err := system.Root.SpawnNamed(props, "slow-grain-1")
	require.NoError(t, err)
	t.Cleanup(func() { system.Root.Poison(pid) })

	lookup.m.Store("slow-grain-1", pid)

	// Request with a short timeout — will time out because actor sleeps 1s.
	_, _ = c.Request("slow-grain-1", "slow", &PingMessage{},
		WithTimeout(200*time.Millisecond),
		WithRetryCount(1),
	)

	// RemovePid must NOT have been called — the actor is alive, just slow.
	calls := lookup.getRemovePidCalls()
	assert.Empty(t, calls,
		"RemovePid must NOT be called on timeout — the actor may still "+
			"be alive. Only dead letters should trigger RemovePid.")
}

// PingMessage is a simple test message for request/response.
type PingMessage struct{}
