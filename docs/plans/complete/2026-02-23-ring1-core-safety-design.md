# Ring 1: Core Safety Design

**Date:** 2026-02-23
**Approach:** Concentric Rings (Ring 1 of 4)
**Goal:** Eliminate all code paths that crash the process or silently corrupt state
**Provider Scope:** automanaged, NATS (all), K8s

## Background

A comprehensive codebase analysis identified 6 critical and 10 high-severity issues across the protoactor-go fork. Ring 1 addresses the most dangerous category: panics in production paths and suppressed errors that can cause silent data corruption.

This ring is a prerequisite for Ring 2 (Remote Resilience) since several Ring 2 changes depend on panics being removed first.

## Prior Work

The 2026-02-16 production readiness effort already addressed:
- atomic.Bool for all provider shutdown flags
- TryResetConsensus implementation
- PubSub.Start panic replacement
- PID cache TTL
- Duplicate spawn prevention in disthash
- Test re-enablement (pubsub, race condition, consensus)
- Provider context cancellation (consul, etcd)

## Changes

### 1. Remote Endpoint Reader - Implement ListProcesses

**File:** `remote/endpoint_reader.go:28-30`

**Current:** `panic("implement me")`

**Design:** Enumerate processes from the actor system's ProcessRegistry. Return a `ListProcessesResponse` containing `ProcessInfo` entries with PID address, ID, and actor type name.

The ProcessRegistry (`actor/process_registry.go`) maintains a `LocalPIDs` sync.Map. Iterate it, collect PIDs, and return them. Filter by any criteria in the `ListProcessesRequest` if the proto defines filters, otherwise return all.

### 2. Remote Endpoint Reader - Implement GetProcessDiagnostics

**File:** `remote/endpoint_reader.go:32-34`

**Current:** `panic("implement me")`

**Design:** For a given PID, look up the process in the registry. Return basic diagnostics:
- Whether the process exists
- The actor type name (via reflection on the underlying actor if available)
- Mailbox message count if accessible

If the process doesn't exist, return a gRPC NotFound status. Check what fields `GetProcessDiagnosticsResponse` proto already defines and populate accordingly.

### 3. Remote Endpoint Reader - Fix mustEmbedUnimplemented

**File:** `remote/endpoint_reader.go:23-26`

**Current:** `panic("implement me")`

**Fix:** Empty body. This is a gRPC forward-compatibility marker method -- it must exist but should be a no-op.

### 4. Remote Endpoint Reader - Implement ClientConnection

**File:** `remote/endpoint_reader.go:114-118`

**Current:** Logs error and does nothing.

**Design:** Handle `ClientConnection` the same way `ServerConnection` is handled (lines 89-113) but without sending the member ID list. Return a `ConnectResponse` with the server's `ServerConnection` info (member ID, blocked members list). The client doesn't need to announce itself to the cluster topology.

### 5. Endpoint Writer Panic Elimination

**File:** `remote/endpoint_writer.go:358-360`

**Current:** `panic(msg.err)` in `restartAfterConnectFailure` handler.

**Fix:** Replace panic with graceful termination:
```go
case *restartAfterConnectFailure:
    state.remote.Logger().Error("EndpointWriter connect failure, terminating endpoint",
        slog.String("address", state.address), slog.Any("error", msg.err))
    terminated := &EndpointTerminatedEvent{Address: state.address}
    state.remote.actorSystem.EventStream.Publish(terminated)
    ctx.Stop(ctx.Self())
```

This matches the existing pattern at lines 60-66 where connection failure after retries publishes EndpointTerminatedEvent. Ring 2 will add exponential backoff here.

### 6. Consul Provider Actor Panic

**File:** `cluster/clusterproviders/consul/provider_actor.go:97-102`

**Current:** `panic(err)` in goroutine when consul watch fails.

**Fix:** Replace panic with error logging and poisoning the provider actor:
```go
go func() {
    if err = plan.RunWithConfig(pa.consulConfig.Address, pa.consulConfig); err != nil {
        ctx.Logger().Error("Failed to start consul watch", slog.Any("error", err))
        ctx.Poison(ctx.Self())
    }
}()
```

`Poison` gracefully stops the actor after processing remaining messages, which is safer than `panic` which crashes the goroutine immediately.

### 7. NATS Identity Error Suppression

**File:** `cluster/identitylookup/nats/nats_identity.go`

**Lines 372, 406, 438:** `data, _ := json.Marshal(&mrec)`

These marshal a `memberRecord{Keys: []string{...}}`. While json.Marshal cannot fail on this type (no channels/funcs/complex numbers), defensive coding requires checking:
```go
data, err := json.Marshal(&mrec)
if err != nil {
    slog.Error("NATS identity: failed to marshal member record",
        slog.String("memberID", memberID), slog.Any("error", err))
    return
}
```

**Line 439:** `_, _ = s.members.Update(ctx, memberID, data, entry.Revision())`

This is a CAS update in `removeKeyFromMember`. If it fails, the tracking record retains a stale key reference. Since this runs during member cleanup (the member is leaving), the tracking record itself will be cleaned up. Log at Warn level:
```go
if _, err := s.members.Update(ctx, memberID, data, entry.Revision()); err != nil {
    slog.Warn("NATS identity: removeKeyFromMember CAS update failed, will be cleaned up on member leave",
        slog.String("memberID", memberID), slog.Any("error", err))
}
```

### 8. Postgres Identity Error Suppression

**File:** `cluster/identitylookup/postgres/postgres_identity.go:129`

**Current:** `_, _ = s.db.ExecContext(ctx, cleanQuery, key)`

**Fix:** Log at Warn level:
```go
if _, err := s.db.ExecContext(ctx, cleanQuery, key); err != nil {
    slog.Warn("Postgres identity: cleanup query failed",
        slog.String("key", key), slog.Any("error", err))
}
```

**Line 276:** `rows, _ := result.RowsAffected()`

**Fix:**
```go
rows, err := result.RowsAffected()
if err != nil {
    slog.Warn("Postgres identity: RowsAffected failed",
        slog.String("key", key), slog.Any("error", err))
}
```

### 9. Proto.Clone Nil Safety

**Files:** `remote/endpoint_writer.go:336`, `remote/endpoint_reader.go:220`

**Current:** `c, _ := proto.Clone(pid).(*actor.PID)` -- if pid is nil, Clone returns nil, type assertion succeeds with nil value, and subsequent `c.RequestId = 0` panics.

**Fix:** Add nil check before clone:
```go
if pid != nil {
    c, _ := proto.Clone(pid).(*actor.PID)
    // ... use c
}
```

### 10. NatsKV Identity Nil Pointer Prevention

**File:** `cluster/clusterproviders/natskv/natskv_identity.go:71-87`

**Current:** `Setup()` returns silently when bucket creation fails, leaving `il.identities` and `il.memberTracker` nil. Any subsequent call to `Get`, `SpawnActivation`, etc. will nil-pointer panic.

**Fix:** The `IdentityLookup` interface's `Setup` method has signature `Setup(cluster *Cluster, kinds []string, isClient bool)` -- no error return. Two options:

**Option A (preferred):** Store the error and fail gracefully on subsequent operations:
```go
type IdentityLookup struct {
    // ... existing fields
    setupErr error
}

func (il *IdentityLookup) Setup(...) {
    // ... existing bucket creation
    if err != nil {
        il.setupErr = fmt.Errorf("natskv identity setup failed: %w", err)
        return
    }
    // ...
}

func (il *IdentityLookup) Get(ctx context.Context, clusterIdentity *cluster.ClusterIdentity) *actor.PID {
    if il.setupErr != nil {
        slog.Error("natskv identity: cannot Get, setup failed", slog.Any("error", il.setupErr))
        return nil
    }
    // ... existing logic
}
```

Apply the same guard to all public methods that use `il.identities` or `il.memberTracker`.

**Option B:** Panic in Setup (fail-fast). This is less desirable because it crashes during cluster startup rather than allowing the cluster to start in a degraded state.

### 11. Prometheus Silent Failure

**File:** `actor/config.go:63-67`

**Current:** Returns nil silently on initialization error.

**Fix:** Log the error. The caller already handles nil:
```go
if err != nil {
    slog.Error("Failed to initialize Prometheus exporter, metrics will be disabled",
        slog.Any("error", err))
    return nil
}
```

## Testing Strategy

- Each fix should have a corresponding unit test or test update
- Run full test suite with race detector after all changes
- Remote endpoint tests: verify ListProcesses returns expected PIDs, GetProcessDiagnostics returns info for known PID and error for unknown
- Error suppression fixes: verify errors are logged (use test log capture)
- Nil safety: test with nil PID inputs

## Risk Assessment

| Change | Risk | Mitigation |
|--------|------|------------|
| ListProcesses implementation | Medium | New code, test thoroughly |
| GetProcessDiagnostics implementation | Medium | New code, test thoroughly |
| Endpoint writer panic removal | Low | Replaces panic with existing pattern |
| Consul actor panic removal | Low | Replaces panic with Poison |
| Error suppression fixes | Low | Adding logging, not changing logic |
| NatsKV nil guard | Low | Defensive checks, no behavior change on success path |

## Dependencies

- None (Ring 1 is the foundation)
- Ring 2 depends on item 5 (endpoint writer panic must be fixed before exponential backoff makes sense)
