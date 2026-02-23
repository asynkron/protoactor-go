# Ring 2: Remote Resilience Design

**Date:** 2026-02-23
**Approach:** Concentric Rings (Ring 2 of 4)
**Goal:** Make the remote package recover from failures instead of dying permanently
**Depends on:** Ring 1 (Core Safety) -- specifically the endpoint writer panic fix

## Background

After Ring 1 eliminates crashes, the remote package still has resilience gaps: fixed-delay retries, permanently dead endpoints after failure, and incomplete graceful shutdown. This ring makes the remote layer self-healing.

## Changes

### 1. Exponential Backoff for Endpoint Writer

**File:** `remote/endpoint_writer.go:40-58`

**Current:** Fixed delay `time.Sleep(state.config.RetryBaseDelay)` between connection retries.

**Design:** Replace with exponential backoff with jitter:

```go
delay := state.config.RetryBaseDelay * time.Duration(1<<uint(attempt))
if delay > state.config.RetryMaxDelay {
    delay = state.config.RetryMaxDelay
}
// Add 0-25% jitter to prevent thundering herd
jitter := time.Duration(rand.Int63n(int64(delay / 4)))
time.Sleep(delay + jitter)
```

**Config changes** (`remote/config.go`):
- Add `RetryMaxDelay time.Duration` field (default 10s)
- Existing `RetryBaseDelay` remains (default 1s)
- Existing `MaxRetryCount` remains (default 5)

With defaults: delays are 1s, 2s, 4s, 8s, 10s (capped) plus jitter.

### 2. Endpoint Supervisor: Restart with Limits

**File:** `remote/endpoint_manager.go:350-357`

**Current:** Supervisor stops failed children. Comment says "restart will cause a start loop."

**Why restart is now safe:** Ring 1 removes the endpoint_writer panic, and this ring adds exponential backoff. The "start loop" was caused by: connect fail -> panic -> restart -> connect fail -> panic -> restart (infinite). With the panic removed and backoff added, the loop is bounded.

**Design:** Use `RestartStatistics` (currently ignored as `_`) to implement windowed restart limiting:

```go
func (state *endpointSupervisor) HandleFailure(
    actorSystem *actor.ActorSystem,
    supervisor actor.Supervisor,
    child *actor.PID,
    rs *actor.RestartStatistics,
    reason any,
    message any,
) {
    actorSystem.Logger().Debug("EndpointSupervisor handling failure",
        slog.Any("reason", reason), slog.Any("message", message))

    // Allow up to 5 restarts within 60 seconds before escalating to stop
    if rs.NumberOfFailures(60*time.Second) > 5 {
        actorSystem.Logger().Warn("EndpointSupervisor stopping child after too many failures",
            slog.String("child", child.Id))
        supervisor.StopChildren(child)

        // Publish termination so the endpoint manager removes this entry
        actorSystem.EventStream.Publish(&EndpointTerminatedEvent{
            Address: child.Id, // endpoint address is encoded in the PID
        })
        return
    }

    supervisor.RestartChildren(child)
}
```

The restart window (60s) and max count (5) should be configurable via `remote/config.go`:
- `SupervisorRestartWindow time.Duration` (default 60s)
- `SupervisorMaxRestarts int` (default 5)

### 3. Graceful Shutdown Sequence

**File:** `remote/server.go:119-145`

**Current:** Works but has misleading TODO comments suggesting it's a workaround. The pattern (GracefulStop with timeout fallback) is actually the standard gRPC approach.

**Design:** Clean up the sequence and comments:

```go
func (r *Remote) Shutdown(graceful bool) {
    if graceful {
        // Phase 1: Stop accepting new connections
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

        // Phase 3: Stop endpoint management (after gRPC is done)
        r.edpManager.stop()
    } else {
        r.s.Stop()
        r.edpManager.stop()
        r.Logger().Info("gRPC server force-stopped")
    }
}
```

Key change: Move `r.edpManager.stop()` AFTER gRPC shutdown, not before. Endpoint writers may still be processing messages during the drain phase.

### 4. Handle Blocked Status

**File:** `remote/endpoint_writer.go:133`

**Current:** `// TODO: handle blocked status received from remote server`

The `ConnectResponse` can include a `Blocked` field indicating the remote node doesn't accept connections from this node (e.g., it's on a block list).

**Design:**
```go
if connectResponse.Blocked {
    state.remote.Logger().Warn("EndpointWriter blocked by remote server",
        slog.String("address", state.address))
    terminated := &EndpointTerminatedEvent{Address: state.address}
    state.remote.actorSystem.EventStream.Publish(terminated)
    return fmt.Errorf("blocked by remote server %s", state.address)
}
```

Check the proto definition for the exact field name of the blocked status in `ConnectResponse`.

### 5. Remote Server Tests

**File:** `remote/server_test.go`

**Current:** Tests disabled because they rely on internal field access.

**Design:** Rewrite as black-box integration tests:

```go
func TestRemote_StartAndShutdown(t *testing.T) {
    // Create actor system + remote on random port
    system := actor.NewActorSystem()
    config := Configure("127.0.0.1", 0) // port 0 = random
    r := NewRemote(system, config)

    err := r.Start()
    require.NoError(t, err)

    // Verify server is listening
    addr := r.Address()
    assert.NotEmpty(t, addr)

    // Graceful shutdown should complete within timeout
    r.Shutdown(true)
}

func TestRemote_GracefulShutdownTimeout(t *testing.T) {
    // Test that forced stop kicks in after timeout
    system := actor.NewActorSystem()
    config := Configure("127.0.0.1", 0,
        WithShutdownTimeout(100*time.Millisecond))
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

    // Force shutdown should be near-instant
    start := time.Now()
    r.Shutdown(false)
    elapsed := time.Since(start)
    assert.Less(t, elapsed, 1*time.Second)
}
```

Additional tests:
- Two Remote instances can communicate (send message, verify delivery)
- Connection failure triggers EndpointTerminatedEvent
- Endpoint reconnects after transient failure (with restart supervisor)

### 6. Dead Code Cleanup

**File:** `remote/endpoint_writer.go:68-78`

The commented-out retry code after the `return` statement on line 66 is dead code. Remove it entirely -- the exponential backoff in item 1 replaces its intent.

## Testing Strategy

- Exponential backoff: unit test that verifies delays increase geometrically (mock time.Sleep or measure actual delays with short base)
- Supervisor restart: test that endpoint recovers after transient failure; test that it stops after too many failures
- Graceful shutdown: test that timeout works; test that graceful completes when possible
- Blocked status: test with mock server returning blocked response
- Integration: two Remote instances, kill one, verify the other detects termination and can reconnect when the killed one restarts

## Risk Assessment

| Change | Risk | Mitigation |
|--------|------|------------|
| Exponential backoff | Low | Replaces fixed sleep, well-understood pattern |
| Supervisor restart | Medium | Previously caused start loops; mitigated by Ring 1 panic fix + backoff + restart limits |
| Shutdown reorder | Medium | Changing edpManager.stop() timing; test thoroughly |
| Blocked status | Low | New code path, isolated |
| Server tests | Low | New tests, no production code change |
| Dead code removal | Trivial | Removing unreachable code |

## Dependencies

- **Ring 1 item 5** (endpoint writer panic fix) must be complete before item 2 (supervisor restart) is safe
- Ring 3 and Ring 4 do not depend on Ring 2
