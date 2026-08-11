package cluster

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pidAlive reports whether a locally-spawned grain PID is still registered
// (i.e., not yet poisoned/terminated).
func pidAlive(c *Cluster, pid *actor.PID) bool {
	_, ok := c.ActorSystem.ProcessRegistry.GetLocal(pid.Id)
	return ok
}

// spawnPlacement is a small helper that spawns a placement actor with the
// given config and returns its PID plus a cleanup.
func spawnPlacement(t *testing.T, c *Cluster, name string, cfg PlacementConfig) *actor.PID {
	t.Helper()
	props := NewPlacementActorProps(c, cfg)
	pid, err := c.ActorSystem.Root.SpawnNamed(props, name)
	require.NoError(t, err)
	t.Cleanup(func() { c.ActorSystem.Root.Poison(pid) })
	return pid
}

// remoteActivation issues a remote-initiated ActivationRequest (empty
// RequestId) to the placement actor and returns the spawned PID.
func remoteActivation(t *testing.T, c *Cluster, placementPID *actor.PID, ci *ClusterIdentity) *actor.PID {
	t.Helper()
	req := &ActivationRequest{ClusterIdentity: ci} // empty RequestId => remote-initiated
	res, err := c.ActorSystem.Root.RequestFuture(placementPID, req, 5*time.Second).Result()
	require.NoError(t, err)
	resp := res.(*ActivationResponse)
	require.False(t, resp.Failed)
	require.NotNil(t, resp.Pid)
	return resp.Pid
}

// TestSelfCheckDecisionTable exercises the four RecordCheck outcomes.
func TestSelfCheckDecisionTable(t *testing.T) {
	cases := []struct {
		name       string
		result     RecordCheck
		wantAlive  bool
		wantCleaup bool
	}{
		{"own disarms", RecordOwn, true, false},
		{"foreign self-poisons", RecordForeign, false, true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClusterWithKind(t, "skind", echoProps())

			var cleanupCalled atomic.Int32
			cfg := PlacementConfig{
				// PersistActivation must be present so the empty-RequestId path
				// runs through persistAndRespond and arms the self-check. It
				// no-ops for remote-initiated requests.
				PersistActivation: func(ctx context.Context, ci *ClusterIdentity, pid *actor.PID, requestID string) error {
					return nil
				},
				CheckActivationRecord: func(ctx context.Context, ci *ClusterIdentity, selfPID *actor.PID) RecordCheck {
					return tc.result
				},
				CleanupOwnRecord: func(ctx context.Context, ci *ClusterIdentity, selfPID *actor.PID) {
					cleanupCalled.Add(1)
				},
				SelfCheckDelay: 50 * time.Millisecond,
			}
			pp := spawnPlacement(t, c, "$sc-"+tc.name, cfg)

			pid := remoteActivation(t, c, pp, &ClusterIdentity{Kind: "skind", Identity: "g1"})
			require.True(t, pidAlive(c, pid), "grain should be alive right after spawn")

			// Wait past the self-check delay.
			assert.Eventually(t, func() bool {
				return pidAlive(c, pid) == tc.wantAlive
			}, 3*time.Second, 25*time.Millisecond,
				"grain liveness should settle to %v", tc.wantAlive)

			if tc.wantCleaup {
				assert.Eventually(t, func() bool { return cleanupCalled.Load() > 0 },
					2*time.Second, 25*time.Millisecond, "CleanupOwnRecord should be called")
			} else {
				// Give the (disarmed) case a moment; cleanup must not fire.
				time.Sleep(200 * time.Millisecond)
				assert.Equal(t, int32(0), cleanupCalled.Load(),
					"CleanupOwnRecord must not be called on RecordOwn")
			}
		})
	}
}

// TestSelfCheckLockOnlyDefers verifies that a lock-only observation never
// poisons; the grain stays alive across re-arms, and once the record flips to
// RecordOwn the self-check disarms (grain still alive).
func TestSelfCheckLockOnlyDefers(t *testing.T) {
	c := newTestClusterWithKind(t, "skind", echoProps())

	var phase atomic.Int32 // 0 => lock-only, 1 => own
	var checks atomic.Int32
	cfg := PlacementConfig{
		PersistActivation: func(ctx context.Context, ci *ClusterIdentity, pid *actor.PID, requestID string) error {
			return nil
		},
		CheckActivationRecord: func(ctx context.Context, ci *ClusterIdentity, selfPID *actor.PID) RecordCheck {
			checks.Add(1)
			if phase.Load() == 0 {
				return RecordLockOnly
			}
			return RecordOwn
		},
		SelfCheckDelay: 40 * time.Millisecond,
	}
	pp := spawnPlacement(t, c, "$sc-lockonly", cfg)
	pid := remoteActivation(t, c, pp, &ClusterIdentity{Kind: "skind", Identity: "g-lock"})

	// Let it re-arm on lock-only at least twice; must stay alive throughout.
	require.Eventually(t, func() bool { return checks.Load() >= 2 },
		2*time.Second, 20*time.Millisecond, "self-check should re-arm on lock-only")
	require.True(t, pidAlive(c, pid), "grain must stay alive while resolution is in flight")

	// Flip to own; the next check should disarm and leave the grain alive.
	phase.Store(1)
	time.Sleep(300 * time.Millisecond)
	assert.True(t, pidAlive(c, pid), "grain must survive once record becomes own")
}

// TestSelfCheckAbsentBeyondHardReapPoisons verifies that a persistently-absent
// record beyond SelfCheckHardReapAge causes the orphan grain to be poisoned.
func TestSelfCheckAbsentBeyondHardReapPoisons(t *testing.T) {
	c := newTestClusterWithKind(t, "skind", echoProps())

	cfg := PlacementConfig{
		PersistActivation: func(ctx context.Context, ci *ClusterIdentity, pid *actor.PID, requestID string) error {
			return nil
		},
		CheckActivationRecord: func(ctx context.Context, ci *ClusterIdentity, selfPID *actor.PID) RecordCheck {
			return RecordAbsent
		},
		SelfCheckDelay:       30 * time.Millisecond,
		SelfCheckHardReapAge: 60 * time.Millisecond,
	}
	pp := spawnPlacement(t, c, "$sc-absent", cfg)
	pid := remoteActivation(t, c, pp, &ClusterIdentity{Kind: "skind", Identity: "g-absent"})

	assert.Eventually(t, func() bool { return !pidAlive(c, pid) },
		3*time.Second, 25*time.Millisecond,
		"absent record beyond hard-reap age must poison the orphan grain")
}

// TestRemoveAndPoisonRemovesBeforePoison verifies that a RemoveAndPoisonRequest
// removes the grain from tracking, poisons it, and acks.
func TestRemoveAndPoisonRemovesBeforePoison(t *testing.T) {
	c := newTestClusterWithKind(t, "skind", echoProps())

	cfg := PlacementConfig{
		PersistActivation: func(ctx context.Context, ci *ClusterIdentity, pid *actor.PID, requestID string) error {
			return nil
		},
		// Self-check enabled but with a long delay so it does not interfere.
		CheckActivationRecord: func(ctx context.Context, ci *ClusterIdentity, selfPID *actor.PID) RecordCheck {
			return RecordOwn
		},
		SelfCheckDelay: time.Hour,
	}
	pp := spawnPlacement(t, c, "$rap", cfg)
	pid := remoteActivation(t, c, pp, &ClusterIdentity{Kind: "skind", Identity: "g-rap"})
	require.True(t, pidAlive(c, pid))

	res, err := c.ActorSystem.Root.RequestFuture(pp, &RemoveAndPoisonRequest{PID: pid}, 5*time.Second).Result()
	require.NoError(t, err)
	_, ok := res.(*RemoveAndPoisonAck)
	require.True(t, ok, "expected RemoveAndPoisonAck, got %T", res)

	assert.Eventually(t, func() bool { return !pidAlive(c, pid) },
		2*time.Second, 25*time.Millisecond, "grain must be poisoned after removeAndPoison")

	// A duplicate request for an untracked PID still acks.
	res2, err := c.ActorSystem.Root.RequestFuture(pp, &RemoveAndPoisonRequest{PID: pid}, 5*time.Second).Result()
	require.NoError(t, err)
	_, ok = res2.(*RemoveAndPoisonAck)
	require.True(t, ok, "untracked removeAndPoison should still ack")
}

// TestStragglerPersistSkippedAfterAbort verifies that a slow persist goroutine,
// once its grain is removed-and-poisoned mid-retry via RemoveAndPoisonRequest,
// skips its next KV attempt (the abort flag keyed by PID) rather than recording
// the actor being torn down.
func TestStragglerPersistSkippedAfterAbort(t *testing.T) {
	c := newTestClusterWithKind(t, "skind", echoProps())

	var attempts atomic.Int32
	var persistedAfterAbort atomic.Int32
	pidCh := make(chan *actor.PID, 1)
	firstAttemptWaits := make(chan struct{})
	var once sync.Once
	aborted := make(chan struct{})

	cfg := PlacementConfig{
		PersistActivation: func(ctx context.Context, ci *ClusterIdentity, pid *actor.PID, requestID string) error {
			n := attempts.Add(1)
			if n == 1 {
				// Publish the PID so the test can target abort, then block
				// until the test has sent removeAndPoison.
				once.Do(func() { pidCh <- pid })
				<-firstAttemptWaits
				return context.DeadlineExceeded // trigger a retry
			}
			// Any attempt after the abort should have been skipped by the
			// abort guard; if we get here the guard failed.
			select {
			case <-aborted:
				persistedAfterAbort.Add(1)
			default:
			}
			return context.DeadlineExceeded
		},
		PersistenceRetries:    5,
		PersistenceRetryDelay: 30 * time.Millisecond,
		CheckActivationRecord: func(ctx context.Context, ci *ClusterIdentity, selfPID *actor.PID) RecordCheck {
			return RecordOwn
		},
		SelfCheckDelay: time.Hour,
	}
	pp := spawnPlacement(t, c, "$straggler", cfg)

	ci := &ClusterIdentity{Kind: "skind", Identity: "g-straggler"}
	fut := c.ActorSystem.Root.RequestFuture(pp,
		&ActivationRequest{ClusterIdentity: ci, RequestId: "lock-1"}, 10*time.Second)

	pid := <-pidCh
	// Abort the in-flight persist by PID via removeAndPoison. The grain is not
	// yet tracked (persist hasn't completed), so onRemoveAndPoison acks
	// immediately, but abortPersist(pid) still sets the abort flag.
	ackFut := c.ActorSystem.Root.RequestFuture(pp, &RemoveAndPoisonRequest{PID: pid}, 5*time.Second)
	_, err := ackFut.Result()
	require.NoError(t, err)
	close(aborted)
	// Release the first attempt so the goroutine proceeds to its retry, which
	// must observe the abort flag and skip the KV update.
	close(firstAttemptWaits)

	res, err := fut.Result()
	require.NoError(t, err)
	resp := res.(*ActivationResponse)
	assert.True(t, resp.Failed, "aborted persist must fail the activation")
	assert.Equal(t, int32(0), persistedAfterAbort.Load(),
		"persist goroutine must skip its KV update after abort")
}

// TestPlacementUnchangedWithoutCallbacks guards the shared-actor blast radius:
// with nil CheckActivationRecord, no self-check is armed and the reuse path
// behaves exactly as before.
func TestPlacementUnchangedWithoutCallbacks(t *testing.T) {
	c := newTestClusterWithKind(t, "skind", echoProps())

	var checks atomic.Int32
	cfg := PlacementConfig{
		PersistActivation: func(ctx context.Context, ci *ClusterIdentity, pid *actor.PID, requestID string) error {
			return nil
		},
		// No CheckActivationRecord / CleanupOwnRecord.
		SelfCheckDelay: 20 * time.Millisecond,
	}
	// Sanity: the check callback (unset) can never be invoked.
	_ = checks
	pp := spawnPlacement(t, c, "$no-cb", cfg)

	// Remote-initiated activation.
	pid := remoteActivation(t, c, pp, &ClusterIdentity{Kind: "skind", Identity: "g-nocb"})

	// With no self-check, the grain must remain alive indefinitely.
	time.Sleep(300 * time.Millisecond)
	assert.True(t, pidAlive(c, pid), "grain must stay alive when self-check is disabled")

	// Reuse path: a second request for the same identity returns the same PID.
	res, err := c.ActorSystem.Root.RequestFuture(pp,
		&ActivationRequest{ClusterIdentity: &ClusterIdentity{Kind: "skind", Identity: "g-nocb"}}, 5*time.Second).Result()
	require.NoError(t, err)
	resp := res.(*ActivationResponse)
	assert.False(t, resp.Failed)
	assert.True(t, pid.Equal(resp.Pid), "reuse path must return the same PID")
}
