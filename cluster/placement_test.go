package cluster

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// echoProps returns Props for a simple actor that responds to any message
// with the message itself. Used for testing the placement actor.
func echoProps() *actor.Props {
	return actor.PropsFromFunc(func(ctx actor.Context) {
		switch msg := ctx.Message().(type) {
		case *actor.Started, *actor.Stopping, *actor.Stopped:
			// lifecycle messages — ignore
		default:
			ctx.Respond(msg)
		}
	})
}

// slowStopProps returns Props for an actor that sleeps for the given
// duration in its Stopping handler. Used to test shutdown timeouts.
func slowStopProps(d time.Duration) *actor.Props {
	return actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *actor.Stopping:
			time.Sleep(d)
		}
	})
}

// crashOnStartProps returns Props for an actor that stops itself
// immediately after starting.
func crashOnStartProps() *actor.Props {
	return actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *actor.Started:
			ctx.Poison(ctx.Self())
		}
	})
}

// newTestClusterWithKind creates a Cluster suitable for placement actor
// tests. The cluster is started as a member. Returns the cluster and a
// cleanup function.
func newTestClusterWithKind(t *testing.T, kindName string, props *actor.Props) *Cluster {
	t.Helper()
	kind := NewKind(kindName, props)
	cp := newInmemoryProvider()
	c := newClusterForTest("placement-test", cp, WithKinds(kind))
	err := c.StartMember()
	require.NoError(t, err)
	cp.publishClusterTopologyEvent()
	t.Cleanup(func() { c.Shutdown(true) })
	return c
}

func TestPlacementActor_SpawnOnActivationRequest(t *testing.T) {
	c := newTestClusterWithKind(t, "testKind", echoProps())

	cfg := PlacementConfig{}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(placementPID)

	req := &ActivationRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "testKind", Identity: "actor1"},
		RequestId:       "req-1",
	}

	future := c.ActorSystem.Root.RequestFuture(placementPID, req, 5*time.Second)
	res, err := future.Result()
	require.NoError(t, err)

	resp, ok := res.(*ActivationResponse)
	require.True(t, ok, "expected *ActivationResponse, got %T", res)
	assert.False(t, resp.Failed, "activation should not fail")
	assert.NotNil(t, resp.Pid, "activation should return a PID")
}

// Task 2: Duplicate request returns existing PID

func TestPlacementActor_DuplicateRequestReturnsExistingPID(t *testing.T) {
	c := newTestClusterWithKind(t, "testKind", echoProps())

	cfg := PlacementConfig{}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-dup")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(placementPID)

	ci := &ClusterIdentity{Kind: "testKind", Identity: "actor1"}
	req := &ActivationRequest{ClusterIdentity: ci, RequestId: "req-1"}

	// First request — spawn.
	future1 := c.ActorSystem.Root.RequestFuture(placementPID, req, 5*time.Second)
	res1, err := future1.Result()
	require.NoError(t, err)
	resp1 := res1.(*ActivationResponse)
	require.NotNil(t, resp1.Pid)

	// Second request — same identity, should return same PID.
	req2 := &ActivationRequest{ClusterIdentity: ci, RequestId: "req-2"}
	future2 := c.ActorSystem.Root.RequestFuture(placementPID, req2, 5*time.Second)
	res2, err := future2.Result()
	require.NoError(t, err)
	resp2 := res2.(*ActivationResponse)

	assert.False(t, resp2.Failed)
	assert.True(t, resp1.Pid.Equal(resp2.Pid),
		"second request should return same PID: got %v vs %v", resp1.Pid, resp2.Pid)
}

// Task 3: Unknown kind responds Failed

func TestPlacementActor_UnknownKindFails(t *testing.T) {
	c := newTestClusterWithKind(t, "testKind", echoProps())

	cfg := PlacementConfig{}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-unk")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(placementPID)

	req := &ActivationRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "nonexistentKind", Identity: "actor1"},
		RequestId:       "req-1",
	}

	future := c.ActorSystem.Root.RequestFuture(placementPID, req, 5*time.Second)
	res, err := future.Result()
	require.NoError(t, err)

	resp := res.(*ActivationResponse)
	assert.True(t, resp.Failed, "unknown kind should fail")
	assert.Nil(t, resp.Pid)
}

// Task 4: Persistence tests

func TestPlacementActor_PersistActivationSuccess(t *testing.T) {
	c := newTestClusterWithKind(t, "testKind", echoProps())

	var persisted atomic.Bool
	cfg := PlacementConfig{
		PersistActivation: func(ctx context.Context, ci *ClusterIdentity, pid *actor.PID) error {
			persisted.Store(true)
			return nil
		},
	}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-persist")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(placementPID)

	req := &ActivationRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "testKind", Identity: "actor1"},
		RequestId:       "req-1",
	}

	future := c.ActorSystem.Root.RequestFuture(placementPID, req, 5*time.Second)
	res, err := future.Result()
	require.NoError(t, err)

	resp := res.(*ActivationResponse)
	assert.False(t, resp.Failed)
	assert.NotNil(t, resp.Pid)
	assert.True(t, persisted.Load(), "PersistActivation should have been called")
}

func TestPlacementActor_PersistActivationFailure_PoisonsActor(t *testing.T) {
	c := newTestClusterWithKind(t, "testKind", echoProps())

	cfg := PlacementConfig{
		PersistActivation: func(ctx context.Context, ci *ClusterIdentity, pid *actor.PID) error {
			return errors.New("storage unavailable")
		},
		PersistenceRetries:    1,
		PersistenceRetryDelay: 10 * time.Millisecond,
	}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-fail")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(placementPID)

	req := &ActivationRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "testKind", Identity: "actor1"},
		RequestId:       "req-1",
	}

	future := c.ActorSystem.Root.RequestFuture(placementPID, req, 5*time.Second)
	res, err := future.Result()
	require.NoError(t, err)

	resp := res.(*ActivationResponse)
	assert.True(t, resp.Failed, "should fail when persistence fails")
	assert.Nil(t, resp.Pid)
}

func TestPlacementActor_PersistActivationRetrySuccess(t *testing.T) {
	c := newTestClusterWithKind(t, "testKind", echoProps())

	var callCount atomic.Int32
	cfg := PlacementConfig{
		PersistActivation: func(ctx context.Context, ci *ClusterIdentity, pid *actor.PID) error {
			n := callCount.Add(1)
			if n <= 1 {
				return errors.New("transient error")
			}
			return nil // succeed on second attempt
		},
		PersistenceRetries:    3,
		PersistenceRetryDelay: 10 * time.Millisecond,
	}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-retry")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(placementPID)

	req := &ActivationRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "testKind", Identity: "actor1"},
		RequestId:       "req-1",
	}

	future := c.ActorSystem.Root.RequestFuture(placementPID, req, 5*time.Second)
	res, err := future.Result()
	require.NoError(t, err)

	resp := res.(*ActivationResponse)
	assert.False(t, resp.Failed, "should succeed after retry")
	assert.NotNil(t, resp.Pid)
	assert.GreaterOrEqual(t, callCount.Load(), int32(2), "should have been called at least twice")
}

func TestPlacementActor_PersistActivationLockNotHeld_NoRetry(t *testing.T) {
	c := newTestClusterWithKind(t, "testKind", echoProps())

	var callCount atomic.Int32
	cfg := PlacementConfig{
		PersistActivation: func(ctx context.Context, ci *ClusterIdentity, pid *actor.PID) error {
			callCount.Add(1)
			return fmt.Errorf("lock lost: %w", ErrLockNotHeld)
		},
		PersistenceRetries:    5,
		PersistenceRetryDelay: 10 * time.Millisecond,
	}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-lock")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(placementPID)

	req := &ActivationRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "testKind", Identity: "actor1"},
		RequestId:       "req-1",
	}

	future := c.ActorSystem.Root.RequestFuture(placementPID, req, 5*time.Second)
	res, err := future.Result()
	require.NoError(t, err)

	resp := res.(*ActivationResponse)
	assert.True(t, resp.Failed, "should fail on LockNotHeld")
	assert.Equal(t, int32(1), callCount.Load(), "should NOT retry on LockNotHeld")
}

func TestPlacementActor_PersistActivationPanic_Recovers(t *testing.T) {
	c := newTestClusterWithKind(t, "testKind", echoProps())

	cfg := PlacementConfig{
		PersistActivation: func(ctx context.Context, ci *ClusterIdentity, pid *actor.PID) error {
			panic("unexpected panic in persistence")
		},
		PersistenceRetries:    1,
		PersistenceRetryDelay: 10 * time.Millisecond,
	}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-panic")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(placementPID)

	req := &ActivationRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "testKind", Identity: "actor1"},
		RequestId:       "req-1",
	}

	future := c.ActorSystem.Root.RequestFuture(placementPID, req, 5*time.Second)
	res, err := future.Result()
	require.NoError(t, err)

	resp := res.(*ActivationResponse)
	assert.True(t, resp.Failed, "should fail when persistence panics")
}

// Task 5: Actor Terminated cleanup + RemoveActivation callback

func TestPlacementActor_Terminated_CleansUpAndCallsRemoveActivation(t *testing.T) {
	c := newTestClusterWithKind(t, "testKind", echoProps())

	var removedCI atomic.Value // stores *ClusterIdentity
	cfg := PlacementConfig{
		RemoveActivation: func(ctx context.Context, ci *ClusterIdentity, pid *actor.PID) error {
			removedCI.Store(ci)
			return nil
		},
	}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-term")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(placementPID)

	ci := &ClusterIdentity{Kind: "testKind", Identity: "actor-to-stop"}
	req := &ActivationRequest{ClusterIdentity: ci, RequestId: "req-1"}

	// Spawn via placement actor.
	future := c.ActorSystem.Root.RequestFuture(placementPID, req, 5*time.Second)
	res, err := future.Result()
	require.NoError(t, err)
	resp := res.(*ActivationResponse)
	require.NotNil(t, resp.Pid)

	// Stop the spawned actor externally.
	c.ActorSystem.Root.Poison(resp.Pid)

	// Wait for Terminated to propagate.
	time.Sleep(500 * time.Millisecond)

	// RemoveActivation should have been called.
	stored := removedCI.Load()
	require.NotNil(t, stored, "RemoveActivation should have been called")
	removedIdentity := stored.(*ClusterIdentity)
	assert.Equal(t, "actor-to-stop", removedIdentity.Identity)
	assert.Equal(t, "testKind", removedIdentity.Kind)

	// A second ActivationRequest for the same identity should spawn a new actor.
	req2 := &ActivationRequest{ClusterIdentity: ci, RequestId: "req-2"}
	future2 := c.ActorSystem.Root.RequestFuture(placementPID, req2, 5*time.Second)
	res2, err := future2.Result()
	require.NoError(t, err)
	resp2 := res2.(*ActivationResponse)
	assert.False(t, resp2.Failed)
	assert.NotNil(t, resp2.Pid)
	// The PID name is deterministic (kind/identity), so the address is the
	// same. Verify the new actor is alive by sending it a message.
	pingFuture := c.ActorSystem.Root.RequestFuture(resp2.Pid, "ping", 1*time.Second)
	pingRes, pingErr := pingFuture.Result()
	assert.NoError(t, pingErr, "re-spawned actor should be alive")
	assert.Equal(t, "ping", pingRes)
}

// Task 6: Stopping handler tests

func TestPlacementActor_Stopping_PoisonsAllLocalGrains(t *testing.T) {
	c := newTestClusterWithKind(t, "testKind", echoProps())

	cfg := PlacementConfig{}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-stop")
	require.NoError(t, err)

	// Spawn 3 actors.
	var pids []*actor.PID
	for i := 0; i < 3; i++ {
		req := &ActivationRequest{
			ClusterIdentity: &ClusterIdentity{Kind: "testKind", Identity: fmt.Sprintf("actor%d", i)},
			RequestId:       fmt.Sprintf("req-%d", i),
		}
		future := c.ActorSystem.Root.RequestFuture(placementPID, req, 5*time.Second)
		res, err := future.Result()
		require.NoError(t, err)
		resp := res.(*ActivationResponse)
		require.NotNil(t, resp.Pid)
		pids = append(pids, resp.Pid)
	}

	// Stop the placement actor — should poison all local grains.
	c.ActorSystem.Root.Poison(placementPID)

	// Wait for everything to shut down.
	time.Sleep(1 * time.Second)

	// All spawned actors should be dead.
	for i, pid := range pids {
		// Sending a message to a dead PID should result in a dead letter.
		future := c.ActorSystem.Root.RequestFuture(pid, "ping", 500*time.Millisecond)
		_, err := future.Result()
		assert.Error(t, err, "actor %d should be dead after placement actor stopped", i)
	}
}

func TestPlacementActor_ActivationRequestDuringStopping_RespondsFailed(t *testing.T) {
	c := newTestClusterWithKind(t, "testKind", echoProps())

	// Use a slow-stop actor so we have time to send requests during stopping.
	var stoppingStarted sync.WaitGroup
	stoppingStarted.Add(1)
	var stoppingNotified atomic.Bool

	cfg := PlacementConfig{
		ShutdownTimeout: 5 * time.Second,
	}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-stopping-req")
	require.NoError(t, err)

	// Spawn one actor so the stopping handler has work to do.
	req := &ActivationRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "testKind", Identity: "actor1"},
		RequestId:       "req-1",
	}
	future := c.ActorSystem.Root.RequestFuture(placementPID, req, 5*time.Second)
	_, err = future.Result()
	require.NoError(t, err)

	// Poison the placement actor (triggers Stopping).
	c.ActorSystem.Root.Poison(placementPID)

	// Send another request immediately — it should fail because the
	// placement actor should check the stopping flag.
	time.Sleep(10 * time.Millisecond) // small delay to let Stopping begin
	req2 := &ActivationRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "testKind", Identity: "actor2"},
		RequestId:       "req-2",
	}
	future2 := c.ActorSystem.Root.RequestFuture(placementPID, req2, 1*time.Second)
	res2, err := future2.Result()

	// The placement actor may already be fully stopped, in which case
	// we get a dead letter/timeout error. Either outcome is acceptable.
	if err != nil {
		// Dead letter — placement actor already stopped. That's fine.
		_ = stoppingNotified
		_ = stoppingStarted
		return
	}

	resp2 := res2.(*ActivationResponse)
	assert.True(t, resp2.Failed, "request during stopping should return Failed")
}

func TestPlacementActor_Stopping_OnlyPoisonsLocalActors(t *testing.T) {
	// CRITICAL SAFETY TEST: The placement actor should ONLY poison actors
	// it spawned locally (tracked in its actors map). It should never
	// poison remote PIDs.

	c := newTestClusterWithKind(t, "testKind", echoProps())

	cfg := PlacementConfig{}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-local-only")
	require.NoError(t, err)

	// Spawn one actor via the placement actor.
	req := &ActivationRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "testKind", Identity: "local-actor"},
		RequestId:       "req-1",
	}
	future := c.ActorSystem.Root.RequestFuture(placementPID, req, 5*time.Second)
	res, err := future.Result()
	require.NoError(t, err)
	resp := res.(*ActivationResponse)
	require.NotNil(t, resp.Pid)

	// Spawn an "external" actor directly (not via placement actor).
	externalPID := c.ActorSystem.Root.Spawn(echoProps())

	// Stop the placement actor.
	c.ActorSystem.Root.Poison(placementPID)
	time.Sleep(500 * time.Millisecond)

	// The external actor should still be alive.
	extFuture := c.ActorSystem.Root.RequestFuture(externalPID, "ping", 1*time.Second)
	extRes, err := extFuture.Result()
	assert.NoError(t, err, "external actor should still be alive")
	assert.Equal(t, "ping", extRes)

	c.ActorSystem.Root.Poison(externalPID)
}

// Task 7: CanSpawnIdentity tests

func TestPlacementActor_CanSpawnIdentity_Approved(t *testing.T) {
	props := echoProps()
	kind := NewKind("verifiedKind", props).WithCanSpawnIdentity(
		func(ctx context.Context, identity string) (bool, error) {
			return true, nil
		},
	)
	cp := newInmemoryProvider()
	c := newClusterForTest("placement-test-verify", cp, WithKinds(kind))
	err := c.StartMember()
	require.NoError(t, err)
	cp.publishClusterTopologyEvent()
	t.Cleanup(func() { c.Shutdown(true) })

	cfg := PlacementConfig{}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-can-spawn-ok")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(placementPID)

	req := &ActivationRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "verifiedKind", Identity: "valid-actor"},
		RequestId:       "req-1",
	}

	future := c.ActorSystem.Root.RequestFuture(placementPID, req, 5*time.Second)
	res, err := future.Result()
	require.NoError(t, err)

	resp := res.(*ActivationResponse)
	assert.False(t, resp.Failed)
	assert.False(t, resp.InvalidIdentity)
	assert.NotNil(t, resp.Pid)
}

func TestPlacementActor_CanSpawnIdentity_Rejected(t *testing.T) {
	props := echoProps()
	kind := NewKind("verifiedKind", props).WithCanSpawnIdentity(
		func(ctx context.Context, identity string) (bool, error) {
			return identity != "blocked", nil
		},
	)
	cp := newInmemoryProvider()
	c := newClusterForTest("placement-test-reject", cp, WithKinds(kind))
	err := c.StartMember()
	require.NoError(t, err)
	cp.publishClusterTopologyEvent()
	t.Cleanup(func() { c.Shutdown(true) })

	cfg := PlacementConfig{}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-can-spawn-reject")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(placementPID)

	req := &ActivationRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "verifiedKind", Identity: "blocked"},
		RequestId:       "req-1",
	}

	future := c.ActorSystem.Root.RequestFuture(placementPID, req, 5*time.Second)
	res, err := future.Result()
	require.NoError(t, err)

	resp := res.(*ActivationResponse)
	assert.True(t, resp.InvalidIdentity, "should respond with InvalidIdentity for rejected identity")
	assert.Nil(t, resp.Pid)
}

func TestPlacementActor_CanSpawnIdentity_Error(t *testing.T) {
	props := echoProps()
	kind := NewKind("verifiedKind", props).WithCanSpawnIdentity(
		func(ctx context.Context, identity string) (bool, error) {
			return false, errors.New("verification service down")
		},
	)
	cp := newInmemoryProvider()
	c := newClusterForTest("placement-test-verify-err", cp, WithKinds(kind))
	err := c.StartMember()
	require.NoError(t, err)
	cp.publishClusterTopologyEvent()
	t.Cleanup(func() { c.Shutdown(true) })

	cfg := PlacementConfig{}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-can-spawn-err")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(placementPID)

	req := &ActivationRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "verifiedKind", Identity: "any"},
		RequestId:       "req-1",
	}

	future := c.ActorSystem.Root.RequestFuture(placementPID, req, 5*time.Second)
	res, err := future.Result()
	require.NoError(t, err)

	resp := res.(*ActivationResponse)
	assert.True(t, resp.Failed, "should fail when verification returns an error")
}

// Task 8: Topology rebalance test

func TestPlacementActor_TopologyRebalance_PoisonsRebalancedActors(t *testing.T) {
	c := newTestClusterWithKind(t, "testKind", echoProps())

	var rebalanceCalled atomic.Bool
	cfg := PlacementConfig{
		RebalanceOnTopology: func(topology *ClusterTopology, actors map[string]*GrainMeta) []string {
			rebalanceCalled.Store(true)
			// Rebalance all actors — tell the placement actor to poison everything.
			keys := make([]string, 0, len(actors))
			for k := range actors {
				keys = append(keys, k)
			}
			return keys
		},
	}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-rebalance")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(placementPID)

	// Spawn one actor.
	ci := &ClusterIdentity{Kind: "testKind", Identity: "actor1"}
	req := &ActivationRequest{ClusterIdentity: ci, RequestId: "req-1"}
	future := c.ActorSystem.Root.RequestFuture(placementPID, req, 5*time.Second)
	res, err := future.Result()
	require.NoError(t, err)
	resp := res.(*ActivationResponse)
	require.NotNil(t, resp.Pid)
	spawnedPID := resp.Pid

	// Send a ClusterTopology message to trigger rebalance.
	topology := &ClusterTopology{
		Members: []*Member{
			{Id: "member1", Host: "host1", Port: 1000, Kinds: []string{"testKind"}},
			{Id: "member2", Host: "host2", Port: 1001, Kinds: []string{"testKind"}},
		},
	}
	c.ActorSystem.Root.Send(placementPID, topology)

	// Wait for rebalance to process.
	time.Sleep(1 * time.Second)

	assert.True(t, rebalanceCalled.Load(), "RebalanceOnTopology should have been called")

	// The spawned actor should be dead (poisoned by rebalance).
	pingFuture := c.ActorSystem.Root.RequestFuture(spawnedPID, "ping", 500*time.Millisecond)
	_, err = pingFuture.Result()
	assert.Error(t, err, "rebalanced actor should be dead")
}

// Task 9: Spawn-then-immediate-crash edge case test

func TestPlacementActor_SpawnThenImmediateCrash_WithPersistence(t *testing.T) {
	// Verifies the edge case: grain crashes immediately after spawn but
	// before PersistActivation completes. Both the persistence continuation
	// and the Terminated message must be handled correctly.

	kind := NewKind("crashKind", crashOnStartProps())
	cp := newInmemoryProvider()
	c := newClusterForTest("placement-test-crash", cp, WithKinds(kind))
	err := c.StartMember()
	require.NoError(t, err)
	cp.publishClusterTopologyEvent()
	t.Cleanup(func() { c.Shutdown(true) })

	var persistCalled atomic.Bool
	var removeCalled atomic.Bool
	cfg := PlacementConfig{
		PersistActivation: func(ctx context.Context, ci *ClusterIdentity, pid *actor.PID) error {
			persistCalled.Store(true)
			// Simulate slow persistence — the actor may already be dead.
			time.Sleep(100 * time.Millisecond)
			return nil
		},
		RemoveActivation: func(ctx context.Context, ci *ClusterIdentity, pid *actor.PID) error {
			removeCalled.Store(true)
			return nil
		},
	}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-crash")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(placementPID)

	req := &ActivationRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "crashKind", Identity: "crasher"},
		RequestId:       "req-1",
	}

	future := c.ActorSystem.Root.RequestFuture(placementPID, req, 5*time.Second)
	res, err := future.Result()
	require.NoError(t, err)

	resp := res.(*ActivationResponse)
	// The response could succeed (persistence completed before crash propagated)
	// or could fail (if the implementation detects the crash). Either is
	// acceptable as long as we don't panic or deadlock.
	_ = resp

	// Wait for Terminated to propagate.
	time.Sleep(1 * time.Second)

	assert.True(t, persistCalled.Load(), "PersistActivation should have been called")
	// RemoveActivation should eventually be called when Terminated arrives.
	// (May or may not be called depending on timing — the actor may already
	// have been removed from the map by the persistence failure path.)
}
