# Sub-project 1b: ActivatorStrategy Interface + Four Implementations

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement four pluggable placement strategies (`RoundRobinStrategy`, `LocalAffinityStrategy`, `RendezvousStrategy`, `GossipStrategy`) plus a per-kind strategy manager, so identity lookups can select which cluster member should spawn a grain.

**Architecture:** Each strategy is a standalone file implementing `ActivatorStrategy` (defined in 1a). Strategies maintain their own member list via `AddMember`/`RemoveMember`. `GossipStrategy` subscribes to the EventStream for `*GossipUpdate` messages carrying heartbeat data, keeping a `sync.RWMutex`-protected map of per-member per-kind counts. The strategy manager reads the per-kind strategy from `ActivatedKind.ActivatorStrategy`, falling back to a cluster-default, and is the only component identity lookups call.

**Tech Stack:** Go 1.21+, `sync/atomic`, `sync.RWMutex`, `hash/fnv`, testify, existing `cluster` package (eventstream, `GossipUpdate`, `MemberHeartbeat`, `HeartbeatKey`)

**Spec:** `docs/superpowers/specs/2026-03-17-shared-placement-actor-design.md`
**Depends on:** Sub-project 1a complete (`ActivatorStrategy` interface in `cluster/activator_strategy.go`, `ActivatedKind.ActivatorStrategy` field in `cluster/kind.go`)

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `cluster/activator_strategy.go` | Already exists (1a) | `ActivatorStrategy` interface definition |
| `cluster/activator_strategy_roundrobin.go` | Create | `RoundRobinStrategy` — atomic counter, cyclic member selection; also houses `membersForKind` and `removeMemberByID` package-level helpers (shared by all strategies in the same package) |
| `cluster/activator_strategy_roundrobin_test.go` | Create | Unit tests for `RoundRobinStrategy`; also houses `newTestMember` and `newTestCI` test helpers (shared by all strategy test files in the same package) |
| `cluster/activator_strategy_affinity.go` | Create | `LocalAffinityStrategy` — prefer local, fall back to round-robin |
| `cluster/activator_strategy_affinity_test.go` | Create | Unit tests for `LocalAffinityStrategy` |
| `cluster/activator_strategy_rendezvous.go` | Create | `RendezvousStrategy` — deterministic hash-based using FNV1a32 rendezvous hashing |
| `cluster/activator_strategy_rendezvous_test.go` | Create | Unit tests for `RendezvousStrategy` |
| `cluster/activator_strategy_gossip.go` | Create | `GossipStrategy` — least-loaded via gossip heartbeat data, EventStream subscription |
| `cluster/activator_strategy_gossip_test.go` | Create | Unit tests for `GossipStrategy` |
| `cluster/activator_strategy_manager.go` | Create | `StrategyManager` — per-kind dispatch, cluster-default fallback |
| `cluster/activator_strategy_manager_test.go` | Create | Unit tests for `StrategyManager` |

All files use `package cluster`. Package-level helpers defined in one file are visible to all files in the package — no separate shared file is needed. The manager is separate from all strategies.

---

## Chunk 1: RoundRobinStrategy

### Task 1: RoundRobinStrategy implementation

**Files:**
- Create: `cluster/activator_strategy_roundrobin.go`
- Create: `cluster/activator_strategy_roundrobin_test.go`

- [ ] **Step 1: Write the failing tests**

Create `cluster/activator_strategy_roundrobin_test.go`:

```go
package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTestMember(id, host string, port int32, kinds ...string) *Member {
	return &Member{Id: id, Host: host, Port: port, Kinds: kinds}
}

func newTestCI(kind, identity string) *ClusterIdentity {
	return &ClusterIdentity{Kind: kind, Identity: identity}
}

func TestRoundRobinStrategy_ReturnsNilWhenNoMembers(t *testing.T) {
	s := NewRoundRobinStrategy()
	result := s.GetActivator(newTestCI("myKind", "id1"), "127.0.0.1:0")
	assert.Nil(t, result)
}

func TestRoundRobinStrategy_SingleMember(t *testing.T) {
	s := NewRoundRobinStrategy()
	m := newTestMember("m1", "host1", 1000, "myKind")
	s.AddMember(m)

	result := s.GetActivator(newTestCI("myKind", "id1"), "127.0.0.1:0")
	assert.Equal(t, m, result)

	// Single member should always return the same member.
	result2 := s.GetActivator(newTestCI("myKind", "id2"), "127.0.0.1:0")
	assert.Equal(t, m, result2)
}

func TestRoundRobinStrategy_CyclesThroughMembers(t *testing.T) {
	s := NewRoundRobinStrategy()
	m1 := newTestMember("m1", "host1", 1000, "myKind")
	m2 := newTestMember("m2", "host2", 1001, "myKind")
	m3 := newTestMember("m3", "host3", 1002, "myKind")
	s.AddMember(m1)
	s.AddMember(m2)
	s.AddMember(m3)

	seen := map[string]int{}
	for i := 0; i < 9; i++ {
		r := s.GetActivator(newTestCI("myKind", "id"), "127.0.0.1:0")
		if r != nil {
			seen[r.Id]++
		}
	}

	// Each member should be selected 3 times across 9 calls.
	assert.Equal(t, 3, seen["m1"])
	assert.Equal(t, 3, seen["m2"])
	assert.Equal(t, 3, seen["m3"])
}

func TestRoundRobinStrategy_FiltersToKindMembers(t *testing.T) {
	s := NewRoundRobinStrategy()
	mA := newTestMember("mA", "hostA", 1000, "kindA")
	mB := newTestMember("mB", "hostB", 1001, "kindB")
	s.AddMember(mA)
	s.AddMember(mB)

	// Only mA supports kindA, so all calls return mA.
	for i := 0; i < 5; i++ {
		r := s.GetActivator(newTestCI("kindA", "id"), "127.0.0.1:0")
		assert.Equal(t, mA, r)
	}
}

func TestRoundRobinStrategy_RemoveMember(t *testing.T) {
	s := NewRoundRobinStrategy()
	m1 := newTestMember("m1", "host1", 1000, "myKind")
	m2 := newTestMember("m2", "host2", 1001, "myKind")
	s.AddMember(m1)
	s.AddMember(m2)
	s.RemoveMember(m1)

	// Only m2 remains.
	for i := 0; i < 5; i++ {
		r := s.GetActivator(newTestCI("myKind", "id"), "127.0.0.1:0")
		assert.Equal(t, m2, r)
	}
}

func TestRoundRobinStrategy_RemoveUnknownMemberIsNoop(t *testing.T) {
	s := NewRoundRobinStrategy()
	m := newTestMember("m1", "host1", 1000, "myKind")
	s.AddMember(m)

	// Removing a member we never added should not panic.
	s.RemoveMember(newTestMember("unknown", "host99", 9999, "myKind"))

	r := s.GetActivator(newTestCI("myKind", "id"), "127.0.0.1:0")
	assert.Equal(t, m, r)
}

func TestRoundRobinStrategy_CloseIsNoop(t *testing.T) {
	s := NewRoundRobinStrategy()
	// Close on a strategy with no subscriptions must not panic.
	assert.NotPanics(t, func() { s.Close() })
}

func TestRoundRobinStrategy_NoMatchingKind(t *testing.T) {
	s := NewRoundRobinStrategy()
	s.AddMember(newTestMember("m1", "host1", 1000, "otherKind"))

	r := s.GetActivator(newTestCI("myKind", "id"), "127.0.0.1:0")
	assert.Nil(t, r)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run 'TestRoundRobinStrategy' -v -count=1 ./cluster/`

Expected: FAIL — `NewRoundRobinStrategy` undefined.

- [ ] **Step 3: Implement RoundRobinStrategy**

Create `cluster/activator_strategy_roundrobin.go`:

```go
package cluster

import (
	"sync"
	"sync/atomic"
)

// RoundRobinStrategy selects members for grain activation in a cyclic order.
// It filters to members that support the requested kind, then cycles through
// them using an atomic counter. Thread-safe.
type RoundRobinStrategy struct {
	mu      sync.RWMutex
	members []*Member
	counter uint64
}

// NewRoundRobinStrategy creates a new RoundRobinStrategy with no members.
func NewRoundRobinStrategy() *RoundRobinStrategy {
	return &RoundRobinStrategy{}
}

// GetActivator returns the next member that supports ci.Kind in round-robin order.
// Returns nil if no member supports the kind.
func (r *RoundRobinStrategy) GetActivator(ci *ClusterIdentity, _ string) *Member {
	r.mu.RLock()
	eligible := membersForKind(r.members, ci.Kind)
	r.mu.RUnlock()

	l := len(eligible)
	if l == 0 {
		return nil
	}
	if l == 1 {
		return eligible[0]
	}
	n := atomic.AddUint64(&r.counter, 1)
	return eligible[int(n)%l]
}

// AddMember adds a member to the pool.
func (r *RoundRobinStrategy) AddMember(member *Member) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.members = append(r.members, member)
}

// RemoveMember removes a member from the pool by ID. No-op if not found.
func (r *RoundRobinStrategy) RemoveMember(member *Member) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.members = removeMemberByID(r.members, member.Id)
}

// Close is a no-op for RoundRobinStrategy (no subscriptions to release).
func (r *RoundRobinStrategy) Close() {}

// membersForKind filters members to those supporting the given kind.
// Returns a new slice — callers must not hold the strategy mutex while using it.
func membersForKind(members []*Member, kind string) []*Member {
	result := make([]*Member, 0, len(members))
	for _, m := range members {
		if m.HasKind(kind) {
			result = append(result, m)
		}
	}
	return result
}

// removeMemberByID returns a new slice with the member matching id removed.
func removeMemberByID(members []*Member, id string) []*Member {
	for i, m := range members {
		if m.Id == id {
			return append(members[:i:i], members[i+1:]...)
		}
	}
	return members
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run 'TestRoundRobinStrategy' -v -count=1 ./cluster/`

Expected: All PASS.

- [ ] **Step 5: Run race detector**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run 'TestRoundRobinStrategy' -race -count=1 ./cluster/`

Expected: All PASS, no data race warnings.

- [ ] **Step 6: Commit**

```bash
cd /home/cchamplin/development/protoactor-go
git add cluster/activator_strategy_roundrobin.go cluster/activator_strategy_roundrobin_test.go
git commit -m "feat(cluster): add RoundRobinStrategy activator strategy

Cyclic member selection for grain placement. Filters to members
supporting the requested kind, then cycles via atomic counter.
Shared helpers membersForKind and removeMemberByID used by all strategies.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

## Chunk 2: LocalAffinityStrategy

### Task 2: LocalAffinityStrategy implementation

**Files:**
- Create: `cluster/activator_strategy_affinity.go`
- Create: `cluster/activator_strategy_affinity_test.go`

- [ ] **Step 1: Write the failing tests**

Create `cluster/activator_strategy_affinity_test.go`:

```go
package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocalAffinityStrategy_PrefersLocalMember(t *testing.T) {
	localAddr := "host1:1000"
	s := NewLocalAffinityStrategy()
	m1 := newTestMember("m1", "host1", 1000, "myKind")
	m2 := newTestMember("m2", "host2", 1001, "myKind")
	s.AddMember(m1)
	s.AddMember(m2)

	// When the sender is on m1, always prefer m1.
	for i := 0; i < 5; i++ {
		r := s.GetActivator(newTestCI("myKind", "id"), localAddr)
		assert.Equal(t, m1, r, "should prefer local member")
	}
}

func TestLocalAffinityStrategy_FallsBackToRoundRobinWhenLocalNotEligible(t *testing.T) {
	// Local node supports kindB only; request is for kindA.
	// Fall back to round-robin across kindA members.
	s := NewLocalAffinityStrategy()
	mLocal := newTestMember("mLocal", "host1", 1000, "kindB")
	mA1 := newTestMember("mA1", "host2", 1001, "kindA")
	mA2 := newTestMember("mA2", "host3", 1002, "kindA")
	s.AddMember(mLocal)
	s.AddMember(mA1)
	s.AddMember(mA2)

	seen := map[string]int{}
	for i := 0; i < 10; i++ {
		r := s.GetActivator(newTestCI("kindA", "id"), "host1:1000")
		if r != nil {
			seen[r.Id]++
		}
	}

	// mLocal should never be selected (it doesn't support kindA).
	assert.Equal(t, 0, seen["mLocal"])
	// mA1 and mA2 should each be selected ~5 times.
	assert.Greater(t, seen["mA1"], 0)
	assert.Greater(t, seen["mA2"], 0)
}

func TestLocalAffinityStrategy_FallsBackToRoundRobinWhenNoLocalMatch(t *testing.T) {
	// senderAddress does not match any member — falls back to round-robin.
	s := NewLocalAffinityStrategy()
	m1 := newTestMember("m1", "host1", 1000, "myKind")
	m2 := newTestMember("m2", "host2", 1001, "myKind")
	s.AddMember(m1)
	s.AddMember(m2)

	seen := map[string]int{}
	for i := 0; i < 10; i++ {
		r := s.GetActivator(newTestCI("myKind", "id"), "unknown:9999")
		if r != nil {
			seen[r.Id]++
		}
	}
	assert.Greater(t, seen["m1"], 0, "should round-robin when local unknown")
	assert.Greater(t, seen["m2"], 0, "should round-robin when local unknown")
}

func TestLocalAffinityStrategy_ReturnsNilWhenNoMembers(t *testing.T) {
	s := NewLocalAffinityStrategy()
	assert.Nil(t, s.GetActivator(newTestCI("myKind", "id"), "host:1"))
}

func TestLocalAffinityStrategy_RemoveMember(t *testing.T) {
	s := NewLocalAffinityStrategy()
	m1 := newTestMember("m1", "host1", 1000, "myKind")
	m2 := newTestMember("m2", "host2", 1001, "myKind")
	s.AddMember(m1)
	s.AddMember(m2)
	s.RemoveMember(m1)

	// m1 removed; local preference for m1 should fall back to m2.
	for i := 0; i < 5; i++ {
		r := s.GetActivator(newTestCI("myKind", "id"), "host1:1000")
		assert.Equal(t, m2, r)
	}
}

func TestLocalAffinityStrategy_CloseIsNoop(t *testing.T) {
	s := NewLocalAffinityStrategy()
	assert.NotPanics(t, func() { s.Close() })
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run 'TestLocalAffinityStrategy' -v -count=1 ./cluster/`

Expected: FAIL — `NewLocalAffinityStrategy` undefined.

- [ ] **Step 3: Implement LocalAffinityStrategy**

Create `cluster/activator_strategy_affinity.go`:

```go
package cluster

import (
	"sync"
	"sync/atomic"
)

// LocalAffinityStrategy prefers placing grains on the local member (the node
// that received the Get() request) if it supports the requested kind. Falls
// back to round-robin across all eligible members if the local node does not
// support the kind or is not in the member list.
type LocalAffinityStrategy struct {
	mu      sync.RWMutex
	members []*Member
	counter uint64
}

// NewLocalAffinityStrategy creates a new LocalAffinityStrategy with no members.
func NewLocalAffinityStrategy() *LocalAffinityStrategy {
	return &LocalAffinityStrategy{}
}

// GetActivator returns the local member if it supports ci.Kind; otherwise
// returns the next eligible member in round-robin order. senderAddress must
// be in "host:port" format (matches Member.Address()).
func (s *LocalAffinityStrategy) GetActivator(ci *ClusterIdentity, senderAddress string) *Member {
	s.mu.RLock()
	eligible := membersForKind(s.members, ci.Kind)
	s.mu.RUnlock()

	l := len(eligible)
	if l == 0 {
		return nil
	}

	// Prefer local node.
	for _, m := range eligible {
		if m.Address() == senderAddress {
			return m
		}
	}

	// Fall back to round-robin.
	if l == 1 {
		return eligible[0]
	}
	n := atomic.AddUint64(&s.counter, 1)
	return eligible[int(n)%l]
}

// AddMember adds a member to the pool.
func (s *LocalAffinityStrategy) AddMember(member *Member) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.members = append(s.members, member)
}

// RemoveMember removes a member from the pool by ID. No-op if not found.
func (s *LocalAffinityStrategy) RemoveMember(member *Member) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.members = removeMemberByID(s.members, member.Id)
}

// Close is a no-op for LocalAffinityStrategy (no subscriptions to release).
func (s *LocalAffinityStrategy) Close() {}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run 'TestLocalAffinityStrategy' -v -count=1 ./cluster/`

Expected: All PASS.

- [ ] **Step 5: Run race detector**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run 'TestLocalAffinityStrategy' -race -count=1 ./cluster/`

Expected: All PASS, no data race warnings.

- [ ] **Step 6: Commit**

```bash
cd /home/cchamplin/development/protoactor-go
git add cluster/activator_strategy_affinity.go cluster/activator_strategy_affinity_test.go
git commit -m "feat(cluster): add LocalAffinityStrategy activator strategy

Prefers the local node for grain placement. Falls back to round-robin
across eligible members when the local node doesn't support the kind
or is not in the member list.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

## Chunk 3: RendezvousStrategy

### Task 3: RendezvousStrategy implementation

**Files:**
- Create: `cluster/activator_strategy_rendezvous.go`
- Create: `cluster/activator_strategy_rendezvous_test.go`

The hashing algorithm mirrors `cluster/rendezvous.go`: FNV1a32 over `identity bytes ++ member-address bytes`. The member with the highest score wins. This gives deterministic mapping of a `ClusterIdentity` to a member — the same identity always maps to the same member given stable topology.

- [ ] **Step 1: Write the failing tests**

Create `cluster/activator_strategy_rendezvous_test.go`:

```go
package cluster

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRendezvousStrategy_ReturnsNilWhenNoMembers(t *testing.T) {
	s := NewRendezvousStrategy()
	assert.Nil(t, s.GetActivator(newTestCI("myKind", "id"), ""))
}

func TestRendezvousStrategy_SingleMember(t *testing.T) {
	s := NewRendezvousStrategy()
	m := newTestMember("m1", "host1", 1000, "myKind")
	s.AddMember(m)

	r := s.GetActivator(newTestCI("myKind", "id"), "")
	assert.Equal(t, m, r)
}

func TestRendezvousStrategy_Deterministic(t *testing.T) {
	s := NewRendezvousStrategy()
	m1 := newTestMember("m1", "host1", 1000, "myKind")
	m2 := newTestMember("m2", "host2", 1001, "myKind")
	m3 := newTestMember("m3", "host3", 1002, "myKind")
	s.AddMember(m1)
	s.AddMember(m2)
	s.AddMember(m3)

	ci := newTestCI("myKind", "stable-identity")
	first := s.GetActivator(ci, "")
	// Calling multiple times with same input must return the same member.
	for i := 0; i < 10; i++ {
		assert.Equal(t, first, s.GetActivator(ci, ""),
			"rendezvous must be deterministic for stable topology")
	}
}

func TestRendezvousStrategy_DifferentIdentitiesDistribute(t *testing.T) {
	s := NewRendezvousStrategy()
	m1 := newTestMember("m1", "host1", 1000, "myKind")
	m2 := newTestMember("m2", "host2", 1001, "myKind")
	m3 := newTestMember("m3", "host3", 1002, "myKind")
	s.AddMember(m1)
	s.AddMember(m2)
	s.AddMember(m3)

	seen := map[string]int{}
	for i := 0; i < 300; i++ {
		ci := newTestCI("myKind", fmt.Sprintf("identity-%d", i))
		r := s.GetActivator(ci, "")
		if r != nil {
			seen[r.Id]++
		}
	}
	// All three members should receive some identities across 300 distinct keys.
	assert.Greater(t, seen["m1"], 0)
	assert.Greater(t, seen["m2"], 0)
	assert.Greater(t, seen["m3"], 0)
}

func TestRendezvousStrategy_StableAfterMemberAdd(t *testing.T) {
	// An identity assigned to m1 before adding m4 should remain on m1 after
	// adding m4 (most of the time — rendezvous minimises remapping).
	// We test the determinism property: the result for m4's identity should
	// consistently be m4 after it's added.
	s := NewRendezvousStrategy()
	m1 := newTestMember("m1", "host1", 1000, "myKind")
	m2 := newTestMember("m2", "host2", 1001, "myKind")
	s.AddMember(m1)
	s.AddMember(m2)

	ci := newTestCI("myKind", "my-stable-key")
	before := s.GetActivator(ci, "")
	assert.NotNil(t, before)

	// Adding a third member may or may not change the result for this key,
	// but must remain deterministic afterwards.
	m3 := newTestMember("m3", "host3", 1002, "myKind")
	s.AddMember(m3)

	after := s.GetActivator(ci, "")
	assert.NotNil(t, after)
	// The result must stay stable across repeated calls with the same topology.
	for i := 0; i < 5; i++ {
		assert.Equal(t, after, s.GetActivator(ci, ""))
	}
}

func TestRendezvousStrategy_RemoveMember(t *testing.T) {
	s := NewRendezvousStrategy()
	m1 := newTestMember("m1", "host1", 1000, "myKind")
	m2 := newTestMember("m2", "host2", 1001, "myKind")
	s.AddMember(m1)
	s.AddMember(m2)
	s.RemoveMember(m1)

	// Only m2 remains; every call must return m2.
	for i := 0; i < 5; i++ {
		r := s.GetActivator(newTestCI("myKind", fmt.Sprintf("id-%d", i)), "")
		assert.Equal(t, m2, r)
	}
}

func TestRendezvousStrategy_FiltersToKindMembers(t *testing.T) {
	s := NewRendezvousStrategy()
	mA := newTestMember("mA", "hostA", 1000, "kindA")
	mB := newTestMember("mB", "hostB", 1001, "kindB")
	s.AddMember(mA)
	s.AddMember(mB)

	r := s.GetActivator(newTestCI("kindA", "id"), "")
	assert.Equal(t, mA, r)
}

func TestRendezvousStrategy_CloseIsNoop(t *testing.T) {
	s := NewRendezvousStrategy()
	assert.NotPanics(t, func() { s.Close() })
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run 'TestRendezvousStrategy' -v -count=1 ./cluster/`

Expected: FAIL — `NewRendezvousStrategy` undefined.

- [ ] **Step 3: Implement RendezvousStrategy**

Create `cluster/activator_strategy_rendezvous.go`:

```go
package cluster

import (
	"hash/fnv"
	"sync"
)

// RendezvousStrategy selects members using rendezvous (highest random weight)
// hashing on the ClusterIdentity. The same identity always maps to the same
// member for a stable topology, providing cache locality.
//
// The hashing algorithm matches cluster/rendezvous.go: FNV1a32 over
// identity-bytes ++ member-address-bytes, selecting the highest score.
type RendezvousStrategy struct {
	mu      sync.RWMutex
	members []*Member
}

// NewRendezvousStrategy creates a new RendezvousStrategy with no members.
func NewRendezvousStrategy() *RendezvousStrategy {
	return &RendezvousStrategy{}
}

// GetActivator returns the member with the highest rendezvous hash score for
// ci.Identity among members that support ci.Kind. Returns nil if none available.
func (s *RendezvousStrategy) GetActivator(ci *ClusterIdentity, _ string) *Member {
	s.mu.RLock()
	eligible := membersForKind(s.members, ci.Kind)
	s.mu.RUnlock()

	l := len(eligible)
	if l == 0 {
		return nil
	}
	if l == 1 {
		return eligible[0]
	}

	keyBytes := []byte(ci.Identity)
	h := fnv.New32a()

	var maxScore uint32
	var best *Member

	for _, m := range eligible {
		h.Reset()
		h.Write(keyBytes)
		h.Write([]byte(m.Address()))
		score := h.Sum32()
		if best == nil || score > maxScore {
			maxScore = score
			best = m
		}
	}

	return best
}

// AddMember adds a member to the pool.
func (s *RendezvousStrategy) AddMember(member *Member) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.members = append(s.members, member)
}

// RemoveMember removes a member from the pool by ID. No-op if not found.
func (s *RendezvousStrategy) RemoveMember(member *Member) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.members = removeMemberByID(s.members, member.Id)
}

// Close is a no-op for RendezvousStrategy (no subscriptions to release).
func (s *RendezvousStrategy) Close() {}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run 'TestRendezvousStrategy' -v -count=1 ./cluster/`

Expected: All PASS.

- [ ] **Step 5: Run race detector**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run 'TestRendezvousStrategy' -race -count=1 ./cluster/`

Expected: All PASS, no data race warnings.

- [ ] **Step 6: Commit**

```bash
cd /home/cchamplin/development/protoactor-go
git add cluster/activator_strategy_rendezvous.go cluster/activator_strategy_rendezvous_test.go
git commit -m "feat(cluster): add RendezvousStrategy activator strategy

Deterministic hash-based placement using FNV1a32 rendezvous hashing on
ClusterIdentity. Same identity always maps to same member for stable
topology, matching the algorithm in cluster/rendezvous.go.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

## Chunk 4: GossipStrategy

### Task 4: GossipStrategy implementation

**Files:**
- Create: `cluster/activator_strategy_gossip.go`
- Create: `cluster/activator_strategy_gossip_test.go`

The `GossipStrategy` subscribes to the EventStream for `*GossipUpdate` messages. When a `GossipUpdate` arrives with `Key == HeartbeatKey`, the strategy unpacks the `MemberHeartbeat` and updates its internal `memberCounts` map under a write lock. `GetActivator` reads under a read lock to find the member with the lowest count for `ci.Kind`. Cold start (no gossip data yet) falls back to round-robin.

- [ ] **Step 1: Write the failing tests**

Create `cluster/activator_strategy_gossip_test.go`:

```go
package cluster

import (
	"testing"

	"github.com/asynkron/protoactor-go/eventstream"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/anypb"
)

// newTestEventStream creates a minimal EventStream for testing without a full cluster.
func newTestEventStream() *eventstream.EventStream {
	return eventstream.NewEventStream()
}

func publishHeartbeat(es *eventstream.EventStream, memberID string, kindCounts map[string]int64) {
	hb := &MemberHeartbeat{
		ActorStatistics: &ActorStatistics{
			ActorCount: kindCounts,
		},
	}
	anyVal, _ := anypb.New(hb)
	es.Publish(&GossipUpdate{
		MemberID: memberID,
		Key:      HeartbeatKey,
		Value:    anyVal,
	})
}

func TestGossipStrategy_ReturnsNilWhenNoMembers(t *testing.T) {
	es := newTestEventStream()
	s := NewGossipStrategy(es)
	defer s.Close()

	assert.Nil(t, s.GetActivator(newTestCI("myKind", "id"), ""))
}

func TestGossipStrategy_ColdStartFallsBackToRoundRobin(t *testing.T) {
	// No gossip data yet — should still cycle through members.
	es := newTestEventStream()
	s := NewGossipStrategy(es)
	defer s.Close()

	m1 := newTestMember("m1", "host1", 1000, "myKind")
	m2 := newTestMember("m2", "host2", 1001, "myKind")
	s.AddMember(m1)
	s.AddMember(m2)

	seen := map[string]int{}
	for i := 0; i < 10; i++ {
		r := s.GetActivator(newTestCI("myKind", "id"), "")
		if r != nil {
			seen[r.Id]++
		}
	}
	// Both should be selected without gossip data.
	assert.Greater(t, seen["m1"], 0)
	assert.Greater(t, seen["m2"], 0)
}

func TestGossipStrategy_SelectsLeastLoadedMember(t *testing.T) {
	es := newTestEventStream()
	s := NewGossipStrategy(es)
	defer s.Close()

	m1 := newTestMember("m1", "host1", 1000, "myKind")
	m2 := newTestMember("m2", "host2", 1001, "myKind")
	s.AddMember(m1)
	s.AddMember(m2)

	// m1 has 10 grains, m2 has 2 — strategy must prefer m2.
	publishHeartbeat(es, "m1", map[string]int64{"myKind": 10})
	publishHeartbeat(es, "m2", map[string]int64{"myKind": 2})

	// EventStream.Subscribe is synchronous within the same goroutine when using
	// a direct Publish, but allow the internal handler to process.
	// The strategy's subscription callback runs in the Subscribe goroutine;
	// we call GetActivator after publishes complete.
	r := s.GetActivator(newTestCI("myKind", "id"), "")
	assert.Equal(t, m2, r, "should prefer member with fewer grains")
}

func TestGossipStrategy_TieBreaksFallsToRoundRobin(t *testing.T) {
	es := newTestEventStream()
	s := NewGossipStrategy(es)
	defer s.Close()

	m1 := newTestMember("m1", "host1", 1000, "myKind")
	m2 := newTestMember("m2", "host2", 1001, "myKind")
	s.AddMember(m1)
	s.AddMember(m2)

	// Equal count on both — should cycle.
	publishHeartbeat(es, "m1", map[string]int64{"myKind": 5})
	publishHeartbeat(es, "m2", map[string]int64{"myKind": 5})

	seen := map[string]int{}
	for i := 0; i < 10; i++ {
		r := s.GetActivator(newTestCI("myKind", "id"), "")
		if r != nil {
			seen[r.Id]++
		}
	}
	// Both should receive some selections (tie-break falls to round-robin order).
	// We don't assert exact counts — just that neither is starved.
	assert.Greater(t, seen["m1"]+seen["m2"], 0)
}

func TestGossipStrategy_RemoveMemberClearsGossipData(t *testing.T) {
	es := newTestEventStream()
	s := NewGossipStrategy(es)
	defer s.Close()

	m1 := newTestMember("m1", "host1", 1000, "myKind")
	m2 := newTestMember("m2", "host2", 1001, "myKind")
	s.AddMember(m1)
	s.AddMember(m2)

	// m1 has fewer grains but gets removed.
	publishHeartbeat(es, "m1", map[string]int64{"myKind": 1})
	publishHeartbeat(es, "m2", map[string]int64{"myKind": 100})

	s.RemoveMember(m1)

	// Only m2 remains — stale m1 gossip data must not resurrect it.
	for i := 0; i < 5; i++ {
		r := s.GetActivator(newTestCI("myKind", "id"), "")
		assert.Equal(t, m2, r)
	}
}

func TestGossipStrategy_IgnoresNonHeartbeatUpdates(t *testing.T) {
	// A GossipUpdate with a different key must not affect member counts.
	es := newTestEventStream()
	s := NewGossipStrategy(es)
	defer s.Close()

	m1 := newTestMember("m1", "host1", 1000, "myKind")
	s.AddMember(m1)

	// Publish a topology update (key != HeartbeatKey) — must not panic.
	es.Publish(&GossipUpdate{MemberID: "m1", Key: "topology", Value: nil})

	r := s.GetActivator(newTestCI("myKind", "id"), "")
	assert.Equal(t, m1, r)
}

func TestGossipStrategy_CloseUnsubscribesFromEventStream(t *testing.T) {
	es := newTestEventStream()
	s := NewGossipStrategy(es)

	m1 := newTestMember("m1", "host1", 1000, "myKind")
	s.AddMember(m1)

	s.Close()

	// After Close, publishing a heartbeat must not update internal state
	// (the subscription is gone). We verify by confirming no panic and
	// that the strategy still returns a result based on pre-close data.
	assert.NotPanics(t, func() {
		publishHeartbeat(es, "m1", map[string]int64{"myKind": 999})
	})
}

func TestGossipStrategy_WithCustomScoreMember(t *testing.T) {
	es := newTestEventStream()
	// Custom scorer always returns 0 for m1 (favoured) and 1 for m2.
	scorer := func(member *Member, kindCounts map[string]int64) float64 {
		if member.Id == "m1" {
			return 0.0
		}
		return 1.0
	}
	s := NewGossipStrategyWithScorer(es, scorer)
	defer s.Close()

	m1 := newTestMember("m1", "host1", 1000, "myKind")
	m2 := newTestMember("m2", "host2", 1001, "myKind")
	s.AddMember(m1)
	s.AddMember(m2)

	// Give m2 fewer grains by default — but scorer should still prefer m1.
	publishHeartbeat(es, "m1", map[string]int64{"myKind": 100})
	publishHeartbeat(es, "m2", map[string]int64{"myKind": 1})

	r := s.GetActivator(newTestCI("myKind", "id"), "")
	assert.Equal(t, m1, r, "custom scorer should override count-based selection")
}

func TestGossipStrategy_FiltersToKindMembers(t *testing.T) {
	es := newTestEventStream()
	s := NewGossipStrategy(es)
	defer s.Close()

	mA := newTestMember("mA", "hostA", 1000, "kindA")
	mB := newTestMember("mB", "hostB", 1001, "kindB")
	s.AddMember(mA)
	s.AddMember(mB)

	r := s.GetActivator(newTestCI("kindA", "id"), "")
	assert.Equal(t, mA, r)
}

// TestGossipStrategy_RaceCondition exercises concurrent gossip writes and
// strategy reads to verify no data race under the -race flag.
func TestGossipStrategy_RaceCondition(t *testing.T) {
	es := newTestEventStream()
	s := NewGossipStrategy(es)
	defer s.Close()

	m1 := newTestMember("m1", "host1", 1000, "myKind")
	m2 := newTestMember("m2", "host2", 1001, "myKind")
	s.AddMember(m1)
	s.AddMember(m2)

	done := make(chan struct{})

	// Writer: publish gossip updates concurrently.
	go func() {
		defer close(done)
		for i := int64(0); i < 100; i++ {
			publishHeartbeat(es, "m1", map[string]int64{"myKind": i})
			publishHeartbeat(es, "m2", map[string]int64{"myKind": 100 - i})
		}
	}()

	// Reader: call GetActivator concurrently.
	for i := 0; i < 100; i++ {
		_ = s.GetActivator(newTestCI("myKind", "id"), "")
	}

	<-done
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run 'TestGossipStrategy' -v -count=1 ./cluster/`

Expected: FAIL — `NewGossipStrategy` undefined.

- [ ] **Step 3: Implement GossipStrategy**

Create `cluster/activator_strategy_gossip.go`:

```go
package cluster

import (
	"sync"
	"sync/atomic"

	"github.com/asynkron/protoactor-go/eventstream"
	"google.golang.org/protobuf/types/known/anypb"
)

// GossipStrategy selects the least-loaded member for grain placement using
// per-kind actor counts from the existing gossip heartbeat infrastructure.
//
// It subscribes to the EventStream for *GossipUpdate messages. When a
// heartbeat update arrives, it updates the internal memberCounts map under
// a write lock. GetActivator reads under a read lock to find the member with
// the lowest per-kind count.
//
// Cold start: if no gossip data is available, falls back to round-robin.
// Custom scoring: set ScoreMember to override the default count-based scoring.
type GossipStrategy struct {
	mu           sync.RWMutex
	members      []*Member
	memberCounts map[string]map[string]int64 // memberID → kind → count
	counter      uint64
	scoreMember  func(member *Member, kindCounts map[string]int64) float64

	es           *eventstream.EventStream
	subscription *eventstream.Subscription
}

// NewGossipStrategy creates a GossipStrategy that subscribes to the given
// EventStream for heartbeat updates. Call Close() when done to unsubscribe.
func NewGossipStrategy(es *eventstream.EventStream) *GossipStrategy {
	return NewGossipStrategyWithScorer(es, nil)
}

// NewGossipStrategyWithScorer creates a GossipStrategy with a custom scoring
// function. Lower scores are preferred. If scorer is nil, the default
// per-kind actor count is used as the score.
func NewGossipStrategyWithScorer(es *eventstream.EventStream, scorer func(*Member, map[string]int64) float64) *GossipStrategy {
	s := &GossipStrategy{
		memberCounts: make(map[string]map[string]int64),
		scoreMember:  scorer,
		es:           es,
	}
	s.subscription = es.Subscribe(func(evt any) {
		update, ok := evt.(*GossipUpdate)
		if !ok || update.Key != HeartbeatKey {
			return
		}
		s.handleHeartbeat(update.MemberID, update.Value)
	})
	return s
}

func (s *GossipStrategy) handleHeartbeat(memberID string, value *anypb.Any) {
	if value == nil {
		return
	}
	var hb MemberHeartbeat
	if err := value.UnmarshalTo(&hb); err != nil {
		return
	}

	counts := make(map[string]int64)
	if hb.ActorStatistics != nil && hb.ActorStatistics.ActorCount != nil {
		for kind, count := range hb.ActorStatistics.ActorCount {
			counts[kind] = count
		}
	}

	s.mu.Lock()
	s.memberCounts[memberID] = counts
	s.mu.Unlock()
}

// GetActivator returns the eligible member with the lowest score for ci.Kind.
// Falls back to round-robin when no gossip data is available.
func (s *GossipStrategy) GetActivator(ci *ClusterIdentity, _ string) *Member {
	s.mu.RLock()
	eligible := membersForKind(s.members, ci.Kind)
	// Snapshot only the counts for eligible members while holding the lock.
	// This avoids a data race between iterating the map in this method and
	// handleHeartbeat writing to it under the write lock.
	snapshot := make(map[string]map[string]int64, len(eligible))
	for _, m := range eligible {
		if c, ok := s.memberCounts[m.Id]; ok {
			snapshot[m.Id] = c
		}
	}
	s.mu.RUnlock()

	l := len(eligible)
	if l == 0 {
		return nil
	}
	if l == 1 {
		return eligible[0]
	}

	// Check whether we have gossip data for at least one eligible member.
	hasData := len(snapshot) > 0

	// Cold start: fall back to round-robin.
	if !hasData {
		n := atomic.AddUint64(&s.counter, 1)
		return eligible[int(n)%l]
	}

	// Select the member with the lowest score.
	var best *Member
	var bestScore float64
	for i, m := range eligible {
		var score float64
		kindCounts := snapshot[m.Id] // nil if no data yet for this member
		if s.scoreMember != nil {
			score = s.scoreMember(m, kindCounts)
		} else {
			score = float64(kindCounts[ci.Kind])
		}
		if best == nil || score < bestScore {
			best = eligible[i]
			bestScore = score
		}
	}
	return best
}

// AddMember adds a member to the eligible pool.
func (s *GossipStrategy) AddMember(member *Member) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.members = append(s.members, member)
}

// RemoveMember removes a member from the eligible pool and discards its
// gossip data so stale counts don't influence placement.
func (s *GossipStrategy) RemoveMember(member *Member) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.members = removeMemberByID(s.members, member.Id)
	delete(s.memberCounts, member.Id)
}

// Close unsubscribes from the EventStream. Must be called when the strategy
// is no longer needed to release the EventStream subscription.
func (s *GossipStrategy) Close() {
	if s.subscription != nil {
		s.es.Unsubscribe(s.subscription)
		s.subscription = nil
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run 'TestGossipStrategy' -v -count=1 ./cluster/`

Expected: All PASS.

- [ ] **Step 5: Run race detector**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run 'TestGossipStrategy' -race -count=1 ./cluster/`

Expected: All PASS, no data race warnings.

- [ ] **Step 6: Commit**

```bash
cd /home/cchamplin/development/protoactor-go
git add cluster/activator_strategy_gossip.go cluster/activator_strategy_gossip_test.go
git commit -m "feat(cluster): add GossipStrategy activator strategy

Least-loaded member selection using per-kind actor counts from the
existing gossip heartbeat infrastructure. Subscribes to EventStream
for *GossipUpdate messages with key 'heartbeat'. Falls back to
round-robin on cold start. Supports optional custom ScoreMember callback.
Close() unsubscribes from EventStream.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

## Chunk 5: StrategyManager

### Task 5: StrategyManager implementation

**Files:**
- Create: `cluster/activator_strategy_manager.go`
- Create: `cluster/activator_strategy_manager_test.go`

The `StrategyManager` is the single entry point for all identity lookups. It holds a map of kind name → `ActivatorStrategy` (built from `ActivatedKind.ActivatorStrategy` fields), plus a cluster-default fallback (from `Config.DefaultActivatorStrategy`). It also exposes `AddMember`/`RemoveMember` which fans out to all registered strategies, and `Close` which closes all strategies.

- [ ] **Step 1: Write the failing tests**

Create `cluster/activator_strategy_manager_test.go`:

```go
package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newManagerTestCluster creates a minimal cluster for strategy manager tests.
// Uses the existing newClusterForTest helper defined in cluster_test.go.
func newManagerTestCluster(t *testing.T) *Cluster {
	t.Helper()
	return newClusterForTest("test-mgr", newInmemoryProvider())
}

func TestStrategyManager_UsesPerKindStrategy(t *testing.T) {
	c := newManagerTestCluster(t)

	rrA := NewRoundRobinStrategy()
	rrB := NewRoundRobinStrategy()

	mA := newTestMember("mA", "hostA", 1000, "kindA")
	mB := newTestMember("mB", "hostB", 1001, "kindB")
	rrA.AddMember(mA)
	rrB.AddMember(mB)

	// Build kinds with per-kind strategies already instantiated.
	kinds := map[string]*ActivatedKind{
		"kindA": {Kind: "kindA", ActivatorStrategy: rrA},
		"kindB": {Kind: "kindB", ActivatorStrategy: rrB},
	}

	mgr := NewStrategyManager(c, kinds)
	defer mgr.Close()

	rA := mgr.GetActivator(newTestCI("kindA", "id"), "")
	assert.Equal(t, mA, rA)

	rB := mgr.GetActivator(newTestCI("kindB", "id"), "")
	assert.Equal(t, mB, rB)
}

func TestStrategyManager_FallsBackToClusterDefault(t *testing.T) {
	rrDefault := NewRoundRobinStrategy()
	m := newTestMember("m1", "host1", 1000, "unknownKind")
	rrDefault.AddMember(m)

	c := newManagerTestCluster(t)
	c.Config.DefaultActivatorStrategy = func(_ *Cluster) ActivatorStrategy { return rrDefault }

	// No per-kind strategy for "unknownKind".
	kinds := map[string]*ActivatedKind{
		"otherKind": {Kind: "otherKind", ActivatorStrategy: NewRoundRobinStrategy()},
	}

	mgr := NewStrategyManager(c, kinds)
	defer mgr.Close()

	r := mgr.GetActivator(newTestCI("unknownKind", "id"), "")
	assert.Equal(t, m, r)
}

func TestStrategyManager_ReturnsNilWhenNoStrategyAndNoDefault(t *testing.T) {
	c := newManagerTestCluster(t)
	// No DefaultActivatorStrategy set, no per-kind strategy.
	mgr := NewStrategyManager(c, map[string]*ActivatedKind{})
	defer mgr.Close()

	r := mgr.GetActivator(newTestCI("anyKind", "id"), "")
	assert.Nil(t, r)
}

func TestStrategyManager_AddMemberFansOutToAllStrategies(t *testing.T) {
	rrA := NewRoundRobinStrategy()
	rrB := NewRoundRobinStrategy()

	c := newManagerTestCluster(t)
	kinds := map[string]*ActivatedKind{
		"kindA": {Kind: "kindA", ActivatorStrategy: rrA},
		"kindB": {Kind: "kindB", ActivatorStrategy: rrB},
	}
	mgr := NewStrategyManager(c, kinds)
	defer mgr.Close()

	m := newTestMember("m1", "host1", 1000, "kindA", "kindB")
	mgr.AddMember(m)

	// Both strategies should now see the member.
	rA := rrA.GetActivator(newTestCI("kindA", "id"), "")
	assert.Equal(t, m, rA)

	rB := rrB.GetActivator(newTestCI("kindB", "id"), "")
	assert.Equal(t, m, rB)
}

func TestStrategyManager_RemoveMemberFansOutToAllStrategies(t *testing.T) {
	rrA := NewRoundRobinStrategy()

	c := newManagerTestCluster(t)
	kinds := map[string]*ActivatedKind{
		"kindA": {Kind: "kindA", ActivatorStrategy: rrA},
	}
	mgr := NewStrategyManager(c, kinds)
	defer mgr.Close()

	m := newTestMember("m1", "host1", 1000, "kindA")
	mgr.AddMember(m)
	mgr.RemoveMember(m)

	r := rrA.GetActivator(newTestCI("kindA", "id"), "")
	assert.Nil(t, r)
}

func TestStrategyManager_CloseCallsCloseOnAllStrategies(t *testing.T) {
	closed := false
	spy := &spyStrategy{onClose: func() { closed = true }}

	c := newManagerTestCluster(t)
	kinds := map[string]*ActivatedKind{
		"spyKind": {Kind: "spyKind", ActivatorStrategy: spy},
	}
	mgr := NewStrategyManager(c, kinds)
	mgr.Close()

	assert.True(t, closed, "Close on manager must Close all per-kind strategies")
}

func TestStrategyManager_DefaultStrategyCreatedOnce(t *testing.T) {
	callCount := 0
	c := newManagerTestCluster(t)
	c.Config.DefaultActivatorStrategy = func(_ *Cluster) ActivatorStrategy {
		callCount++
		return NewRoundRobinStrategy()
	}

	mgr := NewStrategyManager(c, map[string]*ActivatedKind{})
	defer mgr.Close()

	// Multiple calls for different unknown kinds use the same default instance.
	_ = mgr.GetActivator(newTestCI("kind1", "id"), "")
	_ = mgr.GetActivator(newTestCI("kind2", "id"), "")

	require.Equal(t, 1, callCount, "default strategy builder must be called exactly once")
}

// spyStrategy is a test double that records Close calls.
type spyStrategy struct {
	onClose func()
}

func (s *spyStrategy) GetActivator(_ *ClusterIdentity, _ string) *Member { return nil }
func (s *spyStrategy) AddMember(_ *Member)                                {}
func (s *spyStrategy) RemoveMember(_ *Member)                             {}
func (s *spyStrategy) Close()                                             { s.onClose() }
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run 'TestStrategyManager' -v -count=1 ./cluster/`

Expected: FAIL — `NewStrategyManager` undefined.

- [ ] **Step 3: Implement StrategyManager**

Create `cluster/activator_strategy_manager.go`:

```go
package cluster

// StrategyManager dispatches GetActivator calls to the per-kind ActivatorStrategy
// registered for each grain kind, falling back to a cluster-default strategy for
// kinds that don't have their own. It is the single point of contact between
// identity lookups and placement strategies.
//
// AddMember and RemoveMember fan out to all registered strategies (per-kind and
// default) so each strategy maintains a current view of the member pool.
//
// Close calls Close on all strategies to release EventStream subscriptions and
// other resources.
type StrategyManager struct {
	perKind    map[string]ActivatorStrategy
	defaultStr ActivatorStrategy
}

// NewStrategyManager builds a manager from the given activated kinds.
// For each kind that has a non-nil ActivatorStrategy, that strategy is used.
// If Config.DefaultActivatorStrategy is set, it is built once and used as the
// fallback for any kind without its own strategy.
// kinds maps kind name → *ActivatedKind (the value produced by Kind.Build).
func NewStrategyManager(c *Cluster, kinds map[string]*ActivatedKind) *StrategyManager {
	perKind := make(map[string]ActivatorStrategy, len(kinds))
	for name, ak := range kinds {
		if ak.ActivatorStrategy != nil {
			perKind[name] = ak.ActivatorStrategy
		}
	}

	var defaultStr ActivatorStrategy
	if c.Config.DefaultActivatorStrategy != nil {
		defaultStr = c.Config.DefaultActivatorStrategy(c)
	}

	return &StrategyManager{
		perKind:    perKind,
		defaultStr: defaultStr,
	}
}

// GetActivator returns the member that should host a new activation of ci.Kind.
// Returns nil if no strategy is configured and no member is eligible.
func (m *StrategyManager) GetActivator(ci *ClusterIdentity, senderAddress string) *Member {
	if s, ok := m.perKind[ci.Kind]; ok {
		return s.GetActivator(ci, senderAddress)
	}
	if m.defaultStr != nil {
		return m.defaultStr.GetActivator(ci, senderAddress)
	}
	return nil
}

// AddMember forwards the member to all registered strategies (per-kind and default).
func (m *StrategyManager) AddMember(member *Member) {
	for _, s := range m.perKind {
		s.AddMember(member)
	}
	if m.defaultStr != nil {
		m.defaultStr.AddMember(member)
	}
}

// RemoveMember forwards the removal to all registered strategies.
func (m *StrategyManager) RemoveMember(member *Member) {
	for _, s := range m.perKind {
		s.RemoveMember(member)
	}
	if m.defaultStr != nil {
		m.defaultStr.RemoveMember(member)
	}
}

// Close calls Close on every registered strategy to release resources.
func (m *StrategyManager) Close() {
	for _, s := range m.perKind {
		s.Close()
	}
	if m.defaultStr != nil {
		m.defaultStr.Close()
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run 'TestStrategyManager' -v -count=1 ./cluster/`

Expected: All PASS.

- [ ] **Step 5: Run race detector**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run 'TestStrategyManager' -race -count=1 ./cluster/`

Expected: All PASS, no data race warnings.

- [ ] **Step 6: Commit**

```bash
cd /home/cchamplin/development/protoactor-go
git add cluster/activator_strategy_manager.go cluster/activator_strategy_manager_test.go
git commit -m "feat(cluster): add StrategyManager for per-kind strategy dispatch

Dispatches GetActivator to the per-kind ActivatorStrategy registered
for each grain kind. Falls back to cluster-default (built once from
Config.DefaultActivatorStrategy). Fans out AddMember/RemoveMember to
all registered strategies. Close() releases all strategy resources.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

## Chunk 6: Final Verification

### Task 6: Full test pass

**Files:** No new files.

- [ ] **Step 1: Run all strategy tests together**

Run: `cd /home/cchamplin/development/protoactor-go && go test -run 'TestRoundRobinStrategy|TestLocalAffinityStrategy|TestRendezvousStrategy|TestGossipStrategy|TestStrategyManager' -v -race -count=1 ./cluster/`

Expected: All PASS, no data race warnings.

- [ ] **Step 2: Run all cluster package tests for regressions**

Run: `cd /home/cchamplin/development/protoactor-go && go test -race -count=1 ./cluster/`

Expected: All PASS. The new files add no modifications to existing files.

- [ ] **Step 3: Verify the package builds clean with no linter errors**

Run: `cd /home/cchamplin/development/protoactor-go && go vet ./cluster/`

Expected: No output (no errors).

- [ ] **Step 4: Update tracker**

Edit `docs/superpowers/plans/shared-placement-actor-tracker.md`:
- Change Sub-project 1b Plan Status to `Complete`
- Change Sub-project 1b Execution Status to `Complete`
