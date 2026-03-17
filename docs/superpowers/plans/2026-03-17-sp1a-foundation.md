# Sub-project 1a: Foundation — Proto Update, Kind Changes, Utilities

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the proto message changes, Kind builder methods, sentinel errors, and utility functions that all subsequent sub-projects depend on.

**Architecture:** Update `ActivationResponse` proto to add `invalid_identity` field, add `CanSpawnIdentity` and `ActivatorStrategy` fields to `Kind`/`ActivatedKind`, add `LockNotHeld` sentinel error, and add `ValidateActivationMember` utility. All changes are additive — no existing behavior changes.

**Tech Stack:** Go, protobuf (protoc), testify

**Spec:** `docs/superpowers/specs/2026-03-17-shared-placement-actor-design.md`
**Tracker:** `docs/superpowers/plans/shared-placement-actor-tracker.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `cluster/cluster.proto` | Modify (line 94-98) | Add `invalid_identity` field to `ActivationResponse` |
| `cluster/cluster.pb.go` | Regenerate | Generated from proto |
| `cluster/kind.go` | Modify | Add `CanSpawnIdentity`, `ActivatorStrategyBuilder` fields; add builder methods |
| `cluster/kind_test.go` | Create | Tests for new Kind builder methods |
| `cluster/config_opts.go` | Modify | Add `WithDefaultActivatorStrategy` option |
| `cluster/config.go` | Modify | Add `DefaultActivatorStrategy` field to Config |
| `cluster/errors.go` | Create | `ErrLockNotHeld` sentinel error |
| `cluster/errors_test.go` | Create | Test sentinel error semantics |
| `cluster/activation_utils.go` | Create | `ValidateActivationMember` utility |
| `cluster/activation_utils_test.go` | Create | Tests for member validation |

---

### Task 1: Add `invalid_identity` to ActivationResponse proto

**Files:**
- Modify: `cluster/cluster.proto:94-98`
- Regenerate: `cluster/cluster.pb.go`

- [ ] **Step 1: Update the proto definition**

In `cluster/cluster.proto`, change the `ActivationResponse` message from:

```protobuf
message ActivationResponse {
  actor.PID pid = 1;
  bool failed = 2;
  uint64 topology_hash = 3;
}
```

To:

```protobuf
message ActivationResponse {
  actor.PID pid = 1;
  bool failed = 2;
  uint64 topology_hash = 3;
  bool invalid_identity = 4;
}
```

- [ ] **Step 2: Regenerate protobuf**

Run: `cd /home/cchamplin/development/protoactor-go/cluster && bash build.sh`

Verify the generated file has the new field:
Run: `grep -n 'InvalidIdentity' /home/cchamplin/development/protoactor-go/cluster/cluster.pb.go | head -5`

Expected: Lines with `InvalidIdentity bool` field and getter method.

- [ ] **Step 3: Verify existing tests still pass**

Run: `cd /home/cchamplin/development/protoactor-go && go test -count=1 ./cluster/...`

Expected: All PASS (additive proto change, no breakage).

- [ ] **Step 4: Commit**

```bash
cd /home/cchamplin/development/protoactor-go
git add cluster/cluster.proto cluster/cluster.pb.go
git commit -m "proto(cluster): add invalid_identity field to ActivationResponse

Needed by the shared placement actor to signal that a CanSpawnIdentity
predicate rejected the identity. This is an additive proto change —
no existing behavior is affected.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Add `ErrLockNotHeld` sentinel error

**Files:**
- Create: `cluster/errors.go`
- Create: `cluster/errors_test.go`

- [ ] **Step 1: Write the failing test**

Create `cluster/errors_test.go`:

```go
package cluster

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrLockNotHeld_IsSentinel(t *testing.T) {
	// ErrLockNotHeld should be matchable with errors.Is.
	err := fmt.Errorf("persistence failed: %w", ErrLockNotHeld)
	assert.True(t, errors.Is(err, ErrLockNotHeld),
		"ErrLockNotHeld must be usable as a sentinel with errors.Is")
}

func TestErrLockNotHeld_NotMatchOther(t *testing.T) {
	other := errors.New("some other error")
	assert.False(t, errors.Is(other, ErrLockNotHeld))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run TestErrLockNotHeld -v -count=1 ./cluster/`

Expected: FAIL — `ErrLockNotHeld` not defined.

- [ ] **Step 3: Implement the sentinel error**

Create `cluster/errors.go`:

```go
package cluster

import "errors"

// ErrLockNotHeld is returned by PersistActivation callbacks when the spawn
// lock has been stolen by another node. The placement actor should NOT retry
// — the lock is irrecoverably lost. The spawned actor is poisoned immediately.
var ErrLockNotHeld = errors.New("spawn lock is no longer held")
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run TestErrLockNotHeld -v -count=1 ./cluster/`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cchamplin/development/protoactor-go
git add cluster/errors.go cluster/errors_test.go
git commit -m "feat(cluster): add ErrLockNotHeld sentinel error

Used by PersistActivation callbacks to signal that the spawn lock was
stolen, so the placement actor should not retry persistence.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Add `CanSpawnIdentity` and `ActivatorStrategyBuilder` to Kind

**Files:**
- Modify: `cluster/kind.go`
- Create: `cluster/kind_test.go`

- [ ] **Step 1: Write the failing tests**

Create `cluster/kind_test.go`:

```go
package cluster

import (
	"context"
	"testing"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewKind_Defaults(t *testing.T) {
	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	k := NewKind("TestKind", props)

	assert.Equal(t, "TestKind", k.Kind)
	assert.NotNil(t, k.Props)
	assert.Nil(t, k.StrategyBuilder)
	assert.Nil(t, k.CanSpawnIdentity)
	assert.Nil(t, k.ActivatorStrategyBuilder)
}

func TestKind_WithCanSpawnIdentity(t *testing.T) {
	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	predicate := func(ctx context.Context, identity string) (bool, error) {
		return identity != "blocked", nil
	}

	k := NewKind("TestKind", props).WithCanSpawnIdentity(predicate)

	require.NotNil(t, k.CanSpawnIdentity)
	ok, err := k.CanSpawnIdentity(context.Background(), "allowed")
	assert.NoError(t, err)
	assert.True(t, ok)

	ok, err = k.CanSpawnIdentity(context.Background(), "blocked")
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestKind_WithActivatorStrategy(t *testing.T) {
	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	builder := func(c *Cluster) ActivatorStrategy { return nil }

	k := NewKind("TestKind", props).WithActivatorStrategy(builder)

	require.NotNil(t, k.ActivatorStrategyBuilder)
}

func TestKind_BuildPreservesNewFields(t *testing.T) {
	props := actor.PropsFromFunc(func(ctx actor.Context) {})
	predicate := func(ctx context.Context, identity string) (bool, error) {
		return true, nil
	}

	k := NewKind("TestKind", props).WithCanSpawnIdentity(predicate)

	// Build with nil cluster (no strategy builder set).
	ak := k.Build(nil)
	assert.Equal(t, "TestKind", ak.Kind)
	assert.NotNil(t, ak.CanSpawnIdentity)

	ok, err := ak.CanSpawnIdentity(context.Background(), "any")
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestKind_Chaining(t *testing.T) {
	props := actor.PropsFromFunc(func(ctx actor.Context) {})

	// All builder methods should return *Kind for chaining.
	k := NewKind("TestKind", props).
		WithCanSpawnIdentity(func(ctx context.Context, id string) (bool, error) { return true, nil }).
		WithActivatorStrategy(func(c *Cluster) ActivatorStrategy { return nil })

	assert.NotNil(t, k.CanSpawnIdentity)
	assert.NotNil(t, k.ActivatorStrategyBuilder)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run 'TestNewKind_Defaults|TestKind_With|TestKind_Build|TestKind_Chaining' -v -count=1 ./cluster/`

Expected: FAIL — fields and methods don't exist yet.

- [ ] **Step 3: Implement Kind changes**

Modify `cluster/kind.go`. The full replacement:

```go
package cluster

import (
	"context"
	"sync/atomic"

	"github.com/asynkron/protoactor-go/actor"
)

// Kind represents the kinds of actors a cluster can manage
type Kind struct {
	Kind                    string
	Props                   *actor.Props
	StrategyBuilder         func(*Cluster) MemberStrategy
	CanSpawnIdentity        func(ctx context.Context, identity string) (bool, error)
	ActivatorStrategyBuilder func(*Cluster) ActivatorStrategy
}

// NewKind creates a new instance of a kind
func NewKind(kind string, props *actor.Props) *Kind {
	// add cluster middleware
	p := props.Clone(withClusterReceiveMiddleware())
	return &Kind{
		Kind:            kind,
		Props:           p,
		StrategyBuilder: nil,
	}
}

func (k *Kind) WithMemberStrategy(strategyBuilder func(*Cluster) MemberStrategy) *Kind {
	k.StrategyBuilder = strategyBuilder
	return k
}

// WithCanSpawnIdentity sets an optional predicate that is called before
// spawning a grain. If it returns false, the activation is rejected with
// InvalidIdentity. The context allows async implementations (e.g., DB lookups).
func (k *Kind) WithCanSpawnIdentity(fn func(ctx context.Context, identity string) (bool, error)) *Kind {
	k.CanSpawnIdentity = fn
	return k
}

// WithActivatorStrategy sets the placement strategy builder for this kind.
// The builder receives the Cluster and returns an ActivatorStrategy that
// selects which member should host new grain activations for this kind.
func (k *Kind) WithActivatorStrategy(builder func(*Cluster) ActivatorStrategy) *Kind {
	k.ActivatorStrategyBuilder = builder
	return k
}

func (k *Kind) Build(cluster *Cluster) *ActivatedKind {
	var strategy MemberStrategy = nil
	if k.StrategyBuilder != nil {
		strategy = k.StrategyBuilder(cluster)
	}

	var activatorStrategy ActivatorStrategy = nil
	if k.ActivatorStrategyBuilder != nil && cluster != nil {
		activatorStrategy = k.ActivatorStrategyBuilder(cluster)
	}

	return &ActivatedKind{
		Kind:              k.Kind,
		Props:             k.Props,
		Strategy:          strategy,
		ActivatorStrategy: activatorStrategy,
		CanSpawnIdentity:  k.CanSpawnIdentity,
	}
}

type ActivatedKind struct {
	Kind              string
	Props             *actor.Props
	Strategy          MemberStrategy
	ActivatorStrategy ActivatorStrategy
	CanSpawnIdentity  func(ctx context.Context, identity string) (bool, error)
	count             int32
}

func (ak *ActivatedKind) Inc() {
	atomic.AddInt32(&ak.count, 1)
}

func (ak *ActivatedKind) Dec() {
	atomic.AddInt32(&ak.count, -1)
}

func (ak *ActivatedKind) Count() int32 {
	return atomic.LoadInt32(&ak.count)
}
```

**IMPORTANT:** The `WithMemberStrategy` return type changes from `void` to `*Kind` for chaining. This is a minor API change — callers that used it as a statement (`k.WithMemberStrategy(b)`) still work. Callers that stored the result (`_ = k.WithMemberStrategy(b)`) also work.

Also, we need to define the `ActivatorStrategy` interface. Create a stub in a new file `cluster/activator_strategy.go`:

```go
package cluster

// ActivatorStrategy selects which cluster member should host a new grain.
// This is distinct from the existing MemberStrategy interface which handles
// partition routing. ActivatorStrategy is specifically for placement decisions
// when spawning new grains via storage-backed identity providers.
type ActivatorStrategy interface {
	// GetActivator returns the member that should spawn the given identity.
	// senderAddress is the address of the node that received the original
	// request. Returns nil if no suitable member is available.
	GetActivator(ci *ClusterIdentity, senderAddress string) *Member

	// AddMember is called when a member joins the cluster.
	AddMember(member *Member)

	// RemoveMember is called when a member leaves the cluster.
	RemoveMember(member *Member)

	// Close releases resources (e.g., unsubscribes from EventStream).
	Close()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run 'TestNewKind_Defaults|TestKind_With|TestKind_Build|TestKind_Chaining' -v -count=1 ./cluster/`

Expected: All PASS.

- [ ] **Step 5: Run all cluster tests for regressions**

Run: `cd /home/cchamplin/development/protoactor-go && go test -count=1 -race ./cluster/`

Expected: All PASS. The `WithMemberStrategy` return type change should not break existing callers.

- [ ] **Step 6: Commit**

```bash
cd /home/cchamplin/development/protoactor-go
git add cluster/kind.go cluster/kind_test.go cluster/activator_strategy.go
git commit -m "feat(cluster): add CanSpawnIdentity, ActivatorStrategy to Kind

- Add CanSpawnIdentity field (async predicate for spawn verification)
- Add ActivatorStrategyBuilder field (per-kind placement strategy)
- Add WithCanSpawnIdentity() and WithActivatorStrategy() builder methods
- Define ActivatorStrategy interface (stub, implementations in sub-project 1b)
- Change WithMemberStrategy to return *Kind for chaining (non-breaking)
- ActivatedKind.Build() wires both new fields through

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Add `WithDefaultActivatorStrategy` config option

**Files:**
- Modify: `cluster/config.go`
- Modify: `cluster/config_opts.go`

- [ ] **Step 1: Add field to Config and option**

In `cluster/config.go`, add after the `MemberStrategyBuilder` field (line 24):

```go
	DefaultActivatorStrategy func(*Cluster) ActivatorStrategy
```

In `cluster/config_opts.go`, add at the end:

```go
// WithDefaultActivatorStrategy sets the default placement strategy for kinds
// that don't specify their own via WithActivatorStrategy. If not set,
// RoundRobinStrategy is used (once implemented in sub-project 1b).
func WithDefaultActivatorStrategy(builder func(*Cluster) ActivatorStrategy) ConfigOption {
	return func(c *Config) {
		c.DefaultActivatorStrategy = builder
	}
}
```

- [ ] **Step 2: Verify build and tests**

Run: `cd /home/cchamplin/development/protoactor-go && go build ./cluster/ && go test -count=1 ./cluster/`

Expected: Build succeeds, all tests PASS.

- [ ] **Step 3: Commit**

```bash
cd /home/cchamplin/development/protoactor-go
git add cluster/config.go cluster/config_opts.go
git commit -m "feat(cluster): add WithDefaultActivatorStrategy config option

Sets the default placement strategy for kinds that don't specify
their own. Used by the shared placement actor to select which member
should host new grain activations.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Add `ValidateActivationMember` utility

**Files:**
- Create: `cluster/activation_utils.go`
- Create: `cluster/activation_utils_test.go`

- [ ] **Step 1: Write the failing tests**

Create `cluster/activation_utils_test.go`:

```go
package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateActivationMember_MemberAlive(t *testing.T) {
	c := newClusterForTest("test-validate", newInmemoryProvider())
	err := c.StartMember()
	require.NoError(t, err)
	t.Cleanup(func() { c.Shutdown(true) })

	// The cluster's own member should be alive.
	host, port, _ := c.ActorSystem.GetHostPort()
	selfID := fmt.Sprintf("%s@%s:%d", "test-validate", host, port)
	assert.True(t, ValidateActivationMember(c.MemberList, selfID),
		"should return true for alive member")
}

func TestValidateActivationMember_MemberDead(t *testing.T) {
	c := newClusterForTest("test-validate-dead", newInmemoryProvider())
	err := c.StartMember()
	require.NoError(t, err)
	t.Cleanup(func() { c.Shutdown(true) })

	assert.False(t, ValidateActivationMember(c.MemberList, "nonexistent-member-id"),
		"should return false for unknown member")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run TestValidateActivationMember -v -count=1 ./cluster/`

Expected: FAIL — function not defined.

- [ ] **Step 3: Implement the utility**

Create `cluster/activation_utils.go`:

```go
package cluster

// ValidateActivationMember checks whether a member ID is still present in
// the cluster's current topology. Used by identity lookups to detect stale
// activation records left by departed members.
//
// This matches the .NET IdentityStorageWorker.ValidateAndMapToPid() pattern:
// before returning a PID from a stored activation, verify the owning member
// is still alive.
func ValidateActivationMember(ml *MemberList, memberID string) bool {
	return ml.ContainsMemberID(memberID)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run TestValidateActivationMember -v -count=1 ./cluster/`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cchamplin/development/protoactor-go
git add cluster/activation_utils.go cluster/activation_utils_test.go
git commit -m "feat(cluster): add ValidateActivationMember utility

Checks whether a member ID is still present in the cluster topology.
Used by identity lookups to detect stale activation records left by
departed members before returning their PIDs.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: Final verification

- [ ] **Step 1: Run all cluster tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test -race -count=1 ./cluster/...`

Expected: All PASS.

- [ ] **Step 2: Run all root module tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test -race -count=1 ./...`

Expected: All PASS (no regressions from additive changes).

- [ ] **Step 3: Update tracker**

Edit `docs/superpowers/plans/shared-placement-actor-tracker.md`:
- Change Sub-project 1a Plan Status to `Complete`
- Change Sub-project 1a Execution Status to `Complete`
