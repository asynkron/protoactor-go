# Ring 4: Runtime Kind Addition Design

**Date:** 2026-02-23
**Approach:** Concentric Rings (Ring 4 of 4)
**Goal:** Enable registering new Kinds after the cluster is running, with propagation to all cluster members
**Provider Scope:** automanaged, NATS KV, NATS Stream, K8s
**Depends on:** Ring 1 (Core Safety), Ring 3 (Cluster Operations)

## Background

Currently, Kinds are immutable after `Cluster.StartMember()`. They're captured once by each cluster provider and announced to the cluster as a fixed set. This prevents plugin architectures, rolling deployments that introduce new actor types, and multi-tenant systems with dynamic workloads.

The protobuf `Member` message already includes `repeated string kinds` -- no wire protocol changes are needed. The changes are purely in the Go runtime layer.

## Architecture Overview

```
RegisterKind(kind)
    |
    v
[Cluster.kinds map] -- protected by RWMutex
    |
    v
[KindUpdater.UpdateKinds()] -- called on provider
    |
    v
[Provider re-announces] -- heartbeat/KV update/label update
    |
    v
[Other nodes receive topology update]
    |
    v
[MemberList.UpdateClusterTopology()] -- detects Kind changes
    |
    v
[MemberStrategy updated] -- new Kind now routable
```

Propagation is eventually consistent. Local activation is immediate. Remote activation requires one topology cycle.

## Changes

### 1. Thread-Safe Kind Registry

**File:** `cluster/cluster.go`

Add mutex protection to the `kinds` map:

```go
type Cluster struct {
    // ... existing fields ...
    kindsMu sync.RWMutex
    kinds   map[string]*ActivatedKind
}
```

Update all existing accessors:

```go
func (c *Cluster) GetClusterKinds() []string {
    c.kindsMu.RLock()
    defer c.kindsMu.RUnlock()
    keys := make([]string, 0, len(c.kinds))
    for k := range c.kinds {
        keys = append(keys, k)
    }
    return keys
}

func (c *Cluster) GetClusterKind(kind string) *ActivatedKind {
    c.kindsMu.RLock()
    defer c.kindsMu.RUnlock()
    k, ok := c.kinds[kind]
    if !ok {
        c.Logger().Error("Invalid kind", slog.String("kind", kind))
        return nil
    }
    return k
}

func (c *Cluster) TryGetClusterKind(kind string) (*ActivatedKind, bool) {
    c.kindsMu.RLock()
    defer c.kindsMu.RUnlock()
    k, ok := c.kinds[kind]
    return k, ok
}

func (c *Cluster) VirtualActorCount() int64 {
    c.kindsMu.RLock()
    defer c.kindsMu.RUnlock()
    // ... existing counting logic
}
```

`initKinds()` runs before any goroutines, but add write lock for consistency:

```go
func (c *Cluster) initKinds() {
    c.kindsMu.Lock()
    defer c.kindsMu.Unlock()
    for name, kind := range c.Config.Kinds {
        c.kinds[name] = kind.Build(c)
    }
    c.ensureTopicKindRegisteredLocked()
}
```

### 2. RegisterKind / DeregisterKind API

**File:** `cluster/cluster.go`

```go
// RegisterKind registers a new Kind with the cluster at runtime.
// The Kind becomes available for local activation immediately.
// Other cluster members discover the new Kind on the next topology
// update cycle (typically within one heartbeat interval).
//
// Returns an error if a Kind with the same name is already registered.
func (c *Cluster) RegisterKind(kind *Kind) error {
    c.kindsMu.Lock()
    defer c.kindsMu.Unlock()

    if _, exists := c.kinds[kind.Kind]; exists {
        return fmt.Errorf("kind %q is already registered", kind.Kind)
    }

    c.kinds[kind.Kind] = kind.Build(c)

    // Notify provider to re-announce with updated kinds
    c.notifyKindUpdate()
    return nil
}

// DeregisterKind removes a Kind from the cluster. Returns an error if
// the Kind doesn't exist or if there are active actors of that Kind.
// The TopicActorKind cannot be deregistered.
func (c *Cluster) DeregisterKind(kindName string) error {
    c.kindsMu.Lock()
    defer c.kindsMu.Unlock()

    if kindName == TopicActorKind {
        return fmt.Errorf("kind %q is reserved and cannot be deregistered", kindName)
    }

    if _, exists := c.kinds[kindName]; !exists {
        return fmt.Errorf("kind %q is not registered", kindName)
    }

    delete(c.kinds, kindName)
    c.notifyKindUpdate()
    return nil
}

// getClusterKindsLocked returns the kind names while holding the lock.
// Caller must hold kindsMu.
func (c *Cluster) getClusterKindsLocked() []string {
    keys := make([]string, 0, len(c.kinds))
    for k := range c.kinds {
        keys = append(keys, k)
    }
    return keys
}

func (c *Cluster) notifyKindUpdate() {
    // kindsMu must be held by caller
    if updater, ok := c.provider.(KindUpdater); ok {
        kinds := c.getClusterKindsLocked()
        if err := updater.UpdateKinds(kinds); err != nil {
            c.Logger().Error("Failed to notify provider of kind update",
                slog.Any("error", err))
        }
    }
}
```

The `c.provider` field needs to be stored on the Cluster struct. Currently the provider is only passed to `StartMember`. Add it as a field:

```go
type Cluster struct {
    // ... existing fields ...
    provider ClusterProvider
}
```

Set it in `StartMember()`:
```go
func (c *Cluster) StartMember() error {
    c.provider = c.Config.ClusterProvider
    // ... existing code
}
```

### 3. KindUpdater Interface

**File:** `cluster/cluster_provider.go`

```go
// KindUpdater is an optional interface that ClusterProviders can implement
// to support runtime Kind changes. When implemented, the cluster calls
// UpdateKinds after RegisterKind or DeregisterKind is called.
//
// Providers that don't implement this interface still work -- they just
// won't announce Kind changes to the cluster until the node restarts.
type KindUpdater interface {
    // UpdateKinds is called when the cluster's registered Kinds change.
    // The provider should re-announce the node with the updated kind list.
    // The kinds slice contains all currently registered Kind names.
    UpdateKinds(kinds []string) error
}
```

This is opt-in via interface assertion. No changes to the existing `ClusterProvider` interface.

### 4. Automanaged Provider

**File:** `cluster/clusterproviders/automanaged/automanaged.go`

The automanaged provider broadcasts node state periodically via `getCurrentNode()`.

**Change `getCurrentNode()`** to read Kinds from the cluster rather than cached field:

```go
func (p *AutoManaged) getCurrentNode() *Node {
    kinds := p.cluster.GetClusterKinds()
    return NewNode(p.cluster.ActorSystem.ID, p.host, p.port, kinds)
}
```

**Implement `KindUpdater`:**
```go
func (p *AutoManaged) UpdateKinds(kinds []string) error {
    // Kinds will be picked up on the next broadcast cycle via getCurrentNode().
    // For immediate propagation, trigger a broadcast now.
    p.broadcastState()
    return nil
}
```

If `broadcastState` is not a separate method, extract the broadcast logic from the polling loop into a callable method.

Store the cluster reference (may already be stored, verify):
```go
type AutoManaged struct {
    // ...
    cluster *cluster.Cluster
}
```

### 5. NATS KV Provider

**File:** `cluster/clusterproviders/natskv/natskv_provider.go`

The NATS KV provider stores node state in a KV bucket. `registerSelf()` writes the current node to the bucket.

**Implement `KindUpdater`:**
```go
func (p *Provider) UpdateKinds(kinds []string) error {
    p.mu.Lock()
    p.self.Kinds = kinds
    p.mu.Unlock()

    // Re-register with updated kinds. Other nodes watching the
    // bucket will see the update on the next watch event.
    return p.registerSelf()
}
```

If `p.self` doesn't have a mutex (check current code), add one or use the provider-level mutex.

Also update the heartbeat path to refresh Kinds from the cluster on each publish, as a belt-and-suspenders approach:
```go
func (p *Provider) publishHeartbeat() error {
    p.mu.Lock()
    p.self.Kinds = p.cluster.GetClusterKinds()
    p.mu.Unlock()
    // ... existing publish logic
}
```

### 6. NATS Stream Provider

**File:** `cluster/clusterproviders/natsstream/natsstream_provider.go`

Same pattern as NATS KV. The stream provider publishes node state as stream messages.

**Implement `KindUpdater`:**
```go
func (p *Provider) UpdateKinds(kinds []string) error {
    p.mu.Lock()
    p.self.Kinds = kinds
    p.mu.Unlock()

    // Publish an updated heartbeat immediately so other nodes
    // see the new kinds without waiting for the next heartbeat cycle.
    return p.publishHeartbeat()
}
```

Also update the regular heartbeat to refresh Kinds:
```go
func (p *Provider) publishHeartbeat() error {
    p.mu.Lock()
    p.self.Kinds = p.cluster.GetClusterKinds()
    // ... existing publish logic
    p.mu.Unlock()
    // ...
}
```

### 7. K8s Provider

**File:** `cluster/clusterproviders/k8s/k8s_provider.go`

The K8s provider stores Kinds in pod labels. The `replacePodLabels()` method updates labels on the pod.

**Implement `KindUpdater`:**
```go
func (p *Provider) UpdateKinds(kinds []string) error {
    p.knownKinds = kinds

    // Update pod labels so other nodes watching pods see the change.
    return p.updatePodLabels()
}
```

The existing `replacePodLabels()` method should be updated to include the current kinds in the label set. Check how Kinds are currently encoded in labels (likely as a comma-separated string in a label value or as multiple labels).

The K8s pod watcher on other nodes will detect the label change and trigger a topology update.

### 8. MemberList Kind-Change Tracking

**File:** `cluster/member_list.go`

**Current behavior:** `UpdateClusterTopology` computes joined/left members by comparing old and new member sets. It doesn't detect Kind changes on existing members.

**Add Kind-change detection:**

In `UpdateClusterTopology`, after computing joined/left:

```go
func (ml *MemberList) UpdateClusterTopology(members Members) {
    ml.mutex.Lock()
    defer ml.mutex.Unlock()

    // ... existing topology diff logic ...

    // Detect Kind changes on existing members
    for _, newMember := range members {
        if oldMember, exists := ml.membersByID[newMember.Id]; exists {
            if !kindsEqual(oldMember.Kinds, newMember.Kinds) {
                ml.memberKindsChanged(oldMember, newMember)
            }
        }
    }

    // ... existing join/leave processing ...
}
```

Helper:
```go
func kindsEqual(a, b []string) bool {
    if len(a) != len(b) {
        return false
    }
    aSet := make(map[string]struct{}, len(a))
    for _, k := range a {
        aSet[k] = struct{}{}
    }
    for _, k := range b {
        if _, ok := aSet[k]; !ok {
            return false
        }
    }
    return true
}

func (ml *MemberList) memberKindsChanged(oldMember, newMember *Member) {
    ml.cluster.Logger().Info("Member kinds changed",
        slog.String("member", newMember.Id),
        slog.Any("old", oldMember.Kinds),
        slog.Any("new", newMember.Kinds))

    // Remove from strategies for removed kinds
    oldKindSet := make(map[string]struct{}, len(oldMember.Kinds))
    for _, k := range oldMember.Kinds {
        oldKindSet[k] = struct{}{}
    }
    newKindSet := make(map[string]struct{}, len(newMember.Kinds))
    for _, k := range newMember.Kinds {
        newKindSet[k] = struct{}{}
    }

    // Handle removed kinds
    for _, kind := range oldMember.Kinds {
        if _, inNew := newKindSet[kind]; !inNew {
            if strategy, ok := ml.memberStrategyByKind[kind]; ok {
                strategy.RemoveMember(newMember)
            }
        }
    }

    // Handle added kinds
    for _, kind := range newMember.Kinds {
        if _, inOld := oldKindSet[kind]; !inOld {
            if ml.memberStrategyByKind[kind] == nil {
                ml.memberStrategyByKind[kind] = ml.getMemberStrategyByKind(kind)
            }
            ml.memberStrategyByKind[kind].AddMember(newMember)
        }
    }
}
```

**Important:** Check whether the topology diff logic uses `membersByID` or a set comparison. The Kind-change detection must use whatever data structure stores the previous member state, and it must run before the old state is overwritten with the new state.

Also need to verify that `MemberStrategy.AddMember` is idempotent (doesn't double-add if member already exists for that kind). If not, add a check.

### 9. Propagation Timing and Consistency

**Timing guarantees:**

| Phase | Latency | Scope |
|-------|---------|-------|
| Local availability | Immediate | Registering node only |
| Provider announcement | 0-1 heartbeat interval | Provider-specific |
| Remote topology update | 1-2 heartbeat intervals | All cluster nodes |

During the propagation window, a request for the new Kind that routes to a remote node may fail with "unknown kind" if that node hasn't received the topology update yet. This is acceptable because:

1. The cluster already handles this case -- `GetClusterKind` returns nil for unknown kinds, and callers retry
2. The existing `GrainCallConfig.RetryCount` (default 3) provides automatic retry
3. The propagation window is bounded by the heartbeat interval (typically 1-5 seconds)

**Document this in the RegisterKind godoc.**

### 10. Edge Cases

**Duplicate registration:** Return error "kind already registered".

**Reserved kinds:** `TopicActorKind` cannot be registered or deregistered.

**Registration after shutdown:** Check cluster state; return error if cluster is shut down.

**Concurrent registration:** Protected by `kindsMu` write lock. Safe.

**Deregistration with active actors:** The current design allows deregistration even with active actors. The actors continue running but new activations are not possible. If stricter semantics are needed (fail if actors exist), add a `VirtualActorCountForKind(name)` check. For now, keep it simple -- deregistration just removes from the registry and routing.

**Provider doesn't implement KindUpdater:** Log at Info level that the provider doesn't support runtime Kind updates. The Kind is still registered locally and will be announced on next full restart.

## Testing Strategy

### Unit Tests

```go
// cluster/cluster_kind_test.go

func TestCluster_RegisterKind(t *testing.T)
func TestCluster_RegisterKind_Duplicate(t *testing.T)
func TestCluster_RegisterKind_AfterStart(t *testing.T)
func TestCluster_DeregisterKind(t *testing.T)
func TestCluster_DeregisterKind_Reserved(t *testing.T)
func TestCluster_DeregisterKind_NotFound(t *testing.T)
func TestCluster_GetClusterKinds_ThreadSafe(t *testing.T)
func TestCluster_RegisterKind_ConcurrentAccess(t *testing.T)
```

### Integration Tests Per Provider

```go
// Per-provider test file

func TestProvider_KindUpdate_Propagates(t *testing.T)
// 1. Start 2-node cluster
// 2. Register new Kind on node 1
// 3. Wait for topology update
// 4. Verify node 2 sees the new Kind in member info
// 5. Activate actor of new Kind from node 2, verify it routes to node 1

func TestProvider_KindDeregister_Propagates(t *testing.T)
// 1. Start 2-node cluster with Kind X
// 2. Deregister Kind X on node 1
// 3. Wait for topology update
// 4. Verify node 2 no longer routes Kind X to node 1
```

### Race Detector Tests

```go
func TestCluster_RegisterKind_RaceDetector(t *testing.T) {
    // Run with -race flag
    // Concurrent: RegisterKind, GetClusterKind, TryGetClusterKind, GetClusterKinds
}
```

### MemberList Tests

```go
func TestMemberList_KindChange_Detected(t *testing.T)
func TestMemberList_KindChange_StrategyUpdated(t *testing.T)
func TestMemberList_KindAdded_NewStrategyCreated(t *testing.T)
func TestMemberList_KindRemoved_MemberRemovedFromStrategy(t *testing.T)
```

## Risk Assessment

| Change | Risk | Mitigation |
|--------|------|------------|
| Thread-safe Kind registry | Low | Standard RWMutex, no behavior change |
| RegisterKind API | Low | New method, additive |
| DeregisterKind API | Low | New method, additive |
| KindUpdater interface | Low | Opt-in interface, backward compatible |
| Automanaged provider | Low | Simple delegation |
| NATS KV provider | Low | Re-publish existing pattern |
| NATS Stream provider | Low | Re-publish existing pattern |
| K8s provider | Medium | Pod label update via K8s API, may need RBAC |
| MemberList Kind tracking | High | Modifies topology critical path; must not break existing join/leave logic |

## Migration

No breaking changes. All existing code continues to work unchanged:
- Kinds registered via `WithKinds()` config still work exactly as before
- Providers that don't implement `KindUpdater` still work
- The `kinds` map access is now mutex-protected but semantically identical

New capability is purely additive.
