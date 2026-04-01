# Grain Peek Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a non-activating `Peek` method to all identity lookup implementations that checks whether a grain is alive and returns liveness status.

**Architecture:** Add `PeekRequest`/`PeekResponse` protobuf messages for cross-node communication with the placement actor. Extend the `IdentityLookup` interface with a `Peek` method. Each backend (disthash, natskv, natsstream, storage) implements the three-step pattern: check activation record, validate member liveness, confirm process via placement actor.

**Tech Stack:** Go, protobuf (protoc + protoc-gen-go), testify, testcontainers (NATS)

**Spec:** `docs/superpowers/specs/2026-04-01-grain-peek-design.md`

---

## File Structure

### New files
- `cluster/peek.go` — `PeekStatus` enum, `PeekResult` struct, `PeekStatus.String()`

### Modified files
- `cluster/cluster.proto` — add `PeekRequest`, `PeekResponse` messages
- `cluster/cluster.pb.go` — regenerated
- `cluster/identity_lookup.go` — add `Peek` to `IdentityLookup` interface
- `cluster/cluster.go` — add `Cluster.Peek()` convenience method
- `cluster/placement.go` — add `*PeekRequest` case and `onPeekRequest` handler
- `cluster/activator_proxy.go` — add `*PeekRequest` forwarding case
- `cluster/cluster_test.go` — add `Peek` to `fakeIdentityLookup`
- `cluster/identitylookup/disthash/identity_lookup.go` — implement `Peek`
- `cluster/clusterproviders/natskv/natskv_identity.go` — implement `Peek`
- `cluster/clusterproviders/natsstream/natsstream_identity.go` — implement `Peek`
- `cluster/identitylookup/storage/identity_storage_lookup.go` — implement `Peek`

### Test files (new or modified)
- `cluster/placement_test.go` — PeekRequest handler tests
- `cluster/activator_proxy_test.go` — PeekRequest forwarding tests
- `cluster/cluster_test.go` — `Cluster.Peek()` tests
- `cluster/identitylookup/disthash/manager_test.go` — disthash Peek tests
- `cluster/clusterproviders/natskv/natskv_identity_test.go` — natskv Peek tests
- `cluster/clusterproviders/natsstream/natsstream_identity_test.go` — natsstream Peek tests
- `cluster/identitylookup/storage/identity_storage_lookup_test.go` — storage Peek tests

---

### Task 1: Proto Messages and Go Types

**Files:**
- Modify: `cluster/cluster.proto`
- Regenerate: `cluster/cluster.pb.go`
- Create: `cluster/peek.go`

- [ ] **Step 1: Add PeekRequest and PeekResponse to cluster.proto**

Add these messages at the end of `cluster/cluster.proto`, before the closing of the file:

```protobuf
message PeekRequest {
  ClusterIdentity cluster_identity = 1;
}

message PeekResponse {
  bool found = 1;
  actor.PID pid = 2;
}
```

- [ ] **Step 2: Regenerate cluster.pb.go**

Run from the `cluster/` directory:

```bash
cd cluster && bash build.sh
```

Expected: `cluster.pb.go` is regenerated with `PeekRequest` and `PeekResponse` types. Verify by checking for `type PeekRequest struct` and `type PeekResponse struct` in the generated file.

- [ ] **Step 3: Create cluster/peek.go with PeekStatus and PeekResult**

Create `cluster/peek.go`:

```go
package cluster

import "fmt"

// PeekStatus indicates the liveness state of a grain.
type PeekStatus int

const (
	// PeekStatusNotFound means no activation record exists.
	PeekStatusNotFound PeekStatus = iota
	// PeekStatusAlive means the activation record exists, the owning member
	// is in the cluster, and the placement actor confirmed the process is running.
	PeekStatusAlive
	// PeekStatusMemberDead means an activation record exists but the owning
	// member is no longer in the cluster's member list.
	PeekStatusMemberDead
	// PeekStatusStale means an activation record exists and the owning member
	// is alive, but the placement actor reports no such process. The record
	// is orphaned.
	PeekStatusStale
)

func (s PeekStatus) String() string {
	switch s {
	case PeekStatusNotFound:
		return "not_found"
	case PeekStatusAlive:
		return "alive"
	case PeekStatusMemberDead:
		return "member_dead"
	case PeekStatusStale:
		return "stale"
	default:
		return fmt.Sprintf("PeekStatus(%d)", int(s))
	}
}

// PeekResult contains the result of a non-activating grain liveness check.
type PeekResult struct {
	*GrainInfo
	Status PeekStatus
}
```

- [ ] **Step 4: Verify the project compiles**

```bash
go build ./cluster/...
```

Expected: compiles cleanly.

- [ ] **Step 5: Commit**

```bash
git add cluster/cluster.proto cluster/cluster.pb.go cluster/peek.go
git commit -m "feat(cluster): add PeekRequest/PeekResponse proto messages and PeekStatus/PeekResult types"
```

---

### Task 2: IdentityLookup Interface and Cluster.Peek

**Files:**
- Modify: `cluster/identity_lookup.go`
- Modify: `cluster/cluster.go`
- Modify: `cluster/cluster_test.go` (fakeIdentityLookup)

- [ ] **Step 1: Add Peek to the IdentityLookup interface**

In `cluster/identity_lookup.go`, add the `Peek` method to the `IdentityLookup` interface:

```go
type IdentityLookup interface {
	Get(clusterIdentity *ClusterIdentity) *actor.PID

	// Peek checks whether a grain activation exists without triggering
	// activation. It returns liveness status based on the activation record,
	// member health, and placement actor confirmation.
	Peek(clusterIdentity *ClusterIdentity) (*PeekResult, error)

	RemovePid(clusterIdentity *ClusterIdentity, pid *actor.PID)

	Setup(cluster *Cluster, kinds []string, isClient bool)

	Shutdown()
}
```

- [ ] **Step 2: Add Peek to fakeIdentityLookup in cluster_test.go**

In `cluster/cluster_test.go`, add a `Peek` method to `fakeIdentityLookup` after the existing `Shutdown` method:

```go
func (l *fakeIdentityLookup) Peek(identity *ClusterIdentity) (*PeekResult, error) {
	if val, ok := l.m.Load(identity.Identity); ok {
		pid := val.(*actor.PID)
		return &PeekResult{
			GrainInfo: &GrainInfo{
				Identity: identity.Identity,
				Kind:     identity.Kind,
				PID:      pid,
			},
			Status: PeekStatusAlive,
		}, nil
	}
	return &PeekResult{
		GrainInfo: &GrainInfo{
			Identity: identity.Identity,
			Kind:     identity.Kind,
		},
		Status: PeekStatusNotFound,
	}, nil
}
```

- [ ] **Step 3: Add Cluster.Peek convenience method**

In `cluster/cluster.go`, add after the existing `Cluster.Get` method:

```go
// Peek checks whether a grain activation exists without triggering activation.
// Returns a PeekResult with liveness status. See PeekStatus for possible states.
func (c *Cluster) Peek(identity, kind string) (*PeekResult, error) {
	return c.IdentityLookup.Peek(NewClusterIdentity(identity, kind))
}
```

- [ ] **Step 4: Verify compilation fails only for unimplemented Peek on real backends**

```bash
go build ./cluster/...
```

Expected: Compilation errors in disthash, natskv, natsstream, and storage packages because their types don't implement `Peek` yet. The cluster package itself should compile.

- [ ] **Step 5: Add stub Peek methods to all backends to unblock compilation**

Add minimal stubs to each backend so the project compiles. These will be replaced with real implementations in later tasks.

In `cluster/identitylookup/disthash/identity_lookup.go`:

```go
func (p *IdentityLookup) Peek(clusterIdentity *cluster.ClusterIdentity) (*cluster.PeekResult, error) {
	panic("not implemented")
}
```

In `cluster/clusterproviders/natskv/natskv_identity.go`:

```go
func (il *IdentityLookup) Peek(clusterIdentity *cluster.ClusterIdentity) (*cluster.PeekResult, error) {
	panic("not implemented")
}
```

In `cluster/clusterproviders/natsstream/natsstream_identity.go`:

```go
func (il *IdentityLookup) Peek(clusterIdentity *cluster.ClusterIdentity) (*cluster.PeekResult, error) {
	panic("not implemented")
}
```

In `cluster/identitylookup/storage/identity_storage_lookup.go`:

```go
func (l *IdentityStorageLookup) Peek(clusterIdentity *cluster.ClusterIdentity) (*cluster.PeekResult, error) {
	panic("not implemented")
}
```

- [ ] **Step 6: Verify the entire project compiles**

```bash
go build ./...
```

Expected: compiles cleanly.

- [ ] **Step 7: Write test for Cluster.Peek delegation**

Add to `cluster/cluster_test.go`:

```go
func TestClusterPeek_DelegatesToIdentityLookup(t *testing.T) {
	cp := newInmemoryProvider()
	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	kind := NewKind("test-kind", props)
	c := newClusterForTest("peek-test", cp, WithKinds(kind))
	err := c.StartMember()
	require.NoError(t, err)
	defer c.Shutdown(true)

	result, err := c.Peek("some-identity", "test-kind")
	require.NoError(t, err)
	assert.Equal(t, PeekStatusNotFound, result.Status)
	assert.Equal(t, "some-identity", result.Identity)
	assert.Equal(t, "test-kind", result.Kind)
}
```

- [ ] **Step 8: Run the test**

```bash
go test -v -run TestClusterPeek_DelegatesToIdentityLookup ./cluster/
```

Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add cluster/identity_lookup.go cluster/cluster.go cluster/cluster_test.go \
  cluster/identitylookup/disthash/identity_lookup.go \
  cluster/clusterproviders/natskv/natskv_identity.go \
  cluster/clusterproviders/natsstream/natsstream_identity.go \
  cluster/identitylookup/storage/identity_storage_lookup.go
git commit -m "feat(cluster): add Peek to IdentityLookup interface and Cluster.Peek convenience method"
```

---

### Task 3: Placement Actor PeekRequest Handler

**Files:**
- Modify: `cluster/placement.go`
- Modify: `cluster/placement_test.go`

- [ ] **Step 1: Write tests for the placement actor PeekRequest handler**

Add to `cluster/placement_test.go`:

```go
func TestPlacementActor_PeekRequest_Found(t *testing.T) {
	c := newTestClusterWithKind(t, "test-kind", echoProps())

	cfg := PlacementConfig{}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-peek-found")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(placementPID)

	// First, activate a grain.
	activateReq := &ActivationRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "test-kind", Identity: "grain-1"},
		RequestId:       "req-1",
	}
	activateFuture := c.ActorSystem.Root.RequestFuture(placementPID, activateReq, 5*time.Second)
	activateRes, err := activateFuture.Result()
	require.NoError(t, err)
	activateResp := activateRes.(*ActivationResponse)
	require.False(t, activateResp.Failed)
	require.NotNil(t, activateResp.Pid)

	// Peek for the same identity — should be found.
	peekReq := &PeekRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "test-kind", Identity: "grain-1"},
	}
	peekFuture := c.ActorSystem.Root.RequestFuture(placementPID, peekReq, 5*time.Second)
	peekRes, err := peekFuture.Result()
	require.NoError(t, err)
	peekResp, ok := peekRes.(*PeekResponse)
	require.True(t, ok)
	assert.True(t, peekResp.Found)
	assert.Equal(t, activateResp.Pid, peekResp.Pid)
}

func TestPlacementActor_PeekRequest_NotFound(t *testing.T) {
	c := newTestClusterWithKind(t, "test-kind", echoProps())

	cfg := PlacementConfig{}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-peek-notfound")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(placementPID)

	// Peek for an identity that was never activated.
	peekReq := &PeekRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "test-kind", Identity: "nonexistent"},
	}
	peekFuture := c.ActorSystem.Root.RequestFuture(placementPID, peekReq, 5*time.Second)
	peekRes, err := peekFuture.Result()
	require.NoError(t, err)
	peekResp, ok := peekRes.(*PeekResponse)
	require.True(t, ok)
	assert.False(t, peekResp.Found)
	assert.Nil(t, peekResp.Pid)
}

func TestPlacementActor_PeekRequest_WhileStopping(t *testing.T) {
	c := newTestClusterWithKind(t, "test-kind", slowStopProps(2*time.Second))

	cfg := PlacementConfig{}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-peek-stopping")
	require.NoError(t, err)

	// Activate a grain.
	activateReq := &ActivationRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "test-kind", Identity: "grain-1"},
		RequestId:       "req-1",
	}
	activateFuture := c.ActorSystem.Root.RequestFuture(placementPID, activateReq, 5*time.Second)
	activateRes, err := activateFuture.Result()
	require.NoError(t, err)
	activateResp := activateRes.(*ActivationResponse)
	require.False(t, activateResp.Failed)

	// Start poisoning — placement actor enters stopping state.
	c.ActorSystem.Root.Poison(placementPID)
	time.Sleep(100 * time.Millisecond) // Let the poison arrive.

	// Peek while stopping — should report not found.
	peekReq := &PeekRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "test-kind", Identity: "grain-1"},
	}
	peekFuture := c.ActorSystem.Root.RequestFuture(placementPID, peekReq, 5*time.Second)
	peekRes, err := peekFuture.Result()
	require.NoError(t, err)
	peekResp, ok := peekRes.(*PeekResponse)
	require.True(t, ok)
	assert.False(t, peekResp.Found, "peek should report not found while placement is stopping")
}

func TestPlacementActor_PeekRequest_ConcurrentWithActivation(t *testing.T) {
	c := newTestClusterWithKind(t, "test-kind", echoProps())

	cfg := PlacementConfig{}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-peek-concurrent")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(placementPID)

	// Send activation and peek concurrently for 50 different identities.
	const n = 50
	var wg sync.WaitGroup
	wg.Add(n * 2)

	for i := 0; i < n; i++ {
		identity := fmt.Sprintf("grain-%d", i)

		go func() {
			defer wg.Done()
			req := &ActivationRequest{
				ClusterIdentity: &ClusterIdentity{Kind: "test-kind", Identity: identity},
				RequestId:       fmt.Sprintf("req-%s", identity),
			}
			future := c.ActorSystem.Root.RequestFuture(placementPID, req, 5*time.Second)
			_, _ = future.Result()
		}()

		go func() {
			defer wg.Done()
			req := &PeekRequest{
				ClusterIdentity: &ClusterIdentity{Kind: "test-kind", Identity: identity},
			}
			future := c.ActorSystem.Root.RequestFuture(placementPID, req, 5*time.Second)
			res, err := future.Result()
			if err == nil {
				resp, ok := res.(*PeekResponse)
				if ok {
					// Either found or not ��� both valid during concurrent activation.
					_ = resp.Found
				}
			}
		}()
	}

	wg.Wait()
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test -v -race -run "TestPlacementActor_PeekRequest" ./cluster/
```

Expected: FAIL — placement actor doesn't handle `*PeekRequest` yet.

- [ ] **Step 3: Implement the PeekRequest handler in the placement actor**

In `cluster/placement.go`, add the `*PeekRequest` case in the `Receive` method's switch statement, between the `*ListGrainsRequest` case and the `default` case:

```go
	case *PeekRequest:
		p.onPeekRequest(ctx, msg)
```

Add the handler method after `onListGrains`:

```go
// onPeekRequest checks whether a specific identity is active locally
// without spawning. This is a read-only, side-effect-free operation.
func (p *placementActor) onPeekRequest(ctx actor.Context, msg *PeekRequest) {
	if p.stopping {
		ctx.Respond(&PeekResponse{Found: false})
		return
	}

	key := msg.ClusterIdentity.AsKey()
	if meta, found := p.actors[key]; found {
		ctx.Respond(&PeekResponse{Found: true, Pid: meta.PID})
		return
	}

	ctx.Respond(&PeekResponse{Found: false})
}
```

- [ ] **Step 4: Run the tests**

```bash
go test -v -race -run "TestPlacementActor_PeekRequest" ./cluster/
```

Expected: all 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add cluster/placement.go cluster/placement_test.go
git commit -m "feat(cluster): add PeekRequest handler to placement actor"
```

---

### Task 4: Activator Proxy PeekRequest Forwarding

**Files:**
- Modify: `cluster/activator_proxy.go`
- Modify: `cluster/activator_proxy_test.go`

- [ ] **Step 1: Write tests for proxy PeekRequest forwarding**

Add to `cluster/activator_proxy_test.go`:

```go
func TestActivatorProxy_ForwardsPeekRequest(t *testing.T) {
	c := newTestClusterWithKind(t, "test-kind", echoProps())

	cfg := PlacementConfig{}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-proxy-peek")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(placementPID)

	lookup := &fakeIdentityLookup{}
	lookup.Setup(c, nil, false)
	proxyProps := NewActivatorProxyProps(placementPID, lookup)
	proxyPID, err := c.ActorSystem.Root.SpawnNamed(proxyProps, "$test-proxy-peek")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(proxyPID)

	// Activate a grain via the proxy.
	activateReq := &ActivationRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "test-kind", Identity: "grain-1"},
		RequestId:       "req-1",
	}
	activateFuture := c.ActorSystem.Root.RequestFuture(proxyPID, activateReq, 5*time.Second)
	activateRes, err := activateFuture.Result()
	require.NoError(t, err)
	activateResp := activateRes.(*ActivationResponse)
	require.False(t, activateResp.Failed)

	// Peek via the proxy — should forward to placement and return found.
	peekReq := &PeekRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "test-kind", Identity: "grain-1"},
	}
	peekFuture := c.ActorSystem.Root.RequestFuture(proxyPID, peekReq, 5*time.Second)
	peekRes, err := peekFuture.Result()
	require.NoError(t, err)
	peekResp, ok := peekRes.(*PeekResponse)
	require.True(t, ok)
	assert.True(t, peekResp.Found)
	assert.Equal(t, activateResp.Pid, peekResp.Pid)
}

func TestActivatorProxy_ForwardsPeekRequest_NotFound(t *testing.T) {
	c := newTestClusterWithKind(t, "test-kind", echoProps())

	cfg := PlacementConfig{}
	placementProps := NewPlacementActorProps(c, cfg)
	placementPID, err := c.ActorSystem.Root.SpawnNamed(placementProps, "$test-placement-proxy-peek-nf")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(placementPID)

	lookup := &fakeIdentityLookup{}
	lookup.Setup(c, nil, false)
	proxyProps := NewActivatorProxyProps(placementPID, lookup)
	proxyPID, err := c.ActorSystem.Root.SpawnNamed(proxyProps, "$test-proxy-peek-nf")
	require.NoError(t, err)
	defer c.ActorSystem.Root.Poison(proxyPID)

	// Peek for a non-existent grain via the proxy.
	peekReq := &PeekRequest{
		ClusterIdentity: &ClusterIdentity{Kind: "test-kind", Identity: "nonexistent"},
	}
	peekFuture := c.ActorSystem.Root.RequestFuture(proxyPID, peekReq, 5*time.Second)
	peekRes, err := peekFuture.Result()
	require.NoError(t, err)
	peekResp, ok := peekRes.(*PeekResponse)
	require.True(t, ok)
	assert.False(t, peekResp.Found)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test -v -race -run "TestActivatorProxy_ForwardsPeekRequest" ./cluster/
```

Expected: FAIL — proxy doesn't handle `*PeekRequest` yet. The message hits the default case and is ignored, so the future times out.

- [ ] **Step 3: Implement PeekRequest forwarding in the activator proxy**

In `cluster/activator_proxy.go`, add the `*PeekRequest` case in the `Receive` method's switch statement, after the `*ProxyActivationRequest` case:

```go
	case *PeekRequest:
		a.forwardPeekRequest(ctx, msg)
```

Add the handler method:

```go
// forwardPeekRequest forwards a PeekRequest to the local placement actor
// and responds with the result. This is a read-only operation.
func (a *activatorProxy) forwardPeekRequest(ctx actor.Context, msg *PeekRequest) {
	future := ctx.RequestFuture(a.placementPID, msg, proxyForwardTimeout)

	ctx.ReenterAfter(future, func(res any, err error) {
		if err != nil {
			ctx.Logger().Error("Proxy forward PeekRequest failed",
				slog.String("identity", msg.ClusterIdentity.Identity),
				slog.Any("error", err))
			ctx.Respond(&PeekResponse{Found: false})
			return
		}

		ctx.Respond(res)
	})
}
```

- [ ] **Step 4: Run the tests**

```bash
go test -v -race -run "TestActivatorProxy_ForwardsPeekRequest" ./cluster/
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cluster/activator_proxy.go cluster/activator_proxy_test.go
git commit -m "feat(cluster): add PeekRequest forwarding to activator proxy"
```

---

### Task 5: disthash Peek Implementation

**Files:**
- Modify: `cluster/identitylookup/disthash/identity_lookup.go`
- Modify: `cluster/identitylookup/disthash/manager_test.go`

- [ ] **Step 1: Write tests for disthash Peek**

Add to `cluster/identitylookup/disthash/manager_test.go`:

```go
func TestDisthash_Peek_ActiveGrain(t *testing.T) {
	system := actor.NewActorSystem()
	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	kind := cluster.NewKind("TestKind", props)
	provider := test.NewTestProvider(test.NewInMemAgent())
	lookup := New()
	config := cluster.Configure("test-cluster", provider, lookup,
		remote.Configure("127.0.0.1", 0), cluster.WithKinds(kind))
	c := cluster.NewCluster(system, config)

	manager := newPartitionManager(c)
	manager.Start()
	defer manager.Stop()

	// Register self as the only member so rendezvous always maps to us.
	self := &cluster.Member{Host: "127.0.0.1", Port: 0, Id: system.ID, Kinds: []string{"TestKind"}}
	manager.onClusterTopology(&cluster.ClusterTopology{
		Members: cluster.Members{self},
	})
	c.MemberList.UpdateClusterTopology(cluster.Members{self})

	// Activate a grain.
	identity := &cluster.ClusterIdentity{Identity: "abc", Kind: "TestKind"}
	req := &cluster.ActivationRequest{ClusterIdentity: identity}
	future := system.Root.RequestFuture(manager.placementActor, req, 5*time.Second)
	res, err := future.Result()
	require.NoError(t, err)
	resp := res.(*cluster.ActivationResponse)
	require.False(t, resp.Failed)
	require.NotNil(t, resp.Pid)

	// Peek should return Alive.
	peekResult, err := lookup.Peek(identity)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusAlive, peekResult.Status)
	assert.Equal(t, "abc", peekResult.Identity)
	assert.Equal(t, "TestKind", peekResult.Kind)
	assert.Equal(t, resp.Pid, peekResult.PID)
}

func TestDisthash_Peek_NoActivation(t *testing.T) {
	system := actor.NewActorSystem()
	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	kind := cluster.NewKind("TestKind", props)
	provider := test.NewTestProvider(test.NewInMemAgent())
	lookup := New()
	config := cluster.Configure("test-cluster", provider, lookup,
		remote.Configure("127.0.0.1", 0), cluster.WithKinds(kind))
	c := cluster.NewCluster(system, config)

	manager := newPartitionManager(c)
	manager.Start()
	defer manager.Stop()

	self := &cluster.Member{Host: "127.0.0.1", Port: 0, Id: system.ID, Kinds: []string{"TestKind"}}
	manager.onClusterTopology(&cluster.ClusterTopology{
		Members: cluster.Members{self},
	})
	c.MemberList.UpdateClusterTopology(cluster.Members{self})

	// Peek without activating — should return NotFound.
	identity := &cluster.ClusterIdentity{Identity: "nonexistent", Kind: "TestKind"}
	peekResult, err := lookup.Peek(identity)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusNotFound, peekResult.Status)
}

func TestDisthash_Peek_AfterTermination(t *testing.T) {
	system := actor.NewActorSystem()
	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	kind := cluster.NewKind("TestKind", props)
	provider := test.NewTestProvider(test.NewInMemAgent())
	lookup := New()
	config := cluster.Configure("test-cluster", provider, lookup,
		remote.Configure("127.0.0.1", 0), cluster.WithKinds(kind))
	c := cluster.NewCluster(system, config)

	manager := newPartitionManager(c)
	manager.Start()
	defer manager.Stop()

	self := &cluster.Member{Host: "127.0.0.1", Port: 0, Id: system.ID, Kinds: []string{"TestKind"}}
	manager.onClusterTopology(&cluster.ClusterTopology{
		Members: cluster.Members{self},
	})
	c.MemberList.UpdateClusterTopology(cluster.Members{self})

	// Activate and then terminate.
	identity := &cluster.ClusterIdentity{Identity: "will-die", Kind: "TestKind"}
	req := &cluster.ActivationRequest{ClusterIdentity: identity}
	future := system.Root.RequestFuture(manager.placementActor, req, 5*time.Second)
	res, err := future.Result()
	require.NoError(t, err)
	resp := res.(*cluster.ActivationResponse)
	require.False(t, resp.Failed)

	// Stop the grain.
	system.Root.Poison(resp.Pid)
	time.Sleep(500 * time.Millisecond) // Wait for termination to propagate.

	// Peek should return NotFound.
	peekResult, err := lookup.Peek(identity)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusNotFound, peekResult.Status)
}

func TestDisthash_Peek_NoMembers(t *testing.T) {
	system := actor.NewActorSystem()
	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	kind := cluster.NewKind("TestKind", props)
	provider := test.NewTestProvider(test.NewInMemAgent())
	lookup := New()
	config := cluster.Configure("test-cluster", provider, lookup,
		remote.Configure("127.0.0.1", 0), cluster.WithKinds(kind))
	c := cluster.NewCluster(system, config)

	manager := newPartitionManager(c)
	manager.Start()
	defer manager.Stop()

	// No members registered — rendezvous returns empty.
	identity := &cluster.ClusterIdentity{Identity: "abc", Kind: "TestKind"}
	peekResult, err := lookup.Peek(identity)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusNotFound, peekResult.Status)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test -v -race -run "TestDisthash_Peek" ./cluster/identitylookup/disthash/
```

Expected: FAIL — panics with "not implemented".

- [ ] **Step 3: Implement Peek for disthash**

Replace the stub `Peek` method in `cluster/identitylookup/disthash/identity_lookup.go` with:

```go
// Peek checks if a grain activation exists without triggering activation.
// For disthash, this hashes to the owner member and sends a PeekRequest
// to the placement actor.
func (p *IdentityLookup) Peek(clusterIdentity *cluster.ClusterIdentity) (*cluster.PeekResult, error) {
	pm := p.partitionManager
	notFound := &cluster.PeekResult{
		GrainInfo: &cluster.GrainInfo{
			Identity: clusterIdentity.Identity,
			Kind:     clusterIdentity.Kind,
		},
		Status: cluster.PeekStatusNotFound,
	}

	// Snapshot rendezvous under read lock.
	pm.rdvMutex.RLock()
	rdv := pm.rdv
	pm.rdvMutex.RUnlock()

	ownerAddress := rdv.GetByClusterIdentity(clusterIdentity)
	if ownerAddress == "" {
		return notFound, nil
	}

	// Check if the owning member is still in the cluster.
	memberSet := pm.cluster.MemberList.Members()
	if memberSet == nil {
		return notFound, nil
	}

	memberAlive := false
	var ownerMember *cluster.Member
	for _, m := range memberSet.Members() {
		if m.Address() == ownerAddress {
			memberAlive = true
			ownerMember = m
			break
		}
	}

	if !memberAlive {
		return &cluster.PeekResult{
			GrainInfo: &cluster.GrainInfo{
				Identity: clusterIdentity.Identity,
				Kind:     clusterIdentity.Kind,
			},
			Status: cluster.PeekStatusMemberDead,
		}, nil
	}

	// Send PeekRequest to the placement actor on the owning member.
	placementPID := pm.PidOfActivatorActor(ownerAddress)
	future := pm.cluster.ActorSystem.Root.RequestFuture(placementPID, &cluster.PeekRequest{
		ClusterIdentity: clusterIdentity,
	}, 5*time.Second)

	res, err := future.Result()
	if err != nil {
		return nil, fmt.Errorf("peek request to %s failed: %w", ownerAddress, err)
	}

	peekResp, ok := res.(*cluster.PeekResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type from placement actor: %T", res)
	}

	if peekResp.Found {
		return &cluster.PeekResult{
			GrainInfo: &cluster.GrainInfo{
				Identity: clusterIdentity.Identity,
				Kind:     clusterIdentity.Kind,
				PID:      peekResp.Pid,
				MemberID: ownerMember.Id,
			},
			Status: cluster.PeekStatusAlive,
		}, nil
	}

	// disthash has no persistent records — if placement says not found,
	// the grain simply doesn't exist.
	return notFound, nil
}
```

Also add `"fmt"` to the import block if not already present.

- [ ] **Step 4: Run tests**

```bash
go test -v -race -run "TestDisthash_Peek" ./cluster/identitylookup/disthash/
```

Expected: all 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add cluster/identitylookup/disthash/identity_lookup.go cluster/identitylookup/disthash/manager_test.go
git commit -m "feat(disthash): implement Peek for non-activating grain liveness check"
```

---

### Task 6: natskv Peek Implementation

**Files:**
- Modify: `cluster/clusterproviders/natskv/natskv_identity.go`
- Modify: `cluster/clusterproviders/natskv/natskv_identity_test.go`

- [ ] **Step 1: Write tests for natskv Peek**

Add to `cluster/clusterproviders/natskv/natskv_identity_test.go`:

```go
func TestNatsKV_Peek_Alive(t *testing.T) {
	p, c, il := setupPlacementTestCluster(t, "peek-alive")

	// Activate a grain via the identity lookup.
	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "peek-alive-1"}
	pid := il.Get(ci)
	require.NotNil(t, pid, "Get should activate the grain")

	// Peek should return Alive.
	result, err := il.Peek(ci)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusAlive, result.Status)
	assert.Equal(t, "peek-alive-1", result.Identity)
	assert.Equal(t, "TestKind", result.Kind)
	assert.Equal(t, pid, result.PID)

	_ = p // keep reference
}

func TestNatsKV_Peek_NotFound(t *testing.T) {
	_, _, il := setupPlacementTestCluster(t, "peek-notfound")

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "never-activated"}
	result, err := il.Peek(ci)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusNotFound, result.Status)
}

func TestNatsKV_Peek_MemberDead(t *testing.T) {
	_, _, il := setupPlacementTestCluster(t, "peek-member-dead")

	// Write a fake activation record for a dead member into KV.
	ctx := context.Background()
	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "dead-grain"}
	rec := activationRecord{
		MemberID:   "dead-member-id",
		PidID:      "TestKind/dead-grain",
		PidAddress: "dead-host:9999",
	}
	data, _ := json.Marshal(&rec)
	_, err := il.identities.Put(ctx, kvKey(ci), data)
	require.NoError(t, err)

	// "dead-member-id" is not in the member list.
	result, err := il.Peek(ci)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusMemberDead, result.Status)
	assert.Equal(t, "dead-grain", result.Identity)
}

func TestNatsKV_Peek_Stale(t *testing.T) {
	p, c, il := setupPlacementTestCluster(t, "peek-stale")

	// Activate a grain.
	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "peek-stale-1"}
	pid := il.Get(ci)
	require.NotNil(t, pid)

	// Stop the grain process directly (bypassing identity cleanup).
	c.ActorSystem.Root.Poison(pid)
	time.Sleep(500 * time.Millisecond)

	// The NATS KV record still exists, but placement actor no longer has it.
	result, err := il.Peek(ci)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusStale, result.Status)

	_ = p
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test -v -race -run "TestNatsKV_Peek" ./cluster/clusterproviders/natskv/
```

Expected: FAIL — panics with "not implemented".

- [ ] **Step 3: Implement Peek for natskv**

Replace the stub `Peek` method in `cluster/clusterproviders/natskv/natskv_identity.go` with:

```go
// Peek checks if a grain activation exists without triggering activation.
func (il *IdentityLookup) Peek(clusterIdentity *cluster.ClusterIdentity) (*cluster.PeekResult, error) {
	notFound := &cluster.PeekResult{
		GrainInfo: &cluster.GrainInfo{
			Identity: clusterIdentity.Identity,
			Kind:     clusterIdentity.Kind,
		},
		Status: cluster.PeekStatusNotFound,
	}

	if il.setupErr != nil {
		return nil, fmt.Errorf("natskv identity: cannot Peek, setup failed: %w", il.setupErr)
	}
	if il.defunct {
		return nil, fmt.Errorf("natskv identity: cannot Peek, shutdown was called")
	}

	// Step 1: Check for existing activation in NATS KV.
	ctx := context.Background()
	existing := il.getExistingActivation(ctx, clusterIdentity)
	if existing == nil {
		return notFound, nil
	}

	// Step 2: Validate owning member is alive.
	if !cluster.ValidateActivationMember(il.cluster.MemberList, existing.MemberID) {
		return &cluster.PeekResult{
			GrainInfo: &cluster.GrainInfo{
				Identity: clusterIdentity.Identity,
				Kind:     clusterIdentity.Kind,
				MemberID: existing.MemberID,
			},
			Status: cluster.PeekStatusMemberDead,
		}, nil
	}

	// Step 3: Confirm process alive via placement actor.
	ownerAddress := existing.PidAddress
	proxyPID := actor.NewPID(ownerAddress, "$proxy-activator")
	future := il.cluster.ActorSystem.Root.RequestFuture(proxyPID, &cluster.PeekRequest{
		ClusterIdentity: clusterIdentity,
	}, 5*time.Second)

	res, err := future.Result()
	if err != nil {
		return nil, fmt.Errorf("peek request to %s failed: %w", ownerAddress, err)
	}

	peekResp, ok := res.(*cluster.PeekResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", res)
	}

	if peekResp.Found {
		return &cluster.PeekResult{
			GrainInfo: &cluster.GrainInfo{
				Identity: clusterIdentity.Identity,
				Kind:     clusterIdentity.Kind,
				PID:      peekResp.Pid,
				MemberID: existing.MemberID,
			},
			Status: cluster.PeekStatusAlive,
		}, nil
	}

	return &cluster.PeekResult{
		GrainInfo: &cluster.GrainInfo{
			Identity: clusterIdentity.Identity,
			Kind:     clusterIdentity.Kind,
			MemberID: existing.MemberID,
		},
		Status: cluster.PeekStatusStale,
	}, nil
}
```

Add `"context"` and `"fmt"` to the import block if not already present.

- [ ] **Step 4: Run tests**

```bash
go test -v -race -run "TestNatsKV_Peek" ./cluster/clusterproviders/natskv/
```

Expected: all 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add cluster/clusterproviders/natskv/natskv_identity.go cluster/clusterproviders/natskv/natskv_identity_test.go
git commit -m "feat(natskv): implement Peek for non-activating grain liveness check"
```

---

### Task 7: natsstream Peek Implementation

**Files:**
- Modify: `cluster/clusterproviders/natsstream/natsstream_identity.go`
- Modify: `cluster/clusterproviders/natsstream/natsstream_identity_test.go`

- [ ] **Step 1: Write tests for natsstream Peek**

Add to `cluster/clusterproviders/natsstream/natsstream_identity_test.go`:

```go
func TestNatsStream_Peek_Alive(t *testing.T) {
	p, c, il := setupPlacementTestCluster(t, "peek-alive")

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "peek-alive-1"}
	pid := il.Get(ci)
	require.NotNil(t, pid, "Get should activate the grain")

	result, err := il.Peek(ci)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusAlive, result.Status)
	assert.Equal(t, "peek-alive-1", result.Identity)
	assert.Equal(t, "TestKind", result.Kind)
	assert.Equal(t, pid, result.PID)

	_ = p
	_ = c
}

func TestNatsStream_Peek_NotFound(t *testing.T) {
	_, _, il := setupPlacementTestCluster(t, "peek-notfound")

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "never-activated"}
	result, err := il.Peek(ci)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusNotFound, result.Status)
}

func TestNatsStream_Peek_MemberDead(t *testing.T) {
	_, _, il := setupPlacementTestCluster(t, "peek-member-dead")

	// Write a fake activation record for a dead member into the stream.
	ctx := context.Background()
	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "dead-grain"}
	rec := activationRecord{
		MemberID:   "dead-member-id",
		PidID:      "TestKind/dead-grain",
		PidAddress: "dead-host:9999",
	}
	data, _ := json.Marshal(&rec)
	subject := il.identitySubject(ci)
	_, err := il.provider.js.Publish(ctx, subject, data)
	require.NoError(t, err)

	// "dead-member-id" is not in the member list.
	result, err := il.Peek(ci)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusMemberDead, result.Status)
	assert.Equal(t, "dead-grain", result.Identity)
}

func TestNatsStream_Peek_Stale(t *testing.T) {
	p, c, il := setupPlacementTestCluster(t, "peek-stale")

	ci := &cluster.ClusterIdentity{Kind: "TestKind", Identity: "peek-stale-1"}
	pid := il.Get(ci)
	require.NotNil(t, pid)

	// Stop grain directly, bypassing identity cleanup.
	c.ActorSystem.Root.Poison(pid)
	time.Sleep(500 * time.Millisecond)

	result, err := il.Peek(ci)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusStale, result.Status)

	_ = p
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test -v -race -run "TestNatsStream_Peek" ./cluster/clusterproviders/natsstream/
```

Expected: FAIL — panics with "not implemented".

- [ ] **Step 3: Implement Peek for natsstream**

Replace the stub `Peek` method in `cluster/clusterproviders/natsstream/natsstream_identity.go` with:

```go
// Peek checks if a grain activation exists without triggering activation.
func (il *IdentityLookup) Peek(clusterIdentity *cluster.ClusterIdentity) (*cluster.PeekResult, error) {
	notFound := &cluster.PeekResult{
		GrainInfo: &cluster.GrainInfo{
			Identity: clusterIdentity.Identity,
			Kind:     clusterIdentity.Kind,
		},
		Status: cluster.PeekStatusNotFound,
	}

	// Step 1: Check for existing activation in NATS stream.
	ctx := context.Background()
	existing := il.getExistingActivation(ctx, clusterIdentity)
	if existing == nil {
		return notFound, nil
	}

	// Step 2: Validate owning member is alive.
	if !cluster.ValidateActivationMember(il.cluster.MemberList, existing.MemberID) {
		return &cluster.PeekResult{
			GrainInfo: &cluster.GrainInfo{
				Identity: clusterIdentity.Identity,
				Kind:     clusterIdentity.Kind,
				MemberID: existing.MemberID,
			},
			Status: cluster.PeekStatusMemberDead,
		}, nil
	}

	// Step 3: Confirm process alive via placement actor.
	ownerAddress := existing.PidAddress
	proxyPID := actor.NewPID(ownerAddress, "$proxy-activator")
	future := il.cluster.ActorSystem.Root.RequestFuture(proxyPID, &cluster.PeekRequest{
		ClusterIdentity: clusterIdentity,
	}, 5*time.Second)

	res, err := future.Result()
	if err != nil {
		return nil, fmt.Errorf("peek request to %s failed: %w", ownerAddress, err)
	}

	peekResp, ok := res.(*cluster.PeekResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", res)
	}

	if peekResp.Found {
		return &cluster.PeekResult{
			GrainInfo: &cluster.GrainInfo{
				Identity: clusterIdentity.Identity,
				Kind:     clusterIdentity.Kind,
				PID:      peekResp.Pid,
				MemberID: existing.MemberID,
			},
			Status: cluster.PeekStatusAlive,
		}, nil
	}

	return &cluster.PeekResult{
		GrainInfo: &cluster.GrainInfo{
			Identity: clusterIdentity.Identity,
			Kind:     clusterIdentity.Kind,
			MemberID: existing.MemberID,
		},
		Status: cluster.PeekStatusStale,
	}, nil
}
```

Add `"context"` and `"fmt"` to the import block if not already present.

- [ ] **Step 4: Run tests**

```bash
go test -v -race -run "TestNatsStream_Peek" ./cluster/clusterproviders/natsstream/
```

Expected: all 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add cluster/clusterproviders/natsstream/natsstream_identity.go cluster/clusterproviders/natsstream/natsstream_identity_test.go
git commit -m "feat(natsstream): implement Peek for non-activating grain liveness check"
```

---

### Task 8: Storage-based Peek Implementation

**Files:**
- Modify: `cluster/identitylookup/storage/identity_storage_lookup.go`
- Modify: `cluster/identitylookup/storage/identity_storage_lookup_test.go`

- [ ] **Step 1: Write tests for storage-based Peek**

Add to `cluster/identitylookup/storage/identity_storage_lookup_test.go`:

```go
func TestStorageLookup_Peek_Alive(t *testing.T) {
	c, isl, _ := setupTestCluster(t)

	// Activate a grain.
	ci := &cluster.ClusterIdentity{Kind: testKind, Identity: "peek-alive-1"}
	pid := isl.Get(ci)
	require.NotNil(t, pid)

	// Peek should return Alive.
	result, err := isl.Peek(ci)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusAlive, result.Status)
	assert.Equal(t, "peek-alive-1", result.Identity)
	assert.Equal(t, testKind, result.Kind)
	assert.Equal(t, pid, result.PID)

	_ = c
}

func TestStorageLookup_Peek_NotFound(t *testing.T) {
	c, isl, _ := setupTestCluster(t)

	ci := &cluster.ClusterIdentity{Kind: testKind, Identity: "never-activated"}
	result, err := isl.Peek(ci)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusNotFound, result.Status)

	_ = c
}

func TestStorageLookup_Peek_MemberDead(t *testing.T) {
	c, isl, storageLookup := setupTestCluster(t)

	// Manually insert a stored activation for a dead member.
	ci := &cluster.ClusterIdentity{Kind: testKind, Identity: "dead-member-grain"}
	lock := storageLookup.TryAcquireLock(ci)
	require.NotNil(t, lock)

	fakePID := actor.NewPID("dead-host:9999", testKind+"/dead-member-grain")
	storageLookup.StoreActivation("dead-member-id", lock, fakePID)

	// "dead-member-id" is not in the member list.
	result, err := isl.Peek(ci)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusMemberDead, result.Status)
	assert.Equal(t, "dead-member-grain", result.Identity)

	_ = c
}

func TestStorageLookup_Peek_Stale(t *testing.T) {
	c, isl, _ := setupTestCluster(t)

	// Activate a grain.
	ci := &cluster.ClusterIdentity{Kind: testKind, Identity: "peek-stale-1"}
	pid := isl.Get(ci)
	require.NotNil(t, pid)

	// Stop the grain directly, bypassing storage cleanup.
	c.ActorSystem.Root.Poison(pid)
	time.Sleep(500 * time.Millisecond)

	// Storage record still exists but placement actor no longer has it.
	result, err := isl.Peek(ci)
	require.NoError(t, err)
	assert.Equal(t, cluster.PeekStatusStale, result.Status)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test -v -race -run "TestStorageLookup_Peek" ./cluster/identitylookup/storage/
```

Expected: FAIL — panics with "not implemented".

- [ ] **Step 3: Implement Peek for IdentityStorageLookup**

Replace the stub `Peek` method in `cluster/identitylookup/storage/identity_storage_lookup.go` with:

```go
// Peek checks if a grain activation exists without triggering activation.
func (l *IdentityStorageLookup) Peek(clusterIdentity *cluster.ClusterIdentity) (*cluster.PeekResult, error) {
	notFound := &cluster.PeekResult{
		GrainInfo: &cluster.GrainInfo{
			Identity: clusterIdentity.Identity,
			Kind:     clusterIdentity.Kind,
		},
		Status: cluster.PeekStatusNotFound,
	}

	// Step 1: Check for existing activation in storage.
	existing := l.storage.TryGetExistingActivation(clusterIdentity)
	if existing == nil {
		return notFound, nil
	}

	// Step 2: Validate owning member is alive.
	if !cluster.ValidateActivationMember(l.cluster.MemberList, existing.MemberID) {
		return &cluster.PeekResult{
			GrainInfo: &cluster.GrainInfo{
				Identity: clusterIdentity.Identity,
				Kind:     clusterIdentity.Kind,
				MemberID: existing.MemberID,
			},
			Status: cluster.PeekStatusMemberDead,
		}, nil
	}

	// Step 3: Confirm process alive via placement actor.
	pid := l.pidFromStored(existing)
	ownerAddress := pid.Address
	proxyPID := actor.NewPID(ownerAddress, "$proxy-activator")
	future := l.cluster.ActorSystem.Root.RequestFuture(proxyPID, &cluster.PeekRequest{
		ClusterIdentity: clusterIdentity,
	}, 5*time.Second)

	res, err := future.Result()
	if err != nil {
		return nil, fmt.Errorf("peek request to %s failed: %w", ownerAddress, err)
	}

	peekResp, ok := res.(*cluster.PeekResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", res)
	}

	if peekResp.Found {
		return &cluster.PeekResult{
			GrainInfo: &cluster.GrainInfo{
				Identity: clusterIdentity.Identity,
				Kind:     clusterIdentity.Kind,
				PID:      peekResp.Pid,
				MemberID: existing.MemberID,
			},
			Status: cluster.PeekStatusAlive,
		}, nil
	}

	return &cluster.PeekResult{
		GrainInfo: &cluster.GrainInfo{
			Identity: clusterIdentity.Identity,
			Kind:     clusterIdentity.Kind,
			MemberID: existing.MemberID,
		},
		Status: cluster.PeekStatusStale,
	}, nil
}
```

Add `"fmt"` to the import block if not already present.

- [ ] **Step 4: Run tests**

```bash
go test -v -race -run "TestStorageLookup_Peek" ./cluster/identitylookup/storage/
```

Expected: all 4 tests PASS.

- [ ] **Step 5: Run the full test suite**

```bash
go test -race ./cluster/... ./cluster/identitylookup/... ./cluster/clusterproviders/natskv/... ./cluster/clusterproviders/natsstream/...
```

Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add cluster/identitylookup/storage/identity_storage_lookup.go cluster/identitylookup/storage/identity_storage_lookup_test.go
git commit -m "feat(storage): implement Peek for non-activating grain liveness check"
```
