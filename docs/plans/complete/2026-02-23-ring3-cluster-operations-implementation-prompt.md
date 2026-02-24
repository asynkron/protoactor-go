# Ring 3: Cluster Operations - Implementation Plan Prompt

> Use this prompt in a new Claude Code session to generate a step-by-step implementation plan using the `writing-plans` skill.

## Prompt

Use the superpowers:writing-plans skill to create a detailed implementation plan from the design document at `docs/plans/2026-02-23-ring3-cluster-operations-design.md`.

This is Ring 3 of a 4-ring "Concentric Rings" production hardening effort for protoactor-go. Ring 3 fixes operational gaps (resource leaks, hangs, log storms) in the cluster package. Ring 3 can be done in parallel with Ring 2 since they touch different packages.

### Important context not in the design doc:

1. **Informer state structure:** Check `cluster/informer.go` for how `inf.state.Members` is structured. It's a `map[string]*GossipState_GossipMemberState` (from gossip.proto). The `purgeBannedMembers` implementation needs to delete entries from this map. Verify the exact field paths by reading the gossip.proto and generated code.

2. **Informer.otherMembers:** This field is set by `SetState()` method and represents the current cluster topology minus self. It's the authoritative source of truth for which members are alive. Any member ID in `state.Members` that isn't in `otherMembers` and isn't `myID` is stale and should be purged.

3. **PubSubConfig location:** The `PubSubConfig` struct is in `cluster/config.go`. It currently has `SubscriberTimeout time.Duration`. Add `SubscriptionStoreTimeout time.Duration` following the same pattern. The TopicActor is created in `cluster/cluster.go` in `ensureTopicKindRegistered()` -- that's where the config value needs to be threaded through to the actor.

4. **TopicActor constructor:** Check `cluster/pubsub_topic.go` for `NewTopicActor(store, logger)`. You'll need to add the timeout parameter. The TopicActor struct needs a field to store it. Follow how `SubscriberTimeout` is already threaded through.

5. **Actor throttle pattern:** The `actor.NewThrottle` function is in `actor/throttler.go`. Check the signature -- it takes `(maxEvents int32, period time.Duration, throttledCallback func(int32))` and returns a `ShouldThrottle` function that returns `Valve` (Open, Closing, Closed). The pattern is: `if throttle() == actor.Open { log }`.

6. **defaultClusterContext construction:** Check `cluster/default_context.go` for how it's created. The constructor is `newDefaultClusterContext()` called from config. You need to add the throttle field initialization there.

7. **Member list internals:** For the partition test (item 5), check `cluster/member_list.go` for `GetActivatorMember(kind, requestSourceAddress)`. Also check if `membersByID` map exists or if members are tracked differently. The test needs to add members via `UpdateClusterTopology` and then verify routing.

8. **Persistence test actors:** For item 6, read `persistence/plugin_test.go` carefully. The actors use the persistence plugin mixin. The "ugly blocking" is likely because the actor processes messages asynchronously and the test needs to wait for side effects. Consider using `testify`'s `Eventually` or a `sync.WaitGroup` inside the actor.

9. **Existing test helpers:** The cluster package has `newClusterForTest(name, provider, ...opts)` and `newInmemoryProvider()` in test files. Use these for any new cluster tests rather than creating new helpers.

### Expected output:

A plan saved to `docs/plans/2026-02-23-ring3-cluster-operations.md` with:
- One task per design item (6 items)
- Tasks ordered: banned member purge first (most impactful), PubSub timeouts, handler throttling, error docs, then tests
- Each task has: files to modify, step-by-step with TDD, specific commit message
- The handler throttling task should verify the throttle works by counting log output in a test
