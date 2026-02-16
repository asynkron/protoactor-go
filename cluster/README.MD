# Proto.Actor Cluster - Virtual Actors

## Massively distributed actors for Go

Proto.Actor supports the classic actor model also found in Erlang and Akka.
The cluster package uses a different approach: the **Virtual Actor Model**.

In this model each actor appears to *always exist*. There is no explicit lifecycle
as in the classic actor model. You obtain a reference to an actor by requesting
its identity and kind; if the actor does not yet exist, the cluster creates it
for you transparently.

```go
res, err := c.Request("user-123", "UserGrain", &GetProfile{})
```

See [Microsoft Orleans](http://dotnet.github.io/orleans/) for more background
on the Virtual Actor Model.

## Quick start

### 1. Create an actor system and cluster configuration

Every cluster needs an `ActorSystem`, a `ClusterProvider` (membership backend),
an `IdentityLookup` (placement strategy), and a `remote.Config`.

```go
package main

import (
    "fmt"

    "github.com/asynkron/protoactor-go/actor"
    "github.com/asynkron/protoactor-go/cluster"
    "github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
    "github.com/asynkron/protoactor-go/remote"
)

func main() {
    system := actor.NewActorSystem()

    provider := // ... see provider examples below
    lookup   := disthash.New()
    remoteCfg := remote.Configure("localhost", 0)

    cfg := cluster.Configure(
        "my-cluster",
        provider,
        lookup,
        remoteCfg,
        cluster.WithKinds(
            cluster.NewKind("HelloGrain", actor.PropsFromProducer(func() actor.Actor {
                return &HelloActor{}
            })),
        ),
    )

    c := cluster.NewCluster(system, cfg)

    if err := c.StartMember(); err != nil {
        panic(err)
    }
    defer c.Shutdown(true)

    res, err := c.Request("abc", "HelloGrain", &HelloRequest{Name: "Proto.Actor"})
    if err != nil {
        panic(err)
    }
    fmt.Println(res)
}
```

### 2. Choose a cluster provider

#### Consul

```go
import "github.com/asynkron/protoactor-go/cluster/clusterproviders/consul"

provider, err := consul.New()
if err != nil {
    panic(err)
}
```

#### Kubernetes

```go
import "github.com/asynkron/protoactor-go/cluster/clusterproviders/k8s"

provider, err := k8s.New()
if err != nil {
    panic(err)
}
```

#### Automanaged (development / testing)

```go
import "github.com/asynkron/protoactor-go/cluster/clusterproviders/automanaged"

provider := automanaged.New()
```

Other providers: **etcd** (`clusterproviders/etcd`) and **ZooKeeper** (`clusterproviders/zk`).

## Protobuf IDL Definition

Define your messages and grain contracts using Protobuf IDL files.

Here is an example from `/examples/cluster/shared`:

```proto
syntax = "proto3";
package shared;

message HelloRequest {
  string name = 1;
}

message HelloResponse {
  string message = 1;
}

service Hello {
  rpc SayHello (HelloRequest) returns (HelloResponse) {}
}
```

Generate Go code with the `protoc` compiler:

```bash
# generate messages
protoc -I=. --go_out=. protos.proto
# generate grains
protoc -I=. --gorleans_out=. protos.proto
```

## Implementing grains

A grain is a Go struct that implements the generated service interface. The
cluster manages its lifecycle automatically.

```go
package shared

type hello struct{}

func (*hello) SayHello(r *HelloRequest) *HelloResponse {
    return &HelloResponse{Message: "hello " + r.Name}
}

func init() {
    HelloFactory(func() Hello { return &hello{} })
}
```

## Configuration reference

Use `cluster.Configure(...)` to build a `*Config`. Pass functional options to
override defaults. For a non-panicking variant use `cluster.ConfigureWithError(...)`.

```go
cfg := cluster.Configure(
    "my-cluster",      // cluster name (required)
    provider,          // ClusterProvider (required)
    lookup,            // IdentityLookup (required)
    remoteCfg,         // *remote.Config (required)
    // ...functional options...
)
```

### Config options

| Option | Config field | Default | Description |
|--------|-------------|---------|-------------|
| `WithRequestTimeout(d)` | `RequestTimeoutTime` | `5s` | Timeout for grain RPC requests. |
| `WithRequestsLogThrottlePeriod(d)` | `RequestsLogThrottlePeriod` | `2s` | Period over which request log events are throttled. |
| `WithMaxNumberOfEventsInRequestLogThrottlePeriod(n)` | `MaxNumberOfEventsInRequestLogThrottledPeriod` | `3` | Max log events emitted per throttle period. |
| `WithRequestLog(enabled)` | `RequestLog` | `false` | Enable detailed request logging. |
| `WithKinds(kinds...)` | `Kinds` | (empty) | Register grain kinds the member can host. |
| `WithClusterContextProducer(p)` | `ClusterContextProducer` | default context | Factory for the cluster request context. |
| `WithMemberStrategyBuilder(b)` | `MemberStrategyBuilder` | default strategy | Custom strategy for member selection per kind. |
| `WithHeartbeatExpiration(d)` | `HeartbeatExpiration` | `20s` | If a member does not update its heartbeat within this period it is added to the block list. |
| `WithPubSubSubscriberTimeout(d)` | `PubSubConfig.SubscriberTimeout` | `5s` | Timeout when delivering a message batch to a subscriber. |
| `WithGossipInterval(d)` | `GossipInterval` | `300ms` | Interval between gossip protocol rounds. |
| `WithGossipRequestTimeout(d)` | `GossipRequestTimeout` | `500ms` | Timeout for individual gossip requests. |
| `WithGossipFanOut(n)` | `GossipFanOut` | `3` | Number of peers contacted per gossip round. |
| `WithGossipMaxSend(n)` | `GossipMaxSend` | `50` | Maximum number of state entries sent per gossip round. |

### Gossip parameters

The cluster uses a gossip protocol to propagate membership state, heartbeats,
and topology information. Tuning gossip affects convergence speed, network
overhead, and failure-detection latency.

| Parameter | Effect |
|-----------|--------|
| **GossipInterval** (`300ms`) | Lower values speed up convergence but increase network traffic. Raising this reduces bandwidth usage at the cost of slower state propagation. |
| **GossipRequestTimeout** (`500ms`) | How long to wait for a single gossip exchange. Should be greater than the expected network round-trip time. |
| **GossipFanOut** (`3`) | More peers per round means faster convergence and higher resilience to message loss, but proportionally more network traffic. |
| **GossipMaxSend** (`50`) | Caps the number of state entries transmitted in a single round. Increase when the cluster carries a large amount of custom gossip state. |
| **HeartbeatExpiration** (`20s`) | After this period without a heartbeat update a member is considered dead and added to the block list. Lower values detect failures faster but increase the risk of false positives on slow networks. |

## Pub/Sub

The cluster includes a built-in pub/sub system backed by virtual actors
(topic actors). Messages are delivered to subscribers via the cluster's
identity and routing infrastructure.

### Limitations

- **At-most-once delivery** -- messages are delivered in-memory through the
  cluster and are not persisted. If a subscriber is unavailable when a batch
  is delivered, those messages are lost.
- **No persistence** -- topic state (subscriber lists) is held in memory by
  default. Provide a custom `KeyValueStore[*Subscribers]` to persist
  subscriptions across restarts.
- **Subscriber timeout** -- delivery to a subscriber times out after the
  configured `SubscriberTimeout` (default `5s`). Slow consumers may miss
  messages.

### Using the batching producer

`BatchingProducer` accumulates messages and publishes them in batches to
reduce network overhead:

```go
producer := c.BatchingProducer("my-topic",
    cluster.WithBatchingProducerBatchSize(100),
    cluster.WithBatchingProducerMaxQueueSize(5000),
    cluster.WithBatchingProducerPublishTimeout(10*time.Second),
)
defer producer.Dispose()

info, err := producer.Produce(ctx, &MyEvent{Payload: "data"})
```

## FAQ

### What about performance?

Proto.Actor Remoting can pass over 1 million messages per second on a standard
dev machine. The cluster uses the same remoting infrastructure. The cluster API
is request/response, so throughput depends on latency. Use async fire-and-forget
for maximum throughput and request/response when you need a reply.
