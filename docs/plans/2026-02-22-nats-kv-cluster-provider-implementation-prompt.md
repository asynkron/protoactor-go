# Implementation Planning Prompt: NATS KV Cluster Provider

Use this prompt in a new session to create a detailed implementation plan using the `writing-plans` skill.

---

## Prompt

I need to create a detailed implementation plan for a new NATS KV-based cluster provider for our protoactor-go fork. The design document is at `docs/plans/2026-02-22-nats-kv-cluster-provider-design.md` — read it first.

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
  - `startKeepAlive()` goroutine with shutdown check loop
  - `startRoleChangedNotifyLoop()` goroutine that reads from a role channel

- `cluster/clusterproviders/etcd/node.go` — the `Node` struct pattern. Create an equivalent for NATS KV:
  - JSON-serializable struct with ID, Name, Host, Address, Port, Kinds, Meta, Alive fields
  - `MemberStatus() *cluster.Member` converter method
  - `Serialize()`/`Deserialize()` methods
  - `NewNode()` and `NewNodeFromBytes()` constructors

- `cluster/clusterproviders/etcd/singleton.go` — copy this pattern exactly for `SingletonScheduler`. It defines `RoleType` (Leader/Follower), `SingletonScheduler` struct with mutex-protected `props`/`pids` slices, `FromFunc()`/`FromProducer()` methods, and `OnRoleChanged()` that spawns on Leader and poisons on Follower.

- `cluster/clusterproviders/etcd/config.go` — follow the options pattern: `type Option func(*config)`, `WithXxx()` functions, `defaultConfig()`.

**Identity lookup patterns to follow:**

- `cluster/identitylookup/nats/nats_identity.go` — the existing NATS identity storage. Our integrated identity lookup should follow similar patterns but implement `cluster.IdentityLookup` directly instead of `cluster.StorageLookup`:
  - `kvKey()` helper that replaces `/` with `.` in cluster identity keys (NATS KV keys cannot contain `/`)
  - Semaphore-based concurrency limiting with `acquire()`/`release()`
  - `activationRecord` struct with JSON tags for compact serialization
  - Atomic `Create` for lock acquisition, revision-based `Update` for CAS
  - `Watch` with timeout for `WaitForActivation`
  - Member tracking bucket for cleanup on member departure

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
- Uses `replace github.com/asynkron/protoactor-go => ../../../` directive
- Target `github.com/nats-io/nats.go v1.48.0`
- Use `github.com/nats-io/nats-server/v2` for embedded server in unit tests
- Use `github.com/testcontainers/testcontainers-go` for integration tests
- Include compile-time interface checks: `var _ cluster.ClusterProvider = (*Provider)(nil)` and `var _ cluster.IdentityLookup = (*IdentityLookup)(nil)`

**NATS KV API specifics (nats.go v1.48.0):**

- `js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: name, TTL: maxAge, ...})` — use `TTL` field on KeyValueConfig as a safety net, but rely on per-key TTL for member liveness
- `kv.Create(ctx, key, value)` — atomic create, returns `jetstream.ErrKeyExists` if key exists (used for leader election and identity lock)
- `kv.Put(ctx, key, value, jetstream.KeyTTL(duration))` — put with per-key TTL (used for member heartbeat refresh)
- `kv.Update(ctx, key, value, revision)` — CAS update (used for identity activation storage)
- `kv.Get(ctx, key)` — get current value and revision
- `kv.Delete(ctx, key)` — delete key
- `kv.Watch(ctx, keyPattern, opts...)` — watch for changes. Options: `jetstream.UpdatesOnly()`, `jetstream.IncludeHistory()`
- `kv.WatchAll(ctx, opts...)` — watch all keys
- The KV bucket must be created with `AllowMsgTTL: true` in the underlying stream config for per-key TTL to work. This is set via `KeyValueConfig` — check if there's a direct field or if you need to access the underlying stream config.
- When a key expires via TTL, the watcher receives a delete event. The bucket should have `SubjectDeleteMarkerTTL` configured so watchers see expiry events.

**Goroutine safety requirements (from production readiness design at `docs/plans/complete/2026-02-16-cluster-production-readiness-design.md`):**

- Use `atomic.Bool` for shutdown flag, not plain bool
- Use `context.Context` with cancellation for all goroutines
- Use `sync.WaitGroup` for goroutine tracking
- `Shutdown()` must block until all goroutines exit
- Use `defer recover()` in goroutines to prevent panics from crashing the process

**Testing specifics:**

- Embedded NATS server for unit tests: `server.New(&server.Options{JetStream: true, Port: -1, StoreDir: t.TempDir()})` — use `-1` for random port
- Connect with `nats.Connect(srv.ClientURL())`
- For testcontainers, use the `nats:latest` image with JetStream enabled: `-js` flag
- Run `go mod tidy` after creating go.mod
- All tests should use `t.Helper()` in test helpers, `t.Cleanup()` for resource cleanup
- Use `require` and `assert` from `github.com/stretchr/testify`

**Example structure:**

- Single-node example at `examples/cluster-nats-kv/` — follow the pattern from `examples/nats-jetstream-virtual-actor-ingress/` for docker-compose and main.go structure
- Multi-node example at `examples/cluster-nats-kv-multi/` — docker-compose should define a NATS server service, 3 node services, and 1 client service
- Examples should use the integrated identity lookup (not disthash)
- Examples should demonstrate grain registration with `cluster.WithKinds()` and grain calls via `cluster.GetClusterKind()`

**Important implementation details:**

- The provider's `IdentityLookup()` method returns the integrated lookup. If the user passes a different `IdentityLookup` to `cluster.Configure()`, the integrated one is simply unused.
- On `Setup()` of the identity lookup, subscribe to `ClusterTopology` events from `cluster.ActorSystem.EventStream` to detect member departures and clean up their activations.
- The leader election key and member keys should be in the SAME KV bucket to reduce the number of buckets. Use the key prefix to separate them (`<prefix>.members.*` vs `<prefix>.leader`).
- Identity activations go in a SEPARATE bucket because they have different lifecycle characteristics (no TTL on activations, different retention needs).
- When the watcher receives initial values on startup (before `UpdatesOnly` kicks in), process them all to build the initial member list before calling `UpdateClusterTopology` for the first time.
- The `GetHealthStatus() error` method (seen on etcd provider) is not part of the `ClusterProvider` interface but is a useful addition — include it.
