# Ring 3: Cluster Operations Design

**Date:** 2026-02-23
**Approach:** Concentric Rings (Ring 3 of 4)
**Goal:** Fix operational gaps that cause resource leaks, hangs, or degraded performance in the cluster package
**Depends on:** Ring 1 (Core Safety)

## Background

After Rings 1 and 2 eliminate crashes and make the remote layer self-healing, the cluster package still has operational issues: banned members that accumulate forever, storage calls that can hang indefinitely, and log storms during partitions. Ring 3 addresses these operational health issues.

## Changes

### 1. Banned Member Purging in Informer

**File:** `cluster/informer.go:112`

**Current:** `// inf.purgeBannedMembers()  // TODO` -- dead members accumulate in gossip state forever, causing unnecessary gossip traffic.

**Design:** Implement `purgeBannedMembers` to remove members from gossip state that are no longer in the `otherMembers` list:

```go
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

Uncomment the call in `SendState()`:
```go
func (inf *Informer) SendState(sendStateToMember LocalStateSender) {
    inf.purgeBannedMembers()
    // ... rest unchanged
```

**Safety:** `otherMembers` is the authoritative list maintained by the cluster topology. Any member not in this list has already been reported as left/failed. Purging their gossip state is correct -- keeping it wastes bandwidth gossiping state for nodes that no longer exist.

### 2. PubSub Context Timeouts

**File:** `cluster/pubsub_topic.go:249, 268`

**Current:** `context.Background()` for subscription store operations. If the store hangs, the TopicActor blocks forever.

**Design:** Add a configurable timeout:

**Config change** (`cluster/config.go` in `PubSubConfig`):
```go
type PubSubConfig struct {
    SubscriberTimeout      time.Duration
    SubscriptionStoreTimeout time.Duration // NEW, default 5s
}
```

**Usage in pubsub_topic.go:**
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
        logger.Error("Failed to load subscriptions",
            slog.String("topic", topic), slog.Any("error", err))
        return &Subscribers{}
    }
    // ... existing logic
}
```

Same pattern for `saveSubscriptionsInTopicActor`.

The `subscriptionStoreTimeout` should be passed to the TopicActor from the PubSubConfig when it's created.

### 3. Cluster Context Handler Throttling

**File:** `cluster/default_context.go:72, 169`

**Current:** `// TODO: handler throttling and messaging here` -- during network partitions, many requests time out simultaneously, flooding logs.

**Design:** Use a simple log-rate limiter. The actor package already has a throttle pattern. Apply it to timeout error logging:

```go
type defaultClusterContext struct {
    // ... existing fields
    timeoutLogThrottle actor.ShouldThrottle
}

func newDefaultClusterContext(c *Cluster) Context {
    return &defaultClusterContext{
        cluster:            c,
        // Allow 5 timeout log messages per 10 seconds
        timeoutLogThrottle: actor.NewThrottle(5, 10*time.Second, func(count int32) {
            c.Logger().Warn("Cluster request timeout logging throttled",
                slog.Int("suppressed", int(count)))
        }),
    }
}
```

At the timeout handlers:
```go
case <-ctx.Done():
    err = fmt.Errorf("request failed: %w", ctx.Err())
    if c.timeoutLogThrottle() == actor.Open {
        c.cluster.Logger().Warn("Cluster request timed out",
            slog.String("identity", identity),
            slog.String("kind", kind))
    }
    break selectloop
```

This doesn't change behavior -- all timeouts still return errors to callers. It just prevents log storms.

### 4. Cluster Context Error Ambiguity Documentation

**File:** `cluster/default_context.go:98`

**Current:** `// TODO: why is err != nil when res != nil?`

**Replace with:**
```go
// RequestFuture.Result() can return both a response and an error when the
// target actor processes the message but the response delivery encounters
// an issue (e.g., DeadLetterResponse). When we have a valid response,
// we use it regardless of the error, since the actor did produce a result.
resp, err = _context.RequestFuture(pid, message, ttl).Result()
if resp != nil {
    break selectloop
}
```

Add a debug log when both are non-nil for production tracing:
```go
if resp != nil {
    if err != nil {
        c.cluster.Logger().Debug("Cluster request returned both response and error",
            slog.String("identity", identity),
            slog.String("kind", kind),
            slog.Any("error", err))
    }
    break selectloop
}
```

### 5. Member List Partition Test

**File:** `cluster/member_list_test.go:248-260`

**Current:** Disabled test `TestMemberList_getPartitionMemberV2` -- the method no longer exists.

**Design:** Check if `GetActivatorMember` covers the same functionality (routing to the correct partition member for a given kind). If yes, rewrite:

```go
func TestMemberList_GetActivatorMember(t *testing.T) {
    cp := newInmemoryProvider()
    c := newClusterForTest("test-partition", cp,
        WithKinds(NewKind("testKind", actor.PropsFromFunc(func(ctx actor.Context) {}))))

    err := c.StartMember()
    require.NoError(t, err)

    // Add members with the testKind
    c.MemberList.UpdateClusterTopology([]*Member{
        {Id: "m1", Host: "h1", Port: 1, Kinds: []string{"testKind"}},
        {Id: "m2", Host: "h2", Port: 2, Kinds: []string{"testKind"}},
    })

    // Should return a member for the kind
    activator := c.MemberList.GetActivatorMember("testKind", "some-request-source")
    assert.NotEmpty(t, activator)

    // Unknown kind should return empty
    activator = c.MemberList.GetActivatorMember("unknownKind", "some-request-source")
    assert.Empty(t, activator)
}
```

If `getPartitionMemberV2` had different semantics that are no longer relevant, delete the disabled test entirely.

### 6. Persistence Test Synchronization

**File:** `persistence/plugin_test.go:106, 153, 167`

**Current:** Comments say "ugly way to block on a response" and "TODO: I need some help here".

**Design:** Replace sleep-based synchronization with `actor.Future`:

```go
// Instead of:
//   system.Root.Send(pid, msg)
//   time.Sleep(100 * time.Millisecond)
//   // check state

// Use:
future := system.Root.RequestFuture(pid, msg, 5*time.Second)
resp, err := future.Result()
require.NoError(t, err)
// check resp
```

If the actor doesn't respond to the message type, add a response. Or use an event-based approach: subscribe to a known event that the actor publishes after processing.

## Testing Strategy

- Banned member purge: test that gossip state doesn't grow after members leave
- PubSub timeout: test with a mock store that blocks, verify timeout triggers
- Handler throttling: test that logging is limited under burst conditions
- Member list test: verify correct partition routing
- Persistence tests: verify they don't use time.Sleep for synchronization

## Risk Assessment

| Change | Risk | Mitigation |
|--------|------|------------|
| Banned member purge | Low | Simple map cleanup, conservative check |
| PubSub context timeout | Low | Adds timeout to existing calls, no behavior change on happy path |
| Handler throttling | Low | Logging-only change, no behavior change |
| Error docs | Trivial | Comment change |
| Member list test | Low | New test or deletion of stale test |
| Persistence test sync | Low | Test-only changes |

## Dependencies

- Ring 1 must be complete (error suppression fixes are prerequisite for clean operational behavior)
- Ring 3 is independent of Ring 2 (can be done in parallel)
- Ring 4 depends on Ring 3 being done (MemberList changes in Ring 4 build on stable member tracking)
