# Implementation Planning Prompt: NATS JetStream Cluster Provider

Use this prompt in a new session to create a detailed implementation plan using the `writing-plans` skill.

---

## Prompt

I need to create a detailed implementation plan for a new NATS JetStream-based cluster provider for our protoactor-go fork. The design document is at `docs/plans/2026-02-22-nats-jetstream-cluster-provider-design.md` — read it first.

Use the `/writing-plans` skill to create the implementation plan.

### Key Context Not in the Design Doc

**Interfaces to implement:**

1. `cluster.ClusterProvider` (defined in `cluster/cluster_provider.go`):
   - `StartMember(cluster *Cluster) error`
   - `StartClient(cluster *Cluster) error`
   - `Shutdown(graceful bool) error`

2. `cluster.IdentityLookup` (defined in `cluster/identity_lookup.go`):
   - `Get(clusterIdentity *ClusterIdentity) *actor.PID`
   - `RemovePid(clusterIdentity *ClusterIdentity, pid *actor.PID)`
   - `Setup(cluster *Cluster, kinds []string, isClient bool)`
   - `Shutdown()`

3. `RoleChangedListener` — a local interface (not from cluster core) with `OnRoleChanged(RoleType)`. Both etcd and ZK define this locally in their own packages. We do the same.

**Critical patterns to follow from existing providers (study these files):**

- `cluster/clusterproviders/etcd/etcd_provider.go` — the primary reference implementation. Follow its patterns for:
  - `init()` method that extracts host/port from `cluster.ActorSystem.Address()` using `net.SplitHostPort`, gets `cluster.ActorSystem.ID` as memberID, and `c.GetClusterKinds()` for known kinds
  - `publishClusterTopologyEvent()` that converts internal node state to `[]*cluster.Member` and calls `cluster.MemberList.UpdateClusterTopology(members)`
  - `Shutdown()` using `atomic.Bool.CompareAndSwap(false, true)` guard
  - `startWatching()` goroutine with panic recovery and retry loop
  - `startKeepAlive()` goroutine with shutdown check loop (this becomes the heartbeat publisher)
  - `startRoleChangedNotifyLoop()` goroutine that reads from a role channel

- `cluster/clusterproviders/etcd/node.go` — the `Node` struct pattern. Create an equivalent for JetStream:
  - JSON-serializable struct with ID, Name, Host, Address, Port, Kinds, Meta, Alive, Timestamp fields
  - `MemberStatus() *cluster.Member` converter method
  - `Serialize()`/`Deserialize()` methods
  - `NewNode()` and `NewNodeFromBytes()` constructors
  - Add a `Timestamp` field (unlike etcd's Node) because the JetStream provider uses local timeout-based crash detection and needs to track when each member was last seen

- `cluster/clusterproviders/etcd/singleton.go` — copy this pattern exactly for `SingletonScheduler`. It defines `RoleType` (Leader/Follower), `SingletonScheduler` struct with mutex-protected `props`/`pids` slices, `FromFunc()`/`FromProducer()` methods, and `OnRoleChanged()` that spawns on Leader and poisons on Follower.

- `cluster/clusterproviders/etcd/config.go` — follow the options pattern: `type Option func(*config)`, `WithXxx()` functions, `defaultConfig()`.

**Identity lookup patterns to follow:**

- `cluster/identitylookup/nats/nats_identity.go` — the existing NATS identity storage uses KV. Our integrated identity lookup is purely stream-based but follows similar logical patterns:
  - `activationRecord` struct with JSON tags for compact serialization
  - Semaphore-based concurrency limiting with `acquire()`/`release()`
  - Member tracking for cleanup on departure

- `cluster/identitylookup/storage/identity_storage_lookup.go` — the generic adapter shows the full `IdentityLookup.Get()` protocol:
  1. Check for existing activation
  2. If client, wait for activation
  3. Try acquire lock
  4. If lock acquired, spawn actor via `cluster.TryGetClusterKind()` + `cluster.WithClusterIdentity()` + `SpawnNamed()`
  5. Store activation, populate `cluster.PidCache`
  6. If lock not acquired, wait for activation
  - `Setup()` subscribes to `ClusterTopology` events to clean up departing members' activations
  - `Shutdown()` calls `RemoveMemberId(self)`
  - `pidFromStored()` parses "address/id" format

**The `cluster.Member` protobuf struct** (defined in `cluster/cluster.pb.go`):
```go
type Member struct {
    Id    string
    Host  string
    Port  int32
    Kinds []string
}
```

**Module structure:**

- Each provider has its own `go.mod` in its directory (see `cluster/clusterproviders/etcd/go.mod` for reference)
- Uses `replace github.com/awevoke/protoactor-go => ../../../` directive
- Target `github.com/nats-io/nats.go v1.48.0`
- Use `github.com/nats-io/nats-server/v2` for embedded server in unit tests
- Use `github.com/testcontainers/testcontainers-go` for integration tests
- Include compile-time interface checks: `var _ cluster.ClusterProvider = (*Provider)(nil)` and `var _ cluster.IdentityLookup = (*IdentityLookup)(nil)`

**NATS JetStream API specifics (nats.go v1.48.0):**

- Stream creation: `js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{Name: name, Subjects: subjects, MaxMsgsPerSubject: 1, MaxAge: maxAge, AllowMsgTTL: true, SubjectDeleteMarkerTTL: markerTTL, ...})`
- Publish with per-message TTL: `js.Publish(ctx, subject, data, jetstream.WithMsgTTL(duration))` — this is how heartbeat messages get their TTL
- Publish with CAS (expected last subject sequence): `js.Publish(ctx, subject, data, jetstream.WithExpectLastSubjectSequence(seq))` — use seq=0 for atomic create (like KV Create). This returns `*jetstream.PubAck` with the new sequence number on success, or an error if the sequence doesn't match.
- Get last message for subject: `stream.GetLastMsgForSubject(ctx, subject)` — returns `*jetstream.RawStreamMsg` with Data, Sequence, Time fields
- Ordered consumer: `js.OrderedConsumer(ctx, streamName, jetstream.OrderedConsumerConfig{FilterSubjects: subjects})` — use for real-time event consumption. Call `consumer.Messages()` to get a message iterator.
- Stream purge by subject: `stream.PurgeSubject(ctx, subject)` — use for identity cleanup on member departure
- `jetstream.ErrNoMessages` — returned when no messages match (e.g., GetLastMsgForSubject on empty subject)
- The `Nats-Expected-Last-Subject-Sequence` header is set automatically by `WithExpectLastSubjectSequence()`. When the expectation fails, the publish returns an error containing "wrong last sequence".

**Key difference from KV provider — crash detection is local, not server-driven:**

Unlike the KV provider where the server deletes expired keys and watchers see delete events, the JetStream provider's crash detection is based on local timeout tracking:
- Each member maintains a `map[string]time.Time` mapping memberID to last heartbeat time
- A periodic goroutine (the "stale checker") iterates this map every `checkInterval` and removes members whose last heartbeat exceeds `memberTimeout`
- When the consumer receives a heartbeat message, it updates the map with `time.Now()` (not the message timestamp, to avoid clock skew issues between NATS server and client)
- When the consumer receives a leave event, it immediately removes the member
- On startup, when replaying existing stream messages, use the NATS message timestamp to filter out stale members (messages older than `memberTimeout` are ignored)

**Leader election via publish-race CAS — detailed mechanics:**

- All members periodically attempt to claim leadership by publishing to `<prefix>.leader`
- The first attempt uses `WithExpectLastSubjectSequence(0)` (no prior message)
- The current leader refreshes by publishing with `WithExpectLastSubjectSequence(lastSeq)` where `lastSeq` is the sequence from the previous successful publish
- Non-leaders periodically attempt with seq=0, which will fail as long as the leader's message exists
- When the leader's message expires via TTL, the subject effectively resets — the next publish with seq=0 succeeds
- On successful publish, set local role to Leader; on failure, set to Follower
- The leader election attempt interval should be slightly shorter than the leader TTL (e.g., TTL=10s, attempt every 7s) to ensure timely re-election
- Track the leader's memberID from consumed messages so all members know who the current leader is, not just whether they themselves are leader

**Goroutine safety requirements (from production readiness design at `docs/plans/complete/2026-02-16-cluster-production-readiness-design.md`):**

- Use `atomic.Bool` for shutdown flag, not plain bool
- Use `context.Context` with cancellation for all goroutines
- Use `sync.WaitGroup` for goroutine tracking
- `Shutdown()` must block until all goroutines exit
- Use `defer recover()` in goroutines to prevent panics from crashing the process

**This provider has more goroutines than the KV provider:**

1. Heartbeat publisher (periodic publish to `<prefix>.members.<memberID>`)
2. Stream consumer (ordered consumer processing all cluster events)
3. Stale member checker (periodic scan of lastSeen map)
4. Leader election (periodic attempt to claim/refresh leadership)
5. Role changed notifier (reads from role channel, notifies listeners)

All must respect the shutdown context and be tracked by the WaitGroup.

**Testing specifics:**

- Embedded NATS server for unit tests: `server.New(&server.Options{JetStream: true, Port: -1, StoreDir: t.TempDir()})` — use `-1` for random port
- Connect with `nats.Connect(srv.ClientURL())`
- For testcontainers, use the `nats:latest` image with JetStream enabled: `-js` flag
- Run `go mod tidy` after creating go.mod
- All tests should use `t.Helper()` in test helpers, `t.Cleanup()` for resource cleanup
- Use `require` and `assert` from `github.com/stretchr/testify`
- The stale member checker and heartbeat timeout make time-sensitive tests tricky — use short intervals in tests (e.g., heartbeat 100ms, TTL 300ms, timeout 500ms, check interval 100ms) and use `require.Eventually` or polling loops instead of `time.Sleep`

**Example structure:**

- Single-node example at `examples/cluster-nats-stream/` — follow the pattern from `examples/nats-jetstream-virtual-actor-ingress/` for docker-compose and main.go structure
- Multi-node example at `examples/cluster-nats-stream-multi/` — docker-compose should define a NATS server service, 3 node services, and 1 client service
- Examples should use the integrated identity lookup (not disthash)
- Examples should demonstrate grain registration with `cluster.WithKinds()` and grain calls via `cluster.GetClusterKind()`

**Important implementation details:**

- The provider's `IdentityLookup()` method returns the integrated lookup. If the user passes a different `IdentityLookup` to `cluster.Configure()`, the integrated one is simply unused.
- On `Setup()` of the identity lookup, subscribe to `ClusterTopology` events from `cluster.ActorSystem.EventStream` to detect member departures and clean up their activations via stream purge.
- The cluster membership stream and the identity stream are SEPARATE streams — different lifecycle, retention, and subject hierarchies.
- The identity stream does NOT use per-message TTL (activations are permanent until explicitly removed). It uses `MaxMsgsPerSubject: 1` for compaction only.
- Identity key format in subjects: replace `/` with `.` since NATS subjects use `.` as delimiter and `/` is not valid in subject tokens. Use the same `kvKey()` pattern as the existing NATS identity lookup.
- For `WaitForActivation` in the identity lookup: create a temporary ordered consumer filtered to the specific identity subject and wait for a message with a completed activation record (PidID is set). Use a context with timeout (the lock TTL).
- For `RemoveMemberId`: the identity stream doesn't have a member tracking mechanism like the KV identity lookup's member bucket. Instead, iterate over the stream looking for messages where `memberID` matches, and purge those subjects. Alternatively, maintain an in-memory map of memberID -> identity subjects, populated during `StoreActivation`.
- The `GetHealthStatus() error` method (seen on etcd provider) is not part of the `ClusterProvider` interface but is a useful addition — include it.
- `MaxMsgsPerSubject: 1` means that when a member publishes a new heartbeat, the old one is automatically replaced. This is the compaction mechanism. However, note that the old message's sequence number is invalidated — consumers that haven't processed it yet will skip it. This is fine for ordered consumers which handle gaps automatically.
