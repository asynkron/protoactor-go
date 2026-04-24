# Ring 3: Cluster Operations Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix operational gaps (resource leaks, hangs, log storms) in the cluster package

**Architecture:** Six targeted fixes in cluster and persistence packages: implement banned member purging in gossip state, add context timeouts to PubSub store operations, add log throttling to cluster context timeout paths, document an ambiguous error pattern, replace a disabled test with a working one, and modernize persistence test synchronization. All changes are isolated -- no cross-task dependencies.

**Tech Stack:** Go, protoactor-go (actor, cluster, persistence, remote packages), testify, protobuf

---

### Task 1: Banned Member Purging in Informer

**Files:**
- Modify: `cluster/informer.go:112` (uncomment call), add `purgeBannedMembers` method
- Test: `cluster/informer_test.go` (create)

**Step 1: Write the failing test**

Create `cluster/informer_test.go`:

```go
package cluster

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInformer_purgeBannedMembers(t *testing.T) {
	logger := slog.Default()
	inf := newInformer("self", nil, 3, 50, logger)

	// Simulate gossip state with 3 members: self, alive, stale
	inf.state.Members["self"] = &GossipMemberState{
		Values: map[string]*GossipKeyValue{},
	}
	inf.state.Members["alive-member"] = &GossipMemberState{
		Values: map[string]*GossipKeyValue{},
	}
	inf.state.Members["stale-member"] = &GossipMemberState{
		Values: map[string]*GossipKeyValue{},
	}

	// otherMembers only contains "alive-member" -- "stale-member" has left
	inf.otherMembers = []*Member{
		{Id: "alive-member", Host: "h1", Port: 1},
	}

	inf.purgeBannedMembers()

	// self is preserved
	assert.Contains(t, inf.state.Members, "self")
	// alive member is preserved
	assert.Contains(t, inf.state.Members, "alive-member")
	// stale member is purged
	assert.NotContains(t, inf.state.Members, "stale-member")
}

func TestInformer_purgeBannedMembers_noOtherMembers(t *testing.T) {
	logger := slog.Default()
	inf := newInformer("self", nil, 3, 50, logger)

	inf.state.Members["self"] = &GossipMemberState{
		Values: map[string]*GossipKeyValue{},
	}
	inf.state.Members["orphan"] = &GossipMemberState{
		Values: map[string]*GossipKeyValue{},
	}

	// No other members -- all non-self members should be purged
	inf.otherMembers = []*Member{}

	inf.purgeBannedMembers()

	assert.Contains(t, inf.state.Members, "self")
	assert.NotContains(t, inf.state.Members, "orphan")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cluster/ -run TestInformer_purgeBannedMembers -v`
Expected: FAIL -- `purgeBannedMembers` method does not exist (only a commented-out call site)

**Step 3: Implement `purgeBannedMembers` and uncomment the call**

Add the method to `cluster/informer.go` (after the `SendState` method, around line 143):

```go
// purgeBannedMembers removes gossip state entries for members that are no
// longer in the otherMembers list (i.e., they have left or been banned).
// Self is always preserved.
func (inf *Informer) purgeBannedMembers() {
	for memberID := range inf.state.Members {
		if memberID == inf.myID {
			continue
		}
		found := false
		for _, m := range inf.otherMembers {
			if m.Id == memberID {
				found = true
				break
			}
		}
		if !found {
			inf.logger.Debug("Purging banned member from gossip state",
				slog.String("memberID", memberID))
			delete(inf.state.Members, memberID)
		}
	}
}
```

In `SendState` (line 112), change:
```go
// inf.purgeBannedMembers()  // TODO
```
to:
```go
inf.purgeBannedMembers()
```

**Step 4: Run test to verify it passes**

Run: `go test ./cluster/ -run TestInformer_purgeBannedMembers -v`
Expected: PASS

**Step 5: Run all cluster tests to check for regressions**

Run: `go test ./cluster/ -v -count=1`
Expected: All tests PASS

**Step 6: Commit**

```bash
git add cluster/informer.go cluster/informer_test.go
git commit -m "feat(cluster): implement banned member purging in gossip informer

Uncomment and implement purgeBannedMembers() which removes gossip state
entries for members no longer in the otherMembers list. This prevents
unbounded growth of gossip state when members leave the cluster."
```

---

### Task 2: PubSub Subscription Store Context Timeouts

**Files:**
- Modify: `cluster/pubsub.go:45-51` (add `SubscriptionStoreTimeout` to `PubSubConfig`)
- Modify: `cluster/pubsub_topic.go:16-33` (add field and constructor param, use timeout in `loadSubscriptions` and `saveSubscriptionsInTopicActor`)
- Modify: `cluster/cluster.go:268-274` (thread config value through to `NewTopicActor`)
- Test: `cluster/pubsub_topic_test.go` (create)

**Step 1: Write the failing test**

Create `cluster/pubsub_topic_test.go`:

```go
package cluster

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// blockingStore blocks forever on Get/Set until its context is cancelled.
type blockingStore struct{}

func (s *blockingStore) Get(ctx context.Context, key string) (*Subscribers, error) {
	<-ctx.Done()
	return nil, fmt.Errorf("store blocked: %w", ctx.Err())
}

func (s *blockingStore) Set(ctx context.Context, key string, value *Subscribers) error {
	<-ctx.Done()
	return fmt.Errorf("store blocked: %w", ctx.Err())
}

func (s *blockingStore) Clear(ctx context.Context, key string) error {
	<-ctx.Done()
	return fmt.Errorf("store blocked: %w", ctx.Err())
}

func TestTopicActor_loadSubscriptions_timesOut(t *testing.T) {
	store := &blockingStore{}
	logger := slog.Default()
	timeout := 50 * time.Millisecond
	ta := NewTopicActor(store, logger, timeout)

	start := time.Now()
	subs := ta.loadSubscriptions("test-topic", logger)
	elapsed := time.Since(start)

	// Should return empty subscribers, not hang forever
	assert.NotNil(t, subs)
	assert.Empty(t, subs.Subscribers)
	// Should complete within a reasonable multiple of the timeout
	assert.Less(t, elapsed, 500*time.Millisecond)
}

func TestTopicActor_saveSubscriptions_timesOut(t *testing.T) {
	store := &blockingStore{}
	logger := slog.Default()
	timeout := 50 * time.Millisecond
	ta := NewTopicActor(store, logger, timeout)
	ta.topic = "test-topic"

	start := time.Now()
	ta.saveSubscriptionsInTopicActor(logger)
	elapsed := time.Since(start)

	// Should complete within a reasonable multiple of the timeout
	assert.Less(t, elapsed, 500*time.Millisecond)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cluster/ -run TestTopicActor_ -v`
Expected: FAIL -- `NewTopicActor` does not accept a timeout parameter

**Step 3: Add `SubscriptionStoreTimeout` to PubSubConfig**

In `cluster/pubsub.go`, change `PubSubConfig` and `newPubSubConfig`:

```go
type PubSubConfig struct {
	// SubscriberTimeout is a timeout used when delivering a message batch to a subscriber. Default is 5s.
	//
	// This value gets rounded to seconds for optimization of cancellation token creation. Note that internally,
	// cluster request is used to deliver messages to ClusterIdentity subscribers.
	SubscriberTimeout time.Duration

	// SubscriptionStoreTimeout is the timeout for subscription store Get/Set operations.
	// If the store does not respond within this duration, the operation fails gracefully.
	// Default is 5s.
	SubscriptionStoreTimeout time.Duration
}

func newPubSubConfig() *PubSubConfig {
	return &PubSubConfig{
		SubscriberTimeout:        5 * time.Second,
		SubscriptionStoreTimeout: 5 * time.Second,
	}
}
```

**Step 4: Add timeout field to TopicActor and update constructor**

In `cluster/pubsub_topic.go`, change the struct and constructor:

```go
type TopicActor struct {
	topic                     string
	subscribers               map[subscribeIdentityStruct]*SubscriberIdentity
	subscriptionStore         KeyValueStore[*Subscribers]
	subscriptionStoreTimeout  time.Duration
	topologySubscription      *eventstream.Subscription
	shouldThrottle            actor.ShouldThrottle
}

func NewTopicActor(store KeyValueStore[*Subscribers], logger *slog.Logger, subscriptionStoreTimeout time.Duration) *TopicActor {
	return &TopicActor{
		subscriptionStore:        store,
		subscriptionStoreTimeout: subscriptionStoreTimeout,
		subscribers:              make(map[subscribeIdentityStruct]*SubscriberIdentity),
		shouldThrottle: actor.NewThrottleWithLogger(logger, 10, time.Second, func(logger *slog.Logger, count int32) {
			logger.Info("[TopicActor] Throttled logs", slog.Int("count", int(count)))
		}),
	}
}
```

**Step 5: Add context timeouts to `loadSubscriptions` and `saveSubscriptionsInTopicActor`**

In `cluster/pubsub_topic.go`, change `loadSubscriptions` (around line 248):

```go
func (t *TopicActor) loadSubscriptions(topic string, logger *slog.Logger) *Subscribers {
	timeout := t.subscriptionStoreTimeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	state, err := t.subscriptionStore.Get(ctx, topic)
	if err != nil {
		if t.shouldThrottle() == actor.Open {
			logger.Error("Error when loading subscriptions", slog.String("topic", topic), slog.Any("error", err))
		}
		return &Subscribers{}
	}
	if state == nil {
		return &Subscribers{}
	}
	logger.Debug("Loaded subscriptions for topic", slog.String("topic", topic), slog.Any("subscriptions", state))
	return state
}
```

Change `saveSubscriptionsInTopicActor` (around line 265):

```go
func (t *TopicActor) saveSubscriptionsInTopicActor(logger *slog.Logger) {
	subscribers := &Subscribers{Subscribers: maps.Values(t.subscribers)}

	timeout := t.subscriptionStoreTimeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	logger.Debug("Saving subscriptions for topic", slog.String("topic", t.topic), slog.Any("subscriptions", subscribers))
	err := t.subscriptionStore.Set(ctx, t.topic, subscribers)
	if err != nil && t.shouldThrottle() == actor.Open {
		logger.Error("Error when saving subscriptions", slog.String("topic", t.topic), slog.Any("error", err))
	}
}
```

**Step 6: Thread config through `ensureTopicKindRegistered`**

In `cluster/cluster.go`, change `ensureTopicKindRegistered` (line 268-274):

```go
	if !hasTopicKind {
		store := &EmptyKeyValueStore[*Subscribers]{}
		storeTimeout := c.Config.PubSubConfig.SubscriptionStoreTimeout

		c.kinds[TopicActorKind] = NewKind(TopicActorKind, actor.PropsFromProducer(func() actor.Actor {
			return NewTopicActor(store, c.Logger(), storeTimeout)
		})).Build(c)
	}
```

**Step 7: Run test to verify it passes**

Run: `go test ./cluster/ -run TestTopicActor_ -v`
Expected: PASS

**Step 8: Run all cluster tests**

Run: `go test ./cluster/ -v -count=1`
Expected: All tests PASS

**Step 9: Commit**

```bash
git add cluster/pubsub.go cluster/pubsub_topic.go cluster/cluster.go cluster/pubsub_topic_test.go
git commit -m "feat(cluster): add context timeouts to PubSub subscription store operations

Add SubscriptionStoreTimeout to PubSubConfig (default 5s) and thread it
through to TopicActor. loadSubscriptions and saveSubscriptionsInTopicActor
now use context.WithTimeout instead of context.Background(), preventing
indefinite hangs when the subscription store is unresponsive."
```

---

### Task 3: Cluster Context Handler Throttling

**Files:**
- Modify: `cluster/default_context.go:23-37` (add throttle field, initialize in constructor)
- Modify: `cluster/default_context.go:72-74` (use throttle at timeout in `Request`)
- Modify: `cluster/default_context.go:168-171` (use throttle at timeout in `RequestFuture`)
- Test: `cluster/default_context_test.go` (create)

**Step 1: Write the failing test**

Create `cluster/default_context_test.go`:

```go
package cluster

import (
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/stretchr/testify/assert"
)

func TestDefaultContext_timeoutLogThrottle(t *testing.T) {
	cp := newInmemoryProvider()
	c := newClusterForTest("test-throttle", cp)

	ctx := newDefaultClusterContext(c)
	dcc, ok := ctx.(*DefaultContext)
	assert.True(t, ok)
	assert.NotNil(t, dcc.timeoutLogThrottle)

	// First 5 calls should return Open
	for i := 0; i < 5; i++ {
		valve := dcc.timeoutLogThrottle()
		if i < 4 {
			assert.Equal(t, actor.Open, valve, "call %d should be Open", i)
		}
	}
	// 5th call is the threshold boundary (Closing), 6th+ should be Closed
	valve := dcc.timeoutLogThrottle()
	assert.Equal(t, actor.Closed, valve, "call beyond threshold should be Closed")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cluster/ -run TestDefaultContext_timeoutLogThrottle -v`
Expected: FAIL -- `timeoutLogThrottle` field does not exist on `DefaultContext`

**Step 3: Add throttle field and initialize it**

In `cluster/default_context.go`, change the struct and constructor:

```go
type DefaultContext struct {
	cluster            *Cluster
	timeoutLogThrottle actor.ShouldThrottle
}

func newDefaultClusterContext(cluster *Cluster) Context {
	clusterContext := DefaultContext{
		cluster: cluster,
		timeoutLogThrottle: actor.NewThrottle(5, 10*time.Second, func(count int32) {
			cluster.Logger().Warn("Cluster request timeout logging throttled",
				slog.Int("suppressed", int(count)))
		}),
	}

	return &clusterContext
}
```

**Step 4: Use the throttle at the timeout handlers**

In `Request` method (around line 71-74), change:

```go
		case <-ctx.Done():
			// TODO: handler throttling and messaging here
			err = fmt.Errorf("request failed: %w", ctx.Err())

			break selectloop
```

to:

```go
		case <-ctx.Done():
			err = fmt.Errorf("request failed: %w", ctx.Err())
			if dcc.timeoutLogThrottle() == actor.Open {
				dcc.cluster.Logger().Warn("Cluster request timed out",
					slog.String("identity", identity),
					slog.String("kind", kind))
			}
			break selectloop
```

In `RequestFuture` method (around line 168-171), change:

```go
		case <-ctx.Done():
			// TODO: handler throttling and messaging here
			err := fmt.Errorf("request failed: %w", ctx.Err())
			return nil, err
```

to:

```go
		case <-ctx.Done():
			err := fmt.Errorf("request failed: %w", ctx.Err())
			if dcc.timeoutLogThrottle() == actor.Open {
				dcc.cluster.Logger().Warn("Cluster future request timed out",
					slog.String("identity", identity),
					slog.String("kind", kind))
			}
			return nil, err
```

**Step 5: Run test to verify it passes**

Run: `go test ./cluster/ -run TestDefaultContext_timeoutLogThrottle -v`
Expected: PASS

**Step 6: Run all cluster tests**

Run: `go test ./cluster/ -v -count=1`
Expected: All tests PASS

**Step 7: Commit**

```bash
git add cluster/default_context.go cluster/default_context_test.go
git commit -m "feat(cluster): add log throttling for cluster request timeouts

Add a timeoutLogThrottle to DefaultContext (5 events per 10s) that
rate-limits timeout warning logs. During network partitions, many
requests time out simultaneously -- this prevents log storms while
still returning errors to all callers."
```

---

### Task 4: Cluster Context Error Ambiguity Documentation

**Files:**
- Modify: `cluster/default_context.go:98-102` (replace TODO comment, add debug log)

**Step 1: Read the current code to confirm exact lines**

Verify that lines 98-102 of `cluster/default_context.go` still contain the TODO comment and the `resp != nil` check.

**Step 2: Replace the TODO with documentation and add debug log**

In `cluster/default_context.go`, change:

```go
			// TODO: why is err != nil when res != nil?
			resp, err = _context.RequestFuture(pid, message, ttl).Result()
			if resp != nil {
				break selectloop
			}
```

to:

```go
			// RequestFuture.Result() can return both a response and an error when the
			// target actor processes the message but the response delivery encounters
			// an issue (e.g., DeadLetterResponse). When we have a valid response,
			// we use it regardless of the error, since the actor did produce a result.
			resp, err = _context.RequestFuture(pid, message, ttl).Result()
			if resp != nil {
				if err != nil {
					dcc.cluster.Logger().Debug("Cluster request returned both response and error",
						slog.String("identity", identity),
						slog.String("kind", kind),
						slog.Any("error", err))
				}
				break selectloop
			}
```

**Step 3: Run all cluster tests to check for regressions**

Run: `go test ./cluster/ -v -count=1`
Expected: All tests PASS

**Step 4: Commit**

```bash
git add cluster/default_context.go
git commit -m "docs(cluster): document why RequestFuture can return both response and error

Replace the TODO comment with an explanation: Result() can return both
when the actor processes the message but response delivery has issues.
Add a debug log for production tracing when both are non-nil."
```

---

### Task 5: Replace Disabled Member List Partition Test

**Files:**
- Modify: `cluster/member_list_test.go:200-268` (replace disabled tests with working `GetActivatorMember` test)

**Step 1: Write the new test replacing disabled code**

Replace the disabled `TestMemberList_getPartitionMember` (lines 200-223) and the commented-out `TestMemberList_getPartitionMemberV2` (lines 248-268) and benchmark (lines 225-246) with:

```go
func TestMemberList_GetActivatorMember(t *testing.T) {
	t.Parallel()

	c := newClusterForTest("test-activator", newInmemoryProvider())
	obj := NewMemberList(c)

	members := newMembersForTest(3) // these have kind "kind" by default
	obj.UpdateClusterTopology(members)

	t.Run("known kind returns a member", func(t *testing.T) {
		activator := obj.GetActivatorMember("kind", "some-source")
		assert.NotEmpty(t, activator)
	})

	t.Run("consistent routing for same input", func(t *testing.T) {
		first := obj.GetActivatorMember("kind", "test-identity")
		for i := 0; i < 10; i++ {
			again := obj.GetActivatorMember("kind", "test-identity")
			assert.Equal(t, first, again)
		}
	})

	t.Run("unknown kind returns empty", func(t *testing.T) {
		activator := obj.GetActivatorMember("nonexistent-kind", "some-source")
		assert.Empty(t, activator)
	})
}
```

**Step 2: Run the test**

Run: `go test ./cluster/ -run TestMemberList_GetActivatorMember -v`
Expected: PASS

**Step 3: Run all cluster tests**

Run: `go test ./cluster/ -v -count=1`
Expected: All tests PASS

**Step 4: Commit**

```bash
git add cluster/member_list_test.go
git commit -m "test(cluster): replace disabled partition test with GetActivatorMember test

The old getPartitionMemberV2 method no longer exists. Replace the
disabled tests and benchmark with a working test for GetActivatorMember
that verifies routing for known kinds, consistent results, and empty
returns for unknown kinds."
```

---

### Task 6: Persistence Test Synchronization

**Files:**
- Modify: `persistence/plugin_test.go:74-112` (actor responds to Query, remove global state)
- Modify: `persistence/plugin_test.go:139-178` (use RequestFuture instead of Send+WaitGroup)

**Step 1: Read the test file to confirm current state**

Verify lines 74-178 of `persistence/plugin_test.go` match the expected TODO comments and global `queryWg`/`queryState` pattern.

**Step 2: Modify the actor to respond to Query messages**

Change the actor's `Receive` method -- the `*Query` case should use `ctx.Respond` instead of global state:

Replace the global variables (lines 85-88):
```go
var (
	queryWg    sync.WaitGroup
	queryState string
)
```

Remove them entirely.

Change the actor's Receive (lines 90-112) to:

```go
func (a *myActor) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *RequestSnapshot:
		a.PersistSnapshot(newSnapshot(a.state))
	case *Snapshot:
		a.state = msg.state
	case *Message:
		if !a.Recovering() {
			a.PersistReceive(msg)
		}
		a.state = msg.state
	case *Query:
		ctx.Respond(newMessage(a.state))
	}
}
```

**Step 3: Replace Send+WaitGroup with RequestFuture in the test**

Change the test body (inside the `for i, tc` loop). Replace each block:

```go
			// ugly way to block on a response....
			// TODO: I need some help here
			queryWg.Add(1)
			rootContext.Send(pid, &Query{})
			queryWg.Wait()
			// check the state after all these messages
			assert.Equal(t, tc.afterMsgs, queryState)
```

with:

```go
			resp, err := rootContext.RequestFuture(pid, &Query{}, 5*time.Second).Result()
			require.NoError(t, err)
			result, ok := resp.(*Message)
			require.True(t, ok)
			assert.Equal(t, tc.afterMsgs, result.state)
```

Do this for **both** occurrences in the test (first query around line 152-158, second around line 166-172).

Also add `"time"` to the import block if not already present, and remove the `"sync"` import since `queryWg` is gone.

**Step 4: Run the test**

Run: `go test ./persistence/ -run TestRecovery -v`
Expected: PASS

**Step 5: Commit**

```bash
git add persistence/plugin_test.go
git commit -m "refactor(persistence): replace sleep-based test sync with RequestFuture

Remove the global queryWg/queryState pattern and instead have the actor
respond to Query messages via ctx.Respond. Tests now use RequestFuture
for synchronous request-response, eliminating race conditions and the
TODO comments about blocking."
```

---

Plan complete and saved to `docs/plans/2026-02-23-ring3-cluster-operations.md`. Two execution options:

**1. Subagent-Driven (this session)** - I dispatch fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** - Open new session with executing-plans, batch execution with checkpoints

Which approach?
