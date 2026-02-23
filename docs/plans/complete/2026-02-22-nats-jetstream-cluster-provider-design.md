# NATS JetStream Cluster Provider Design

**Date:** 2026-02-22
**Provider:** NATS JetStream (raw streams/consumers)
**NATS Server:** Minimum 2.11, optional 2.12 features
**Client Library:** nats.go v1.48.0

## Background

The protoactor-go cluster package currently has 5 cluster providers (Consul, etcd, ZooKeeper, Kubernetes, automanaged) but none based on NATS. A NATS identity lookup already exists (`cluster/identitylookup/nats/`) using JetStream KV, but there is no NATS-based cluster provider for member discovery, health checking, or leader election.

This design adds a NATS JetStream-based cluster provider that uses raw JetStream streams and consumers for member registration and discovery. It is purely stream/event-sourced with no KV dependency. It includes an integrated identity lookup to avoid duplicating member state across separate provider and identity components.

## Architecture

### Approach

A dedicated JetStream stream with `MaxMsgsPerSubject: 1` acts as a compacted log. Members publish heartbeats as messages with per-message TTL. An ordered consumer receives all cluster events. A local timeout mechanism detects crashed members whose heartbeats stop arriving. Leader election uses publish-race with CAS semantics via `Nats-Expected-Last-Subject-Sequence` headers.

### Package Structure

```
cluster/clusterproviders/natsstream/
├── natsstream_provider.go           # ClusterProvider implementation
├── natsstream_identity.go           # Integrated IdentityLookup
├── config.go                        # Options pattern
├── singleton.go                     # SingletonScheduler + RoleChangedListener
├── natsstream_provider_test.go      # Unit tests
├── natsstream_identity_test.go      # Identity lookup tests
├── natsstream_integration_test.go   # Integration tests (testcontainers)
└── go.mod                           # Separate module (optional dependency)
```

### Constructor API

```go
// Accept *nats.Conn
provider, err := natsstream.New(conn, natsstream.WithStreamName("MY_CLUSTER"), ...)

// Accept jetstream.JetStream
provider, err := natsstream.NewFromJetStream(js, ...)

// Integrated identity lookup (default)
lookup := provider.IdentityLookup()

// Override with external identity lookup if desired
clusterConfig := cluster.Configure("my-cluster", provider, disthash.New(), remoteConfig)
```

## Stream Layout

Stream name configurable, default: `PROTOACTOR_<clusterName>`.

| Subject | Payload | MaxMsgsPerSubject | Purpose |
|---------|---------|-------------------|---------|
| `<prefix>.members.<memberID>` | JSON: `{id, host, port, kinds[], seq, timestamp}` | 1 | Latest heartbeat/state per member |
| `<prefix>.join.<memberID>` | JSON: `{id, host, port, kinds[]}` | 1 | Join announcement |
| `<prefix>.leave.<memberID>` | JSON: `{id, reason}` | 1 | Graceful leave |
| `<prefix>.leader` | JSON: `{memberID, electedAt, term}` | 1 | Current leader claim |

Subject prefix configurable, default: `cluster.<clusterName>`.

### Stream Configuration

```
MaxMsgsPerSubject: 1          (compacted - only latest per subject)
MaxAge:           1h          (configurable safety net cleanup)
AllowMsgTTL:      true        (per-message TTL for heartbeats)
Retention:        LimitsPolicy
Storage:          FileStorage (configurable)
Replicas:         1           (configurable)
```

## Member Registration & Discovery

### StartMember Flow

1. Connect to JetStream, create/bind stream with subjects `<prefix>.>`
2. Publish join event to `<prefix>.join.<memberID>`
3. Publish initial heartbeat to `<prefix>.members.<memberID>` with per-message TTL
4. Start heartbeat goroutine (periodically publish to `<prefix>.members.<memberID>` with TTL)
5. Create an ordered consumer on the stream to receive all cluster events
6. On startup, replay existing messages to build initial member list:
   - For each `<prefix>.members.*` message, check timestamp -- if within heartbeat window, add to members
   - Skip expired/stale entries
7. Call `cluster.MemberList.UpdateClusterTopology()` with initial set
8. Continue consuming new messages for ongoing topology changes
9. Attempt leader election

### StartClient Flow

Same but skips steps 2-4 (no join, no heartbeats). Consumer-only mode.

### Shutdown Flow

1. Cancel heartbeat goroutine
2. Publish leave event to `<prefix>.leave.<memberID>`
3. If leader, publish empty leader message or let TTL expire
4. Stop consumer
5. Notify `UpdateClusterTopology` with self removed

## Health Checking / Crash Detection

- Members publish heartbeats to `<prefix>.members.<memberID>` every `heartbeatInterval` (default 2s) with per-message TTL (default 5s)
- `MaxMsgsPerSubject: 1` ensures only the latest heartbeat is retained
- When a member crashes, its heartbeat message expires via per-message TTL
- Each member maintains a local map of `memberID -> lastSeenTimestamp`
- A periodic check goroutine (every `checkInterval`, default 3s) scans the map and removes members whose last heartbeat exceeds the `memberTimeout` threshold (default 8s -- slightly longer than TTL to allow for jitter)
- On removal, calls `UpdateClusterTopology` without the stale member
- Graceful shutdown publishes a leave event for immediate detection -- the consumer processes the leave and removes the member without waiting for timeout

## Leader Election

Uses publish-race CAS semantics on `<prefix>.leader` via JetStream publish headers:

- To claim leadership: publish with `Nats-Expected-Last-Subject-Sequence: 0` (expects no prior message). First publisher wins.
- Leader refreshes by publishing with the expected sequence from its last publish
- When leader's message expires (TTL), sequence resets and all members can race again
- Role changes trigger `RoleChangedListener.OnRoleChanged(Leader/Follower)`
- `SingletonScheduler` spawns/poisons actors based on role (same pattern as etcd/ZK)

### Edge Cases

- Split-brain: TTL-based leadership ensures convergence -- the member that can't refresh loses leadership
- No leader: if all leader claims expire and no member can write (NATS down), singleton actors are poisoned on all nodes. Leadership re-established automatically when NATS recovers

## Integrated Identity Lookup

### Additional Stream

Default: `PROTOACTOR_<clusterName>_IDENTITIES`.

| Subject | Payload | MaxMsgsPerSubject | Purpose |
|---------|---------|-------------------|---------|
| `<prefix>.identities.<kind>.<identity>` | JSON: `{lockID, pidID, pidAddress, memberID}` | 1 | Activation record |

### Operations

All operations are purely stream-based using publish headers for atomicity:

- `TryAcquireLock`: Publish with `Nats-Expected-Last-Subject-Sequence: 0` (succeeds only if no existing message for this subject -- atomic create semantics)
- `StoreActivation`: Publish with `Nats-Expected-Last-Subject-Sequence: <lockSeq>` (CAS -- only succeeds if the lock message is still the latest)
- `TryGetExistingActivation`: Use `jetstream.Stream.GetLastMsgForSubject()` to get the latest message for the identity subject
- `WaitForActivation`: Create an ordered consumer filtered to the specific identity subject, wait for an activation message to appear
- `RemoveMemberId`: Publish tombstone/empty messages for all identities owned by the departing member (or use stream purge by subject filter)
- Implements `IdentityLookup` interface directly (not via the generic `storage` adapter)

## Configuration

```go
type Config struct {
    // Connection
    ClusterName       string            // Required

    // Stream
    StreamName        string            // Default: "PROTOACTOR_{clusterName}"
    IdentityStream    string            // Default: "PROTOACTOR_{clusterName}_IDENTITIES"
    SubjectPrefix     string            // Default: "cluster.{clusterName}"
    Replicas          int               // Stream replicas (default: 1)
    Storage           jetstream.StorageType // File or Memory (default: File)
    MaxAge            time.Duration     // Stream max age (default: 1h)

    // Timing
    HeartbeatInterval time.Duration     // Publish frequency (default: 2s)
    HeartbeatTTL      time.Duration     // Per-message TTL (default: 5s)
    MemberTimeout     time.Duration     // Local timeout threshold (default: 8s)
    CheckInterval     time.Duration     // Stale member check frequency (default: 3s)
    LeaderTTL         time.Duration     // Leader message TTL (default: 10s)

    // Identity
    LockTTL           time.Duration     // Spawn lock TTL (default: 5s)
    MaxConcurrency    int               // Concurrent identity ops (default: 200)

    // Leader Election
    RoleChangedListener RoleChangedListener
}
```

## Error Handling

### NATS Connection Loss

- Relies on nats.go client's built-in reconnection logic (user configures reconnect options on their `*nats.Conn`)
- During disconnection, log warnings and continue running -- heartbeat publishes fail but client auto-retries on reconnect
- On reconnect, re-establish consumers and perform full state reconciliation (rebuild member list from current stream state)
- If connection is permanently lost (closed, not reconnecting), log error and trigger shutdown

### Stale State on Reconnect

- Consumer replays from stream, checks message timestamps against heartbeat window

### JetStream Unavailability

- `StartMember`/`StartClient` return an error immediately -- fail fast
- If stream disappears mid-operation, log error and attempt recreation with backoff

## Observability

Uses `log/slog` with structured fields:

```
slog.String("provider", "natsstream")
slog.String("cluster", clusterName)
slog.String("member", memberID)
```

Key log events:
- `INFO`: member joined, member left, leader elected, leader stepped down, topology updated
- `WARN`: heartbeat publish failed (transient), NATS reconnecting, stale member detected
- `ERROR`: NATS connection lost, stream creation failed, identity lock contention exceeded

## Goroutine Lifecycle

- `atomic.Bool` for shutdown flag
- `context.Context` with cancellation for all goroutines
- `sync.WaitGroup` to track goroutine completion during shutdown
- `Shutdown()` blocks until all goroutines exit

## Thread Safety

- Provider struct fields protected by `sync.RWMutex` where needed (member map)
- Stream publish operations are inherently safe (server-side coordination)
- Identity lookup uses semaphore for concurrency limiting

## Testing

### Unit Tests (Embedded NATS Server)

- `TestStartMember_PublishesJoinAndHeartbeat` -- verify messages on correct subjects
- `TestStartMember_ReplaysExistingMembers` -- pre-publish heartbeats, verify initial topology
- `TestStartClient_ConsumerOnly` -- verify no publishes, topology received
- `TestShutdown_PublishesLeave` -- verify leave event published
- `TestMemberCrash_TimeoutDetection` -- stop heartbeats, verify member removed after timeout
- `TestHeartbeat_RefreshesMessage` -- verify `MaxMsgsPerSubject: 1` keeps only latest
- `TestLeaderElection_CAS` -- publish-race with expected sequence, first wins
- `TestLeaderElection_Failover` -- leader stops refreshing, follower wins re-election
- `TestRoleChangedListener_Called` -- verify callback fires
- `TestSingletonScheduler_SpawnOnLeader` -- actors spawn when becoming leader, poison on follower
- `TestIdentityLookup_AcquireLock` -- publish with expected seq 0
- `TestIdentityLookup_StoreActivation` -- publish with expected seq from lock
- `TestIdentityLookup_GetExisting` -- `GetLastMsgForSubject` returns activation
- `TestIdentityLookup_ConcurrentActivation` -- race condition handled correctly
- `TestIdentityLookup_RemoveMember` -- purge by subject filter
- `TestConfigDefaults` -- verify all defaults
- `TestCustomStreamName` -- verify configurable stream name
- `TestCustomSubjectPrefix` -- verify configurable subject prefix
- `TestStreamRetention` -- verify `MaxMsgsPerSubject` and `MaxAge` applied

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
examples/cluster-nats-stream/
├── main.go              # Single node with JetStream provider + integrated identity
├── docker-compose.yml   # NATS server
└── README.md
```

### Multi-Node

```
examples/cluster-nats-stream-multi/
├── node/main.go         # Cluster member (run multiple instances)
├── client/main.go       # Cluster client that makes grain calls
├── shared/protos.go     # Shared protobuf definitions
├── docker-compose.yml   # NATS server + 3 cluster nodes + client
└── README.md
```

Demonstrates: multi-member discovery, leader election, singleton actors, grain placement across nodes, member join/leave with topology updates, failover behavior.

## Documentation

- Per-provider README at `cluster/clusterproviders/natsstream/README.md`
- Quick start, configuration reference, NATS server version requirements
- Architecture overview of member discovery, health checking, leader election
- Update cluster README provider comparison table
