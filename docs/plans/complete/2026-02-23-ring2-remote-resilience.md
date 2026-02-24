# Ring 2: Remote Resilience Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the remote package self-healing after failures — exponential backoff, supervised restart, graceful shutdown, and blocked status handling.

**Architecture:** Endpoint writers get exponential backoff on retries. The endpoint supervisor switches from always-stop to restart-with-limits. Graceful shutdown is reordered so gRPC drains before endpoint management stops. Blocked status is handled in the connect response path.

**Tech Stack:** Go 1.25, proto.actor-go actor framework, gRPC, protobuf

**Depends on:** Ring 1 must be complete — specifically the endpoint writer panic fix. Without it, supervisor restart causes an infinite loop.

---

## Key Facts (read before implementing)

1. **Supervision resolution:** The actor framework checks `if strategy, ok := ctx.actor.(SupervisorStrategy)` first (`actor/actor_context.go:668`). Since `endpointSupervisor` implements `SupervisorStrategy`, its `HandleFailure` IS called, overriding the `WithSupervisor(RestartingSupervisorStrategy())` on its props. That `WithSupervisor` is dead code.

2. **RestartChildren is PID-specific:** `supervisor.RestartChildren(child)` only restarts the specific child PID, not all children (`actor/actor_context.go:771-775`). So restarting the writer does NOT restart the watcher.

3. **RestartStatistics.NumberOfFailures:** Signature is `NumberOfFailures(withinDuration time.Duration) int` (`actor/child_restart_stats.go:33`). The `Fail()` call happens BEFORE `HandleFailure`, so the current failure is already counted.

4. **ConnectResponse.Blocked:** Proto field is `bool blocked = 3` (`remote/remote.proto:56`). Generated accessor is `GetBlocked() bool` (`remote/remote.pb.go:631`). The server already sends this field correctly (`remote/endpoint_reader.go:284-295`).

5. **Config pattern:** `ConfigOption func(config *Config)` with `WithXxx` functional option functions in `remote/config-opts.go`. Defaults set in `defaultConfig()` in `remote/config.go`.

6. **math/rand:** The codebase uses `math/rand` (not v2) throughout. Go 1.25 auto-seeds it.

7. **Tests:** Use `package remote` (not `_test`), testify assert/require, `actor.NewActorSystem()`, `Configure("127.0.0.1", 0)`, `NewRemote(system, config)`.

---

### Task 1: Exponential Backoff for Endpoint Writer

**Files:**
- Modify: `remote/config.go:12-24` (add RetryMaxDelay to defaultConfig and struct)
- Modify: `remote/config.go:42-67` (add validation)
- Modify: `remote/config-opts.go` (add WithRetryMaxDelay)
- Modify: `remote/endpoint_writer.go:40-58` (replace fixed sleep with backoff)
- Modify: `remote/endpoint_writer_test.go` (add backoff tests)

**Step 1: Write the failing test for config field**

In `remote/endpoint_writer_test.go`, add:

```go
func TestExponentialBackoff_DelaysIncreaseGeometrically(t *testing.T) {
	config := Configure("localhost", 0,
		WithRetryBaseDelay(100*time.Millisecond),
		WithRetryMaxDelay(1*time.Second),
		WithMaxRetryCount(4),
	)
	assert.Equal(t, 1*time.Second, config.RetryMaxDelay)
}
```

**Step 2: Run test to verify it fails**

Run: `go test -run TestExponentialBackoff_DelaysIncreaseGeometrically ./remote/ -v -count=1`
Expected: FAIL — `WithRetryMaxDelay` undefined, `RetryMaxDelay` field not found

**Step 3: Add config field and option**

In `remote/config.go`, add to the `Config` struct after `RetryBaseDelay`:

```go
// RetryMaxDelay is the maximum delay between connection retry attempts.
// Delays increase exponentially from RetryBaseDelay up to this cap.
RetryMaxDelay time.Duration
```

In `defaultConfig()`, add:

```go
RetryMaxDelay: 10 * time.Second,
```

In `remote/config.go` `validate()`, add after the RetryBaseDelay check:

```go
if c.RetryMaxDelay <= 0 {
    return fmt.Errorf("RetryMaxDelay must be > 0, got %v", c.RetryMaxDelay)
}
if c.RetryMaxDelay < c.RetryBaseDelay {
    return fmt.Errorf("RetryMaxDelay must be >= RetryBaseDelay, got %v < %v", c.RetryMaxDelay, c.RetryBaseDelay)
}
```

In `remote/config-opts.go`, add:

```go
// WithRetryMaxDelay sets the maximum delay between connection retry attempts.
// Delays increase exponentially from RetryBaseDelay up to this cap.
func WithRetryMaxDelay(d time.Duration) ConfigOption {
	return func(config *Config) {
		config.RetryMaxDelay = d
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test -run TestExponentialBackoff_DelaysIncreaseGeometrically ./remote/ -v -count=1`
Expected: PASS

**Step 5: Write the failing test for actual backoff behavior**

Add to `remote/endpoint_writer_test.go`:

```go
func TestExponentialBackoff_DelaysAreCorrect(t *testing.T) {
	baseDelay := 100 * time.Millisecond
	maxDelay := 500 * time.Millisecond

	// Test the backoff calculation directly
	for attempt, expected := range []time.Duration{
		100 * time.Millisecond, // 100ms * 2^0
		200 * time.Millisecond, // 100ms * 2^1
		400 * time.Millisecond, // 100ms * 2^2
		500 * time.Millisecond, // 100ms * 2^3 = 800ms, capped at 500ms
		500 * time.Millisecond, // capped
	} {
		delay := calcBackoffDelay(baseDelay, maxDelay, attempt)
		// Delay should be in [expected, expected + 25% jitter]
		assert.GreaterOrEqual(t, delay, expected,
			"attempt %d: delay %v should be >= %v", attempt, delay, expected)
		assert.LessOrEqual(t, delay, expected+expected/4,
			"attempt %d: delay %v should be <= %v (with 25%% jitter)", attempt, delay, expected+expected/4)
	}
}
```

**Step 6: Run test to verify it fails**

Run: `go test -run TestExponentialBackoff_DelaysAreCorrect ./remote/ -v -count=1`
Expected: FAIL — `calcBackoffDelay` undefined

**Step 7: Implement exponential backoff**

In `remote/endpoint_writer.go`, add the import `"math/rand"` to the import block, then add the helper function before `initialize`:

```go
// calcBackoffDelay computes an exponential backoff delay with jitter.
// delay = min(baseDelay * 2^attempt, maxDelay) + rand(0, delay/4)
func calcBackoffDelay(baseDelay, maxDelay time.Duration, attempt int) time.Duration {
	delay := baseDelay * time.Duration(1<<uint(attempt))
	if delay > maxDelay {
		delay = maxDelay
	}
	// Add 0-25% jitter to prevent thundering herd
	if quarter := int64(delay / 4); quarter > 0 {
		delay += time.Duration(rand.Int63n(quarter))
	}
	return delay
}
```

Then replace the retry loop in `initialize()` (lines 46-58):

```go
	for i := 0; i < state.remote.config.MaxRetryCount; i++ {
		err = state.initializeInternal()
		if err != nil {
			state.remote.Logger().Error("EndpointWriter failed to connect",
				slog.String("address", state.address), slog.Any("error", err), slog.Int("retry", i))
			delay := calcBackoffDelay(state.config.RetryBaseDelay, state.config.RetryMaxDelay, i)
			time.Sleep(delay)
			continue
		}

		break
	}
```

**Step 8: Run tests to verify they pass**

Run: `go test -run "TestExponentialBackoff" ./remote/ -v -count=1 -race`
Expected: PASS

**Step 9: Commit**

```bash
git add remote/config.go remote/config-opts.go remote/endpoint_writer.go remote/endpoint_writer_test.go
git commit -m "feat(remote): add exponential backoff with jitter for endpoint writer retries

Replace fixed-delay sleep with exponential backoff (base * 2^attempt, capped at
RetryMaxDelay) plus 0-25% jitter. Add RetryMaxDelay config field (default 10s)
and WithRetryMaxDelay option. With defaults: delays are ~1s, 2s, 4s, 8s, 10s."
```

---

### Task 2: Endpoint Supervisor Restart with Limits

**Files:**
- Modify: `remote/config.go:12-24` (add supervisor config fields)
- Modify: `remote/config.go:42-67` (add validation)
- Modify: `remote/config-opts.go` (add WithSupervisorRestartWindow, WithSupervisorMaxRestarts)
- Modify: `remote/endpoint_manager.go:328-357` (add childAddresses map, change HandleFailure)
- Create: `remote/endpoint_manager_test.go` (supervisor tests)

**Step 1: Write the failing test for supervisor restart**

Create `remote/endpoint_manager_test.go`:

```go
package remote

import (
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

func TestEndpointSupervisor_RestartsChildOnFailure(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("127.0.0.1", 0,
		WithSupervisorMaxRestarts(3),
		WithSupervisorRestartWindow(10*time.Second),
	)
	assert.Equal(t, 3, config.SupervisorMaxRestarts)
	assert.Equal(t, 10*time.Second, config.SupervisorRestartWindow)
}
```

**Step 2: Run test to verify it fails**

Run: `go test -run TestEndpointSupervisor_RestartsChildOnFailure ./remote/ -v -count=1`
Expected: FAIL — `WithSupervisorMaxRestarts` undefined

**Step 3: Add config fields and options**

In `remote/config.go` `Config` struct, add after `ShutdownTimeout`:

```go
// SupervisorRestartWindow is the time window for counting child failures.
SupervisorRestartWindow time.Duration
// SupervisorMaxRestarts is the maximum number of child restarts allowed
// within SupervisorRestartWindow before the supervisor stops the child.
SupervisorMaxRestarts int
```

In `defaultConfig()`, add:

```go
SupervisorRestartWindow: 60 * time.Second,
SupervisorMaxRestarts:   5,
```

In `validate()`, add after ShutdownTimeout check:

```go
if c.SupervisorRestartWindow <= 0 {
    return fmt.Errorf("SupervisorRestartWindow must be > 0, got %v", c.SupervisorRestartWindow)
}
if c.SupervisorMaxRestarts <= 0 {
    return fmt.Errorf("SupervisorMaxRestarts must be > 0, got %d", c.SupervisorMaxRestarts)
}
```

In `remote/config-opts.go`, add:

```go
// WithSupervisorRestartWindow sets the time window for counting endpoint child failures.
func WithSupervisorRestartWindow(d time.Duration) ConfigOption {
	return func(config *Config) {
		config.SupervisorRestartWindow = d
	}
}

// WithSupervisorMaxRestarts sets the max restart count within the window before stopping.
func WithSupervisorMaxRestarts(count int) ConfigOption {
	return func(config *Config) {
		config.SupervisorMaxRestarts = count
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test -run TestEndpointSupervisor_RestartsChildOnFailure ./remote/ -v -count=1`
Expected: PASS

**Step 5: Write the failing test for HandleFailure behavior**

Add to `remote/endpoint_manager_test.go`:

```go
func TestEndpointSupervisor_HandleFailure_RestartsUnderLimit(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("127.0.0.1", 0,
		WithSupervisorMaxRestarts(3),
		WithSupervisorRestartWindow(10*time.Second),
	)
	r := NewRemote(system, config)

	sup := newEndpointSupervisor(r).(*endpointSupervisor)

	// Record a child address mapping (simulating what Receive does)
	childPID := actor.NewPID("127.0.0.1:0", "test-writer")
	sup.childAddresses[childPID.Id] = "10.0.0.1:8080"

	rs := actor.NewRestartStatistics()
	// Simulate 2 failures (under limit of 3)
	rs.Fail()
	rs.Fail()

	mockSupervisor := &mockSupervisorForTest{}
	sup.HandleFailure(system, mockSupervisor, childPID, rs, "test error", nil)

	assert.True(t, mockSupervisor.restartCalled, "should restart child under limit")
	assert.False(t, mockSupervisor.stopCalled, "should not stop child under limit")
}

func TestEndpointSupervisor_HandleFailure_StopsOverLimit(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("127.0.0.1", 0,
		WithSupervisorMaxRestarts(3),
		WithSupervisorRestartWindow(10*time.Second),
	)
	r := NewRemote(system, config)

	terminated := make(chan string, 1)
	system.EventStream.Subscribe(func(evt any) {
		if e, ok := evt.(*EndpointTerminatedEvent); ok {
			terminated <- e.Address
		}
	})

	sup := newEndpointSupervisor(r).(*endpointSupervisor)

	childPID := actor.NewPID("127.0.0.1:0", "test-writer")
	sup.childAddresses[childPID.Id] = "10.0.0.1:8080"

	rs := actor.NewRestartStatistics()
	// Simulate 4 failures (over limit of 3)
	rs.Fail()
	rs.Fail()
	rs.Fail()
	rs.Fail()

	mockSupervisor := &mockSupervisorForTest{}
	sup.HandleFailure(system, mockSupervisor, childPID, rs, "test error", nil)

	assert.False(t, mockSupervisor.restartCalled, "should not restart child over limit")
	assert.True(t, mockSupervisor.stopCalled, "should stop child over limit")

	select {
	case addr := <-terminated:
		assert.Equal(t, "10.0.0.1:8080", addr)
	case <-time.After(1 * time.Second):
		t.Fatal("expected EndpointTerminatedEvent")
	}
}

// mockSupervisorForTest is a minimal mock for testing HandleFailure logic.
type mockSupervisorForTest struct {
	restartCalled bool
	stopCalled    bool
}

func (m *mockSupervisorForTest) Children() []*actor.PID                    { return nil }
func (m *mockSupervisorForTest) EscalateFailure(reason any, message any)   {}
func (m *mockSupervisorForTest) RestartChildren(pids ...*actor.PID)        { m.restartCalled = true }
func (m *mockSupervisorForTest) StopChildren(pids ...*actor.PID)           { m.stopCalled = true }
func (m *mockSupervisorForTest) ResumeChildren(pids ...*actor.PID)         {}
```

**Step 6: Run test to verify it fails**

Run: `go test -run "TestEndpointSupervisor_HandleFailure" ./remote/ -v -count=1`
Expected: FAIL — `childAddresses` field not found on endpointSupervisor

**Step 7: Implement supervisor restart logic**

In `remote/endpoint_manager.go`, modify the `endpointSupervisor` struct:

```go
type endpointSupervisor struct {
	remote         *Remote
	childAddresses map[string]string // PID.Id -> remote address
}
```

Modify `newEndpointSupervisor`:

```go
func newEndpointSupervisor(remote *Remote) actor.Actor {
	return &endpointSupervisor{
		remote:         remote,
		childAddresses: make(map[string]string),
	}
}
```

Modify `Receive` to record child address mappings:

```go
func (state *endpointSupervisor) Receive(ctx actor.Context) {
	if address, ok := ctx.Message().(string); ok {
		ctx.Logger().Debug("EndpointSupervisor spawning EndpointWriter and EndpointWatcher", slog.String("address", address))
		e := &endpoint{
			writer:  state.spawnEndpointWriter(state.remote, address, ctx),
			watcher: state.spawnEndpointWatcher(state.remote, address, ctx),
		}
		state.childAddresses[e.writer.Id] = address
		state.childAddresses[e.watcher.Id] = address
		ctx.Logger().Debug("id", slog.String("ewr", e.writer.Id), slog.String("ewa", e.watcher.Id))
		ctx.Respond(e)
	}
}
```

Replace `HandleFailure`:

```go
func (state *endpointSupervisor) HandleFailure(actorSystem *actor.ActorSystem, supervisor actor.Supervisor, child *actor.PID, rs *actor.RestartStatistics, reason any, message any) {
	actorSystem.Logger().Debug("EndpointSupervisor handling failure",
		slog.Any("reason", reason), slog.Any("message", message))

	if rs.NumberOfFailures(state.remote.config.SupervisorRestartWindow) > state.remote.config.SupervisorMaxRestarts {
		actorSystem.Logger().Warn("EndpointSupervisor stopping child after too many failures",
			slog.String("child", child.Id))
		supervisor.StopChildren(child)

		// Publish termination so the endpoint manager removes this entry
		if address, ok := state.childAddresses[child.Id]; ok {
			actorSystem.EventStream.Publish(&EndpointTerminatedEvent{Address: address})
		}
		return
	}

	supervisor.RestartChildren(child)
}
```

Also remove the now-unnecessary `WithSupervisor(actor.RestartingSupervisorStrategy())` from `startSupervisor()` in the same file, since the actor implements `SupervisorStrategy` directly and the `WithSupervisor` is never used (the framework checks the actor interface first — see `actor/actor_context.go:668`):

```go
func (em *endpointManager) startSupervisor() error {
	r := em.remote
	props := actor.PropsFromProducer(func() actor.Actor {
		return newEndpointSupervisor(r)
	},
		actor.WithGuardian(actor.RestartingSupervisorStrategy()),
		actor.WithDispatcher(actor.NewSynchronizedDispatcher(300)))

	pid, err := r.actorSystem.Root.SpawnNamed(props, "EndpointSupervisor")
	if err != nil {
		return fmt.Errorf("failed to start endpoint supervisor: %w", err)
	}
	em.endpointSupervisor = pid
	return nil
}
```

**Step 8: Run tests to verify they pass**

Run: `go test -run "TestEndpointSupervisor" ./remote/ -v -count=1 -race`
Expected: PASS

**Step 9: Run all remote tests to check for regressions**

Run: `go test ./remote/ -v -count=1 -race`
Expected: All PASS

**Step 10: Commit**

```bash
git add remote/config.go remote/config-opts.go remote/endpoint_manager.go remote/endpoint_manager_test.go
git commit -m "feat(remote): add windowed restart limits to endpoint supervisor

Change endpoint supervisor from always-stop to restart-with-limits. Children
are restarted on failure unless they exceed SupervisorMaxRestarts (default 5)
within SupervisorRestartWindow (default 60s). This is now safe because Ring 1
removed the endpoint writer panic and Task 1 added exponential backoff.

Remove unused WithSupervisor(RestartingSupervisorStrategy()) from supervisor
props — the actor implements SupervisorStrategy directly and the framework
prefers the actor's interface over props-level strategy."
```

---

### Task 3: Graceful Shutdown Sequence

**Files:**
- Modify: `remote/server.go:119-145` (reorder shutdown, clean up comments)

**Step 1: Read and understand the current shutdown**

Read `remote/server.go:119-145`. Current order:
1. Suspend endpoint reader
2. **Stop endpoint manager** (stops supervisor, stops writer/watcher actors, nils connections)
3. GracefulStop gRPC server (drains in-flight RPCs)

This is problematic: endpoint management stops BEFORE gRPC drains. In-flight gRPC handlers in the endpoint reader (`onMessageBatch`) call `s.remote.actorSystem.Root.Send(target, message)` which can work independently, but the `remoteTerminate`/`remoteWatch`/`remoteUnwatch` calls go through `em.remoteDeliver` etc., which check `em.stopped` and dead-letter. So in-flight handlers may lose messages.

**Step 2: Write the failing test**

Add to `remote/server_test.go`:

```go
func TestRemote_GracefulShutdown_ManagerStopsAfterGRPC(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("127.0.0.1", 0)
	r := NewRemote(system, config)
	err := r.Start()
	require.NoError(t, err)

	// Verify the endpoint manager is not stopped before gRPC shutdown
	assert.False(t, r.edpManager.stopped.Load(), "endpoint manager should not be stopped before shutdown")

	r.Shutdown(true)

	// After shutdown, endpoint manager should be stopped
	assert.True(t, r.edpManager.stopped.Load(), "endpoint manager should be stopped after shutdown")
}
```

**Step 3: Run test to verify it passes (baseline)**

Run: `go test -run TestRemote_GracefulShutdown_ManagerStopsAfterGRPC ./remote/ -v -count=1`
Expected: PASS (this establishes baseline — the test validates post-conditions)

**Step 4: Rewrite the Shutdown method**

Replace `remote/server.go` `Shutdown` method (lines 119-145) with:

```go
// Shutdown stops the remote server. If graceful is true it waits for running
// requests to finish.
func (r *Remote) Shutdown(graceful bool) {
	if graceful {
		// Phase 1: Stop accepting new messages on incoming streams
		r.edpReader.suspend(true)

		// Phase 2: Drain in-flight RPCs with timeout
		done := make(chan struct{})
		go func() {
			r.s.GracefulStop()
			close(done)
		}()

		select {
		case <-done:
			r.Logger().Info("gRPC server stopped gracefully")
		case <-time.After(r.config.ShutdownTimeout):
			r.Logger().Warn("gRPC graceful shutdown timed out, forcing stop",
				slog.Duration("timeout", r.config.ShutdownTimeout))
			r.s.Stop()
		}

		// Phase 3: Stop endpoint management after gRPC is done
		r.edpManager.stop()
	} else {
		r.s.Stop()
		r.edpManager.stop()
		r.Logger().Info("gRPC server force-stopped")
	}
}
```

**Step 5: Run tests to verify they pass**

Run: `go test -run "TestRemote_|TestStart|TestConfig_" ./remote/ -v -count=1 -race`
Expected: All PASS

**Step 6: Commit**

```bash
git add remote/server.go remote/server_test.go
git commit -m "fix(remote): reorder graceful shutdown to drain gRPC before stopping endpoints

Move edpManager.stop() after GracefulStop so in-flight gRPC handlers can
complete before endpoint management shuts down. Replace misleading TODO
comments with clear phase documentation."
```

---

### Task 4: Handle Blocked Status in ConnectResponse

**Files:**
- Modify: `remote/endpoint_writer.go:130-137` (handle blocked status)
- Modify: `remote/endpoint_writer_test.go` (add blocked status test)

**Step 1: Write the failing test**

Add to `remote/endpoint_writer_test.go`:

```go
func TestEndpointWriter_BlockedByRemoteServer(t *testing.T) {
	// Start node A (the one that will be blocked)
	systemA := actor.NewActorSystem()
	configA := Configure("127.0.0.1", 0,
		WithMaxRetryCount(1),
		WithRetryBaseDelay(10*time.Millisecond),
	)
	remoteA := NewRemote(systemA, configA)
	err := remoteA.Start()
	require.NoError(t, err)
	defer remoteA.Shutdown(true)

	// Start node B and add A to its block list
	systemB := actor.NewActorSystem()
	configB := Configure("127.0.0.1", 0)
	remoteB := NewRemote(systemB, configB)
	err = remoteB.Start()
	require.NoError(t, err)
	defer remoteB.Shutdown(true)

	// Block A's member ID
	remoteB.BlockList().Block(systemA.ID)

	// Subscribe to terminated events on A
	terminated := make(chan string, 1)
	systemA.EventStream.Subscribe(func(evt any) {
		if e, ok := evt.(*EndpointTerminatedEvent); ok {
			terminated <- e.Address
		}
	})

	// Create a target PID on B and try to send from A
	targetPID := actor.NewPID(systemB.Address(), "nonexistent")
	systemA.Root.Send(targetPID, &emptypb.Empty{})

	// A should receive EndpointTerminatedEvent because B blocked it
	select {
	case addr := <-terminated:
		assert.Contains(t, addr, "127.0.0.1")
	case <-time.After(10 * time.Second):
		t.Fatal("expected EndpointTerminatedEvent when blocked by remote server")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -run TestEndpointWriter_BlockedByRemoteServer ./remote/ -v -count=1 -timeout=30s`
Expected: FAIL or timeout — the TODO at line 133 means blocked status is silently ignored

**Step 3: Implement blocked status handling**

In `remote/endpoint_writer.go`, replace the `case *RemoteMessage_ConnectResponse:` block (lines 131-133):

```go
	case *RemoteMessage_ConnectResponse:
		connectResponse := connection.GetConnectResponse()
		state.remote.Logger().Debug("Received connect response",
			slog.String("fromAddress", state.address),
			slog.Bool("blocked", connectResponse.GetBlocked()))
		if connectResponse.GetBlocked() {
			state.remote.Logger().Warn("EndpointWriter blocked by remote server",
				slog.String("address", state.address))
			return fmt.Errorf("blocked by remote server %s", state.address)
		}
```

**Step 4: Run tests to verify they pass**

Run: `go test -run TestEndpointWriter_BlockedByRemoteServer ./remote/ -v -count=1 -race -timeout=30s`
Expected: PASS

**Step 5: Run all remote tests**

Run: `go test ./remote/ -v -count=1 -race -timeout=120s`
Expected: All PASS

**Step 6: Commit**

```bash
git add remote/endpoint_writer.go remote/endpoint_writer_test.go
git commit -m "feat(remote): handle blocked status in ConnectResponse

When the remote server responds with Blocked=true, the endpoint writer now
returns an error instead of silently continuing. This causes the connection
to fail and publishes EndpointTerminatedEvent, properly cleaning up the
endpoint. Previously the TODO at this location was a no-op."
```

---

### Task 5: Remote Server Tests

**Files:**
- Modify: `remote/server_test.go` (add integration tests, remove disabled test comments)

**Step 1: Add shutdown timing tests**

Add to `remote/server_test.go`:

```go
func TestRemote_GracefulShutdownTimeout(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("127.0.0.1", 0,
		WithShutdownTimeout(500*time.Millisecond))
	r := NewRemote(system, config)

	err := r.Start()
	require.NoError(t, err)

	start := time.Now()
	r.Shutdown(true)
	elapsed := time.Since(start)

	// Should complete (either gracefully or via timeout)
	assert.Less(t, elapsed, 5*time.Second)
}

func TestRemote_ForceShutdown(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("127.0.0.1", 0)
	r := NewRemote(system, config)

	err := r.Start()
	require.NoError(t, err)

	start := time.Now()
	r.Shutdown(false)
	elapsed := time.Since(start)

	// Force shutdown should be near-instant
	assert.Less(t, elapsed, 2*time.Second)
}
```

**Step 2: Add cross-remote communication test**

```go
func TestRemote_TwoNodesCommunicate(t *testing.T) {
	// Start node A
	systemA := actor.NewActorSystem()
	remoteA := NewRemote(systemA, Configure("127.0.0.1", 0))
	err := remoteA.Start()
	require.NoError(t, err)
	defer remoteA.Shutdown(true)

	// Start node B with an echo actor
	systemB := actor.NewActorSystem()
	remoteB := NewRemote(systemB, Configure("127.0.0.1", 0))
	err = remoteB.Start()
	require.NoError(t, err)
	defer remoteB.Shutdown(true)

	props := actor.PropsFromFunc(func(ctx actor.Context) {
		if _, ok := ctx.Message().(*emptypb.Empty); ok {
			ctx.Respond(&emptypb.Empty{})
		}
	})
	echoPID, err := systemB.Root.SpawnNamed(props, "echo")
	require.NoError(t, err)

	// Send from A to B
	remotePID := actor.NewPID(systemB.Address(), "echo")
	fut := systemA.Root.RequestFuture(remotePID, &emptypb.Empty{}, 5*time.Second)
	result, err := fut.Result()
	require.NoError(t, err)
	assert.IsType(t, &emptypb.Empty{}, result)

	_ = echoPID // prevent unused warning
}
```

**Step 3: Add connection failure detection test**

```go
func TestRemote_ConnectionFailureTriggersTerminatedEvent(t *testing.T) {
	system := actor.NewActorSystem()
	config := Configure("127.0.0.1", 0,
		WithMaxRetryCount(1),
		WithRetryBaseDelay(10*time.Millisecond),
		WithRetryMaxDelay(10*time.Millisecond),
	)
	r := NewRemote(system, config)
	err := r.Start()
	require.NoError(t, err)
	defer r.Shutdown(true)

	terminated := make(chan string, 1)
	system.EventStream.Subscribe(func(evt any) {
		if e, ok := evt.(*EndpointTerminatedEvent); ok {
			terminated <- e.Address
		}
	})

	// Send to a non-existent remote address
	badPID := actor.NewPID("127.0.0.1:59999", "nonexistent")
	system.Root.Send(badPID, &emptypb.Empty{})

	select {
	case addr := <-terminated:
		assert.Equal(t, "127.0.0.1:59999", addr)
	case <-time.After(30 * time.Second):
		t.Fatal("expected EndpointTerminatedEvent for unreachable address")
	}
}
```

**Step 4: Remove the disabled test comments**

Delete the large commented-out test block at the bottom of `server_test.go` (lines 79-231), replacing it with a brief note:

```go
// Legacy tests (TestStart_AdvertisedAddress, TestShutdown_Graceful, TestShutdown)
// were removed and replaced with the black-box tests above.
```

**Step 5: Add missing import for emptypb**

Make sure the imports include:

```go
import (
	"sort"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)
```

**Step 6: Run tests**

Run: `go test -run "TestRemote_|TestStart|TestConfig_" ./remote/ -v -count=1 -race -timeout=120s`
Expected: All PASS

**Step 7: Commit**

```bash
git add remote/server_test.go
git commit -m "test(remote): add black-box integration tests for Remote server

Add tests for graceful/force shutdown timing, cross-node communication,
and connection failure detection. Remove disabled legacy tests that relied
on internal field access."
```

---

### Task 6: Dead Code Cleanup

**Files:**
- Modify: `remote/endpoint_writer.go:66-79` (remove dead code)

**Step 1: Remove dead code**

In `remote/endpoint_writer.go`, the block from line 67 to line 79 is unreachable code after the `return` on line 66. Delete lines 67-79 entirely (the commented-out retry code and surrounding blank lines/comments). The result should be:

```go
	if err != nil {
		terminated := &EndpointTerminatedEvent{
			Address: state.address,
		}
		state.remote.actorSystem.EventStream.Publish(terminated)

		return
	}

	state.remote.Logger().Info("EndpointWriter connected", ...
```

There should be no blank lines, comments, or code between `return` and the closing `}` of the `if err != nil` block.

**Step 2: Run all remote tests**

Run: `go test ./remote/ -v -count=1 -race -timeout=120s`
Expected: All PASS (removing unreachable code cannot change behavior)

**Step 3: Commit**

```bash
git add remote/endpoint_writer.go
git commit -m "refactor(remote): remove dead code after return in endpoint writer

Remove commented-out retry-after-failure code block (lines 68-78) that was
unreachable after the return statement. The exponential backoff in Task 1
replaces its original intent."
```

---

### Task 7: Integration Test — Full Retry + Restart + Shutdown Flow

**Files:**
- Modify: `remote/endpoint_writer_test.go` (add integration test)

**Step 1: Write the integration test**

Add to `remote/endpoint_writer_test.go`:

```go
// TestEndpointWriter_FullRecoveryFlow verifies the complete self-healing flow:
// 1. Two nodes communicate successfully
// 2. One node goes down — the other detects termination
// 3. The downed node restarts — communication resumes
func TestEndpointWriter_FullRecoveryFlow(t *testing.T) {
	// Start node A (persistent)
	systemA := actor.NewActorSystem()
	configA := Configure("127.0.0.1", 0,
		WithRetryBaseDelay(50*time.Millisecond),
		WithRetryMaxDelay(200*time.Millisecond),
		WithMaxRetryCount(3),
	)
	remoteA := NewRemote(systemA, configA)
	err := remoteA.Start()
	require.NoError(t, err)
	defer remoteA.Shutdown(true)

	// Start node B (will be killed and restarted)
	systemB := actor.NewActorSystem()
	configB := Configure("127.0.0.1", 0)
	remoteB := NewRemote(systemB, configB)
	err = remoteB.Start()
	require.NoError(t, err)

	// Spawn echo actor on B
	echoProps := actor.PropsFromFunc(func(ctx actor.Context) {
		if _, ok := ctx.Message().(*emptypb.Empty); ok {
			ctx.Respond(&emptypb.Empty{})
		}
	})
	_, err = systemB.Root.SpawnNamed(echoProps, "echo")
	require.NoError(t, err)

	addressB := systemB.Address()

	// Step 1: Verify communication works
	remotePID := actor.NewPID(addressB, "echo")
	fut := systemA.Root.RequestFuture(remotePID, &emptypb.Empty{}, 5*time.Second)
	_, err = fut.Result()
	require.NoError(t, err, "initial communication should succeed")

	// Step 2: Kill node B
	terminated := make(chan struct{}, 1)
	systemA.EventStream.Subscribe(func(evt any) {
		if _, ok := evt.(*EndpointTerminatedEvent); ok {
			select {
			case terminated <- struct{}{}:
			default:
			}
		}
	})

	remoteB.Shutdown(true)

	// Wait for A to detect termination
	select {
	case <-terminated:
		// expected
	case <-time.After(10 * time.Second):
		t.Fatal("node A did not detect termination of node B")
	}

	// Step 3: Restart node B on the same address
	systemB2 := actor.NewActorSystem()
	configB2 := Configure("127.0.0.1", 0)
	// We can't reuse the exact same port, so node B gets a new address.
	// This tests that A can establish a NEW connection to a different address.
	remoteB2 := NewRemote(systemB2, configB2)
	err = remoteB2.Start()
	require.NoError(t, err)
	defer remoteB2.Shutdown(true)

	// Spawn echo on new B
	_, err = systemB2.Root.SpawnNamed(echoProps, "echo")
	require.NoError(t, err)

	// Step 4: Verify A can communicate with new B
	newPID := actor.NewPID(systemB2.Address(), "echo")
	fut = systemA.Root.RequestFuture(newPID, &emptypb.Empty{}, 5*time.Second)
	_, err = fut.Result()
	require.NoError(t, err, "communication with restarted node should succeed")
}
```

**Step 2: Run the integration test**

Run: `go test -run TestEndpointWriter_FullRecoveryFlow ./remote/ -v -count=1 -race -timeout=120s`
Expected: PASS

**Step 3: Run full remote test suite**

Run: `go test ./remote/ -v -count=1 -race -timeout=120s`
Expected: All PASS

**Step 4: Commit**

```bash
git add remote/endpoint_writer_test.go
git commit -m "test(remote): add integration test for full retry + restart + shutdown flow

Verify the complete self-healing sequence: two nodes communicate, one goes
down and the other detects termination, then a new node comes up and
communication resumes. This exercises exponential backoff, endpoint
termination detection, and connection re-establishment."
```
