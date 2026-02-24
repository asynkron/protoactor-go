# Ring 4: Runtime Kind Addition - Implementation Plan Prompt

> Use this prompt in a new Claude Code session to generate a step-by-step implementation plan using the `writing-plans` skill.

## Prompt

Use the superpowers:writing-plans skill to create a detailed implementation plan from the design document at `docs/plans/2026-02-23-ring4-runtime-kind-addition-design.md`.

This is Ring 4 of a 4-ring "Concentric Rings" production hardening effort for protoactor-go. Ring 4 adds the ability to register new Kinds after the cluster is running. This is the most architecturally significant ring. **Rings 1 and 3 must be complete before starting Ring 4.**

### Important context not in the design doc:

1. **Cluster struct fields:** Check `cluster/cluster.go` lines 21-35. The Cluster struct currently has `kinds map[string]*ActivatedKind` as a private field. You need to add `kindsMu sync.RWMutex` and `provider ClusterProvider`. The provider is currently only accessed within `StartMember()` -- you need to save it as a field.

2. **Kind.Build method:** Check `cluster/kind.go` for the `Build(c *Cluster)` method on `Kind`. It returns `*ActivatedKind`. The build process wraps the Props with cluster middleware. This must be called under the write lock since it reads cluster state.

3. **ActivatedKind vs Kind:** `Kind` is the user-facing config type. `ActivatedKind` is the internal type with built Props and strategy. `RegisterKind` takes a `*Kind` and calls `Build()` internally. Users should never need to create `ActivatedKind` directly.

4. **GetClusterKinds current implementation:** Check if `GetClusterKinds()` already exists as a method on Cluster. If it doesn't exist, it may be a free function or inline code. Search for where kind names are collected into a string slice -- that's what providers call at startup. The method may be named differently (e.g., `getClusterKindNames()`).

5. **Provider storage of cluster reference:** Check each provider to see if they already store a reference to the Cluster. The `init(c *cluster.Cluster)` method (or `StartMember` first arg) passes the cluster -- most providers store it. If a provider doesn't store it, add the field.

6. **Automanaged broadcast:** Check `cluster/clusterproviders/automanaged/automanaged.go` for the broadcast mechanism. It may use a ticker loop that calls a method like `getCurrentNode()` or `broadcastState()`. The `UpdateKinds` implementation needs to either update the cached node state or trigger an immediate broadcast. Find the exact method names.

7. **NATS KV registerSelf:** Check `cluster/clusterproviders/natskv/natskv_provider.go` for `registerSelf()` or equivalent. It writes the node state to a KV bucket. The `UpdateKinds` implementation calls this after updating `p.self.Kinds`. Check if `p.self` is accessed from multiple goroutines -- if so, ensure the mutex protects it.

8. **NATS Stream publishHeartbeat:** Check `cluster/clusterproviders/natsstream/natsstream_provider.go` for the heartbeat publish method. Same pattern as NATS KV. The heartbeat is published periodically and carries the full node state including Kinds.

9. **K8s pod labels for Kinds:** Check `cluster/clusterproviders/k8s/k8s_provider.go` for how Kinds are encoded in pod labels. Look for `LabelKinds` or similar constants. K8s labels have value length limits (63 chars). If Kinds are encoded as a comma-separated string in a single label, there's a size limit. Check how this is handled currently and whether it needs to change for dynamic Kinds.

10. **MemberList topology diff:** The most delicate change is in `cluster/member_list.go`. Read the entire `UpdateClusterTopology` method carefully. It:
    - Computes joined/left by comparing old and new member sets
    - Calls `memberJoin` for new members (adds to MemberStrategy per kind)
    - Calls `memberLeave` for departed members
    - Publishes `ClusterTopology` event
    The Kind-change detection must happen BEFORE the old member state is overwritten. Find exactly where `previousMembers` is stored and when it's updated.

11. **MemberStrategy interface:** Check `cluster/member_strategy.go` for the `MemberStrategy` interface. Verify that `AddMember` is idempotent. If a member is already added for a Kind, adding it again must not create duplicates. If it's not idempotent, the Kind-change code needs to check before adding.

12. **Topology hash:** Check whether the topology hash calculation includes Kinds. If it doesn't, Kind-only changes won't trigger topology events. You may need to update the hash to include Kind lists, or find another way to trigger re-evaluation.

13. **Race detector:** ALL new code must pass `go test -race`. The RWMutex is critical for this. Run the race detector on every test step.

14. **Test infrastructure for multi-node:** The NATS providers have integration tests using testcontainers (see `cluster/clusterproviders/natskv/natskv_provider_integration_test.go`). Use this pattern for Ring 4 integration tests. For automanaged, two in-process nodes can communicate without external infrastructure.

### Expected output:

A plan saved to `docs/plans/2026-02-23-ring4-runtime-kind-addition.md` with:
- Tasks ordered: thread-safe registry first (foundation), then RegisterKind API, then KindUpdater interface, then providers (automanaged, natskv, natsstream, k8s), then MemberList, then integration tests
- MemberList task should be the most detailed since it's the highest risk
- Each provider task should be independent (can be done in any order after the API is ready)
- Final integration test task that exercises the full flow: register Kind, verify propagation, activate actor
- Use TDD for every task
- Run `go test -race` after every change
