# NATS KV Cluster Provider Design

**Date:** 2026-02-22
**Provider:** NATS KV (KeyValue buckets)
**NATS Server:** Minimum 2.11, optional 2.12 features
**Client Library:** nats.go v1.48.0

## Background

The protoactor-go cluster package currently has 5 cluster providers (Consul, etcd, ZooKeeper, Kubernetes, automanaged) but none based on NATS. A NATS identity lookup already exists (`cluster/identitylookup/nats/`) using JetStream KV, but there is no NATS-based cluster provider for member discovery, health checking, or leader election.

This design adds a NATS KV-based cluster provider that uses KeyValue buckets for member registration and discovery via KV watches. It includes an integrated identity lookup to avoid duplicating member state across separate provider and identity components.

## Architecture

### Approach

Members write a key per member with a short TTL. A KV watcher on a wildcard pattern detects joins (Put events), leaves (Delete events), and crashes (TTL expiry with limit markers). Leader election uses atomic `Create` on a dedicated key.

### Package Structure

```
cluster/clusterproviders/natskv/
├── natskv_provider.go           # ClusterProvider implementation
├── natskv_identity.go           # Integrated IdentityLookup
├── config.go                    # Options pattern
├── singleton.go                 # SingletonScheduler + RoleChangedListener
├── natskv_provider_test.go      # Unit tests
├── natskv_identity_test.go      # Identity lookup tests
├── natskv_integration_test.go   # Integration tests (testcontainers)
└── go.mod                       # Separate module (optional dependency)
```

### Constructor API

```go
// Accept *nats.Conn
provider, err := natskv.New(conn, natskv.WithBucketName("my_cluster_members"), ...)

// Accept jetstream.JetStream
provider, err := natskv.NewFromJetStream(js, ...)

// Integrated identity lookup (default)
lookup := provider.IdentityLookup()

// Override with external identity lookup if desired
clusterConfig := cluster.Configure("my-cluster", provider, disthash.New(), remoteConfig)
```

## Member Registration & Discovery

### KV Bucket Layout

Bucket name configurable, default: `protoactor_<clusterName>_members`.

| Key | Value | TTL |
|-----|-------|-----|
| `<prefix>.members.<memberID>` | JSON: `{id, host, port, kinds[], seq}` | Configurable (default 5s) |
| `<prefix>.leader` | JSON: `{memberID, electedAt}` | Configurable (default 10s) |

Key prefix configurable, default: `cluster`.

### StartMember Flow

1. Connect to JetStream, create/bind KV bucket with `AllowMsgTTL: true`
2. Serialize self as JSON, `Put` to `<prefix>.members.<memberID>` with `KeyTTL`
3. Start refresh goroutine (periodically re-`Put` with TTL to keep key alive)
4. Start watcher on `<prefix>.members.>` to detect all member changes
5. On initial watch, load all existing members -> call `cluster.MemberList.UpdateClusterTopology()`
6. On watch events (Put/Delete/Purge), rebuild member list -> call `UpdateClusterTopology()`
7. Attempt leader election

### StartClient Flow

Same as StartMember but skips self-registration and refresh. Watch-only mode.

### Shutdown Flow

1. Cancel refresh goroutine
2. Delete own member key
3. If leader, delete leader key
4. Stop watcher
5. Notify `UpdateClusterTopology` with self removed

## Health Checking / Crash Detection

- Members refresh their key every `refreshInterval` (default 2s) with TTL of `memberTTL` (default 5s)
- If a member crashes, its key expires after the TTL
- NATS KV emits a delete marker (via `SubjectDeleteMarkerTTL`) which the watcher receives
- Watcher removes the expired member from the local member list and calls `UpdateClusterTopology`
- Graceful shutdown explicitly deletes the key for faster detection

## Leader Election

- Use atomic `Create` on `<prefix>.leader` key with a TTL
- First member to successfully `Create` becomes leader, others get `ErrKeyExists`
- Leader refreshes the key periodically (like member keys)
- All members watch `<prefix>.leader`:
  - On delete/expiry: all members race to `Create` again
  - On put: read new leader, update local role
- Role changes trigger `RoleChangedListener.OnRoleChanged(Leader/Follower)`
- `SingletonScheduler` spawns/poisons actors based on role (same pattern as etcd/ZK)

## Integrated Identity Lookup

### Additional KV Bucket

Default: `protoactor_<clusterName>_identities`.

| Key | Value | Purpose |
|-----|-------|---------|
| `<prefix>.identities.<kind>.<identity>` | JSON: `{lockID, pidID, pidAddress, memberID}` | Activation record |

### Operations

- Shares the same `jetstream.JetStream` handle as the provider
- `TryAcquireLock`: `Create` on identity key (atomic, fails if exists)
- `StoreActivation`: `Update` with revision check (CAS)
- `TryGetExistingActivation`: `Get` on identity key
- `WaitForActivation`: `Watch` on identity key until activation appears
- `RemoveMemberId`: scan and delete all activations owned by a member
- Implements `IdentityLookup` interface directly (not via the generic `storage` adapter)

## Configuration

```go
type Config struct {
    // Connection
    ClusterName     string            // Required

    // KV Bucket
    BucketName      string            // Default: "protoactor_{clusterName}_members"
    IdentityBucket  string            // Default: "protoactor_{clusterName}_identities"
    KeyPrefix       string            // Default: "cluster"
    Replicas        int               // KV bucket replicas (default: 1)

    // Timing
    MemberTTL       time.Duration     // Key TTL (default: 5s)
    RefreshInterval time.Duration     // Refresh frequency (default: 2s)
    LeaderTTL       time.Duration     // Leader key TTL (default: 10s)

    // Identity
    LockTTL         time.Duration     // Spawn lock TTL (default: 5s)
    MaxConcurrency  int               // Concurrent identity ops (default: 200)

    // Leader Election
    RoleChangedListener RoleChangedListener
}
```

## Error Handling

### NATS Connection Loss

- Relies on nats.go client's built-in reconnection logic (user configures reconnect options on their `*nats.Conn`)
- During disconnection, log warnings and continue running -- heartbeat publishes fail but client auto-retries on reconnect
- On reconnect, re-establish watchers and perform full state reconciliation (rebuild member list from current KV state)
- If connection is permanently lost (closed, not reconnecting), log error and trigger shutdown

### Stale State on Reconnect

- Watcher restarts from latest revision, full key scan to rebuild state

### JetStream Unavailability

- `StartMember`/`StartClient` return an error immediately -- fail fast
- If bucket disappears mid-operation, log error and attempt recreation with backoff

### Leader Election Edge Cases

- Split-brain: TTL-based leadership ensures convergence -- the member that can't refresh loses leadership
- No leader: if all leader claims expire and no member can write (NATS down), singleton actors are poisoned on all nodes. Leadership re-established automatically when NATS recovers

## Observability

Uses `log/slog` with structured fields:

```
slog.String("provider", "natskv")
slog.String("cluster", clusterName)
slog.String("member", memberID)
```

Key log events:
- `INFO`: member joined, member left, leader elected, leader stepped down, topology updated
- `WARN`: heartbeat refresh failed (transient), NATS reconnecting, stale member detected
- `ERROR`: NATS connection lost, bucket creation failed, identity lock contention exceeded

## Goroutine Lifecycle

- `atomic.Bool` for shutdown flag
- `context.Context` with cancellation for all goroutines
- `sync.WaitGroup` to track goroutine completion during shutdown
- `Shutdown()` blocks until all goroutines exit

## Thread Safety

- Provider struct fields protected by `sync.RWMutex` where needed (member map)
- KV operations are inherently safe (server-side coordination)
- Identity lookup uses semaphore for concurrency limiting

## Testing

### Unit Tests (Embedded NATS Server)

- `TestStartMember_RegistersSelfInKV` -- verify key created with correct value and TTL
- `TestStartMember_DiscoversExistingMembers` -- pre-populate KV, verify initial topology
- `TestStartClient_WatchOnly` -- verify no key written, topology received
- `TestShutdown_Graceful` -- verify key deleted, watcher stopped
- `TestShutdown_RemovesLeaderKey` -- leader shutdown releases leadership
- `TestMemberCrash_TTLExpiry` -- stop refreshing, verify key expires and watcher detects removal
- `TestLeaderElection_FirstWins` -- two members race, one becomes leader
- `TestLeaderElection_Failover` -- leader crashes, follower takes over
- `TestRoleChangedListener_Called` -- verify callback fires on role transitions
- `TestSingletonScheduler_SpawnOnLeader` -- actors spawn when becoming leader, poison on follower
- `TestIdentityLookup_AcquireLock` -- atomic create succeeds first time, fails on duplicate
- `TestIdentityLookup_StoreActivation` -- CAS update succeeds with correct revision
- `TestIdentityLookup_ConcurrentActivation` -- two nodes racing for same identity, one wins
- `TestIdentityLookup_RemoveMember` -- all activations for a member cleaned up
- `TestConfigDefaults` -- verify all defaults applied correctly
- `TestCustomBucketName` -- verify configurable bucket name used
- `TestCustomKeyPrefix` -- verify configurable key prefix used

### Integration Tests (Testcontainers)

- `TestIntegration_TwoMemberCluster` -- two members discover each other, topology converges
- `TestIntegration_MemberJoinLeave` -- third member joins, then leaves gracefully
- `TestIntegration_MemberCrash` -- kill a member process, verify others detect departure
- `TestIntegration_LeaderElection_ThreeNodes` -- three members, one elected leader
- `TestIntegration_LeaderFailover` -- kill leader, verify new leader elected
- `TestIntegration_GrainActivation` -- activate a virtual actor, verify PID resolution
- `TestIntegration_GrainFailover` -- kill member hosting grain, verify re-activation on another member
- `TestIntegration_TopologyConsensus` -- verify gossip topology hash converges across members

### Conformance Tests

Runs the shared `ClusterProvider` conformance test suite.

## Examples

### Single-Node

```
examples/cluster-nats-kv/
├── main.go              # Single node with NATS KV provider + integrated identity
├── docker-compose.yml   # NATS server
└── README.md
```

### Multi-Node

```
examples/cluster-nats-kv-multi/
├── node/main.go         # Cluster member (run multiple instances)
├── client/main.go       # Cluster client that makes grain calls
├── shared/protos.go     # Shared protobuf definitions
├── docker-compose.yml   # NATS server + 3 cluster nodes + client
└── README.md
```

Demonstrates: multi-member discovery, leader election, singleton actors, grain placement across nodes, member join/leave with topology updates, failover behavior.

## Documentation

- Per-provider README at `cluster/clusterproviders/natskv/README.md`
- Quick start, configuration reference, NATS server version requirements
- Architecture overview of member discovery, health checking, leader election
- Update cluster README provider comparison table
