# NATS Examples & Identity Lookup Design

## Overview

Add five NATS-based examples to protoactor-go demonstrating virtual actor ingresses, message forwarding, stream locality with soft-binding, and duplicate activation prevention. Also add two new `StorageLookup` implementations (Postgres, NATS JetStream KV) and a `LocalAffinityStrategy` placement strategy as reusable library packages.

All examples use real nats.go client with docker-compose infrastructure. Each example is a self-contained Go module under `examples/` following existing project conventions.

## Library Packages

### `cluster/identitylookup/postgres/` — Postgres StorageLookup

Implements `cluster.StorageLookup` using PostgreSQL.

- **Table schema:** `cluster_identities` with columns: `key` (kind/identity), `lock_id`, `pid_id`, `pid_address`, `member_id`, `lock_expires_at`
- **Atomic operations:** `INSERT ... ON CONFLICT` and `SELECT ... FOR UPDATE` (no Lua scripts needed)
- **Lock TTL:** `lock_expires_at` timestamp; background goroutine cleans stale locks
- **Member cleanup:** `DELETE WHERE member_id = $1`
- **Concurrency control:** Semaphore pattern matching Redis implementation
- **Constructor:** `postgres.New(clusterName string, db *sql.DB, opts ...Option)`

### `cluster/identitylookup/nats/` — NATS JetStream StorageLookup

Implements `cluster.StorageLookup` using NATS JetStream KV buckets.

- **KV bucket:** `{clusterName}_identities` stores JSON-encoded activation records keyed by `{kind}/{identity}`
- **Locking:** KV `Create` (fails if key exists) for atomic lock acquisition; KV revision as CAS token
- **Lock TTL:** Per-key TTL via JetStream KV
- **WaitForActivation:** KV `Watch` on specific key (event-driven, not polling)
- **Member tracking:** Second KV bucket `{clusterName}_members` maps `{memberID}` to list of identity keys
- **Constructor:** `nats.New(clusterName string, js jetstream.JetStream, opts ...Option)`

### `cluster/placement/` — LocalAffinityStrategy

A `MemberStrategy` for subject-based placement affinity.

- Nodes register bound subjects as metadata (cluster member tags)
- Placement scores members by subject match; falls back to rendezvous hash
- Actors self-migrate: detect remote traffic from forwarder, poison at configurable random interval to respawn on preferred node

## Example 1: NATS Virtual Actor Ingress

**Directory:** `examples/nats-virtual-actor-ingress/`

Demonstrates a NATS core subscriber consuming messages and routing them to virtual actors in a cluster. Direct Go equivalent of the C# Kafka ingress example.

**Structure:**
```
examples/nats-virtual-actor-ingress/
├── docker-compose.yml          # NATS server + Consul
├── go.mod
├── shared/
│   ├── protos.proto            # DeviceMessage, Ack, DeviceState, service Device
│   ├── protos.pb.go
│   ├── protos_grain.pb.go
│   └── build.sh
├── ingress/
│   └── main.go                 # NATS subscriber → cluster.RequestAsync per message
└── node/
    └── main.go                 # Cluster member hosting DeviceActor grain
```

**Flow:**
1. `node/main.go` starts cluster member with `DeviceActor` grain kind (Consul + disthash)
2. `ingress/main.go` connects to NATS, subscribes to `devices.>` wildcard
3. On each message: extract device ID from subject (`devices.{deviceID}`), deserialize protobuf, call `cluster.RequestAsync<Ack>`
4. DeviceActor accumulates state, responds with Ack
5. Separate goroutine publishes simulated device messages

**Proto:**
```protobuf
message DeviceMessage { string data = 1; }
message Ack {}
message DeviceState { string data = 1; int32 message_count = 2; }
service Device {
  rpc HandleMessage(DeviceMessage) returns (Ack) {}
}
```

## Example 2: NATS JetStream Virtual Actor Ingress

**Directory:** `examples/nats-jetstream-virtual-actor-ingress/`

Same concept as Example 1 but with JetStream for durable delivery, consumer groups, and ack-based flow control.

**Structure:**
```
examples/nats-jetstream-virtual-actor-ingress/
├── docker-compose.yml          # NATS (JetStream enabled) + Consul
├── go.mod
├── shared/                     # Same Device service proto
├── ingress/
│   └── main.go                 # JetStream pull consumer → batch to virtual actors
└── node/
    └── main.go                 # Cluster member hosting DeviceActor grain
```

**Key differences from Example 1:**
- Creates JetStream stream `DEVICES` with subject `devices.>`
- Durable pull consumer with `Fetch(batchSize)` for batch processing
- Fan out batch to actors concurrently via `cluster.RequestAsync`
- Ack JetStream messages only after all actors respond — at-least-once semantics
- Un-acked messages redelivered automatically by JetStream
- Throughput measurement (messages/second)

## Example 3: NATS Forwarder

**Directory:** `examples/nats-forwarder/`

General-purpose NATS-to-actor message forwarding with subject-based routing. Shows NATS as a bridge between external systems and the actor world.

**Structure:**
```
examples/nats-forwarder/
├── docker-compose.yml          # NATS + Consul
├── go.mod
├── shared/
│   ├── protos.proto            # Command, CommandResult
│   ├── protos.pb.go
│   └── build.sh
├── forwarder/
│   └── main.go                 # NATS → actor forwarder, subject-based routing
├── worker/
│   └── main.go                 # Cluster member with worker actors
└── publisher/
    └── main.go                 # External publisher sending commands via NATS
```

**Patterns demonstrated:**
- Subject-to-actor routing (`commands.{workerName}.{action}` → actor address)
- Bidirectional integration (consume from NATS, publish results back to `results.{workerName}`)
- NATS request-reply for synchronous command/response through actors
- Forwarder actor as reusable middleware pattern

**Proto:**
```protobuf
message Command {
  string action = 1;
  bytes payload = 2;
  string reply_subject = 3;
}
message CommandResult {
  string worker = 1;
  string action = 2;
  bool success = 3;
  string result = 4;
}
```

## Example 4: NATS Stream Locality / Soft Binding

**Directory:** `examples/nats-stream-locality/`

Multiple cluster nodes each subscribe to wildcard JetStream subjects. A custom `MemberStrategy` places actors preferentially on nodes subscribed to matching subjects. Actors self-migrate on scaling events.

**Structure:**
```
examples/nats-stream-locality/
├── docker-compose.yml          # NATS (JetStream) + Consul
├── go.mod
├── shared/
│   ├── protos.proto            # SensorReading, Ack, service Sensor
│   ├── protos.pb.go
│   ├── protos_grain.pb.go
│   ├── build.sh
│   └── affinity.go             # LocalAffinityStrategy + poison-on-remote middleware
├── node/
│   └── main.go                 # Cluster member: --subjects flag, JetStream consumer
└── publisher/
    └── main.go                 # Publishes sensor data across many subjects
```

**How it works:**
1. Each node starts with `--subjects="sensors.building-a.>"`, registers bound subjects as member tags
2. Node creates JetStream pull consumer filtered to its subjects, forwards to `SensorActor` grains
3. `LocalAffinityStrategy` scores members by subject match; ties broken by rendezvous hash
4. `SensorActor` checks if sender is remote; with configurable probability (e.g. 10%), poisons itself to respawn on preferred node
5. Publisher sends across `sensors.building-a.floor-1.temp`, `sensors.building-b.floor-2.humidity`, etc.

**Demo scenario:** 2 nodes for 2 buildings, actors gradually migrate to achieve full local affinity.

## Example 5: Duplicate Activation Prevention

**Directory:** `examples/nats-identity-prevention/`

Uses Postgres and NATS JetStream `StorageLookup` implementations to prevent duplicate virtual actor activations across a cluster.

**Structure:**
```
examples/nats-identity-prevention/
├── docker-compose.yml          # NATS (JetStream) + Postgres + Consul
├── go.mod
├── shared/
│   ├── protos.proto            # Counter service: Increment, GetCount
│   ├── protos.pb.go
│   ├── protos_grain.pb.go
│   └── build.sh
├── postgres-node/
│   └── main.go                 # Cluster member using postgres.StorageLookup
├── nats-node/
│   └── main.go                 # Cluster member using nats.StorageLookup
└── client/
    └── main.go                 # Concurrent requests to same grain identity
```

**Proto:**
```protobuf
message IncrementRequest { int32 amount = 1; }
message GetCountRequest {}
message CountResponse { int32 count = 1; }
service Counter {
  rpc Increment(IncrementRequest) returns (CountResponse) {}
  rpc GetCount(GetCountRequest) returns (CountResponse) {}
}
```

**How it works:**
1. `postgres-node` uses `storage.New(postgres.New(clusterName, db))` as identity lookup
2. `nats-node` uses `storage.New(nats.New(clusterName, js))` as identity lookup
3. `client` spawns multiple goroutines all requesting the same `Counter` grain concurrently
4. Storage-backed spawn lock ensures single activation; others wait via `WaitForActivation`
5. Count is consistent and monotonically increasing despite concurrent requests
6. `--backend` flag selects `postgres` or `nats` demo scenario

The two node types run independently as separate demo scenarios, not mixed in one cluster.

## Dependencies

- `github.com/nats-io/nats.go` — NATS client
- `github.com/nats-io/nats.go/jetstream` — JetStream API (same module)
- `github.com/lib/pq` or `github.com/jackc/pgx/v5` — Postgres driver (for postgres StorageLookup)
- Existing: `github.com/asynkron/protoactor-go`, consul provider, disthash, protobuf tooling
