# Cluster Providers

Proto.Actor's cluster module supports pluggable providers for member discovery and
topology management. Each provider implements the `cluster.ClusterProvider`
interface (`StartMember`, `StartClient`, `Shutdown`) and is responsible for
registering nodes, watching for membership changes, and publishing topology
updates to the cluster's `MemberList`.

This document compares the five built-in providers and offers guidance on when to
use each one.

---

## Provider Comparison

| Feature | Consul | Kubernetes | etcd | ZooKeeper | Automanaged |
|---|---|---|---|---|---|
| External dependency | Consul agent | Kubernetes API | etcd cluster | ZooKeeper ensemble | None |
| Health checking | TTL-based via Consul agent | Pod readiness probes | Lease keep-alive | Ephemeral znodes + session | HTTP `/_health` polling |
| Membership discovery | Consul service catalog (blocking queries) | Pod label watching | Key prefix watching | ZNode children watching | HTTP health endpoint polling |
| Leader election | No (use Consul sessions separately) | No | Yes (lowest lease sequence) | Yes (lowest ephemeral sequence) | No |
| Singleton scheduling | No | No | Yes (`SingletonScheduler`) | Yes (`RoleChangedListener`) | No |
| Client mode | Yes | Yes | Yes | Yes | Yes |
| Configuration options | TTL, refresh interval, deregister timeout, blocking wait time | Namespace (auto-detected) | Base key, keep-alive TTL, retry interval, role listener | Base key, session timeout, auth, role listener | Refresh TTL, auto-manage port, host list |
| Production ready | Yes | Yes | Yes | Yes | No (development/testing only) |

---

## When to Use Each Provider

### Consul

**Best for:** General-purpose service discovery, multi-datacenter deployments,
environments already running HashiCorp tooling.

Choose Consul when:

- You already run Consul for service mesh or configuration management.
- You need TTL-based health checks with automatic deregistration of failed nodes
  (configurable via `deregisterCritical`).
- You want blocking long-poll queries for near-real-time topology updates
  without constant polling overhead.
- You operate across multiple data centers and want Consul's built-in WAN
  federation support.

The Consul provider registers each node as a Consul service with TTL health
checks. It uses Consul's blocking query mechanism (`WaitIndex`) to efficiently
detect membership changes. When a node fails to refresh its TTL, Consul marks it
critical and eventually deregisters it automatically.

### Kubernetes (k8s)

**Best for:** Kubernetes-native deployments where you want zero extra
infrastructure for cluster discovery.

Choose Kubernetes when:

- Your application already runs inside Kubernetes pods.
- You want to avoid deploying and managing a separate discovery service.
- You can rely on Kubernetes pod readiness probes for health checking.
- You need the provider to integrate naturally with Kubernetes rolling updates
  and pod lifecycle events.

The Kubernetes provider uses the Kubernetes API to watch pods matching a label
selector (`cluster.proto.actor/cluster=<name>`). It registers membership
information by patching pod labels and detects topology changes through the
Kubernetes watch API. Pods are only included as members when all their containers
report ready status.

### etcd

**Best for:** Environments that already run etcd, or when you need built-in
leader election and singleton actor scheduling.

Choose etcd when:

- You already operate an etcd cluster (e.g., as part of Kubernetes control plane
  infrastructure you have access to).
- You need built-in leader election: the etcd provider tracks leadership by
  comparing lease sequence numbers and exposes a `RoleChangedListener` callback.
- You want to run singleton actors that are spawned only on the leader node,
  using the included `SingletonScheduler`.
- You prefer lease-based registration with automatic expiry on node failure.

The etcd provider stores node information under a configurable base key
(default: `/protoactor/<cluster-name>/`). Each node acquires an etcd lease and
periodically refreshes it via `KeepAlive`. When a node crashes or loses
connectivity, the lease expires and etcd automatically removes the key,
triggering a watch notification to remaining members.

### ZooKeeper (zk)

**Best for:** Legacy environments running ZooKeeper, or when you need strong
consistency guarantees for membership and leader election.

Choose ZooKeeper when:

- Your infrastructure already includes a ZooKeeper ensemble (e.g., for Kafka,
  Hadoop, or other Apache projects).
- You need strong consistency for membership data -- ZooKeeper provides
  linearizable writes and sequential consistency for reads.
- You need built-in leader election: like the etcd provider, ZooKeeper uses
  ephemeral sequential znodes to elect a leader based on the lowest sequence
  number.
- You want automatic member cleanup when sessions expire, thanks to ZooKeeper's
  ephemeral nodes.

The ZooKeeper provider creates ephemeral sequential znodes under a cluster path
(default: `/protoactor/<cluster-name>/`). When a node's session is lost,
ZooKeeper automatically removes its znode. The provider watches for children
changes on the cluster path and re-evaluates leadership after each topology
change. It also supports authentication via `WithAuth`.

### Automanaged

**Best for:** Local development, integration testing, and quick prototyping.

Choose Automanaged when:

- You are developing locally and do not want to run external infrastructure.
- You are writing integration tests that require a cluster but need minimal
  setup.
- You know all node addresses ahead of time (they are provided as a static host
  list).

The Automanaged provider is **not recommended for production use**. It stores
cluster state in memory and uses a simple HTTP health endpoint (`/_health`) for
peer discovery. Each node starts an HTTP server (using Echo) and periodically
polls all configured hosts to build the cluster topology. There is no automatic
failure detection beyond HTTP request timeouts, and the static host list does not
support dynamic scaling.

---

## Setup Instructions

### Consul Provider

**Prerequisites:** A running Consul agent reachable from your application.

```go
import (
    "github.com/asynkron/protoactor-go/cluster"
    "github.com/asynkron/protoactor-go/cluster/clusterproviders/consul"
    "github.com/hashicorp/consul/api"
)

// Using default Consul configuration (connects to local agent at 127.0.0.1:8500)
provider, err := consul.New()
if err != nil {
    log.Fatal(err)
}

// Or with custom Consul configuration and options
provider, err := consul.NewWithConfig(&api.Config{
    Address: "consul.example.com:8500",
}, consul.WithTTL(5*time.Second), consul.WithRefreshTTL(2*time.Second))
if err != nil {
    log.Fatal(err)
}

clusterConfig := cluster.Configure("my-cluster", provider, /* identity lookup */)
```

**Consul options:**

| Option | Default | Description |
|---|---|---|
| `WithTTL(d)` | 3s | Consul service check TTL |
| `WithRefreshTTL(d)` | 1s | How often the TTL is refreshed |

### Kubernetes Provider

**Prerequisites:** The application must run inside a Kubernetes pod. The pod's
service account needs permission to `get` and `patch` pods, and to `list` and
`watch` pods in its namespace.

```go
import (
    "github.com/asynkron/protoactor-go/cluster"
    "github.com/asynkron/protoactor-go/cluster/clusterproviders/k8s"
)

// Using in-cluster configuration (auto-detected from pod environment)
provider, err := k8s.New()
if err != nil {
    log.Fatal(err)
}

clusterConfig := cluster.Configure("my-cluster", provider, /* identity lookup */)
```

**Required RBAC permissions:**

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: protoactor-cluster
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "watch", "patch"]
```

The provider automatically patches pod labels with cluster metadata using the
`cluster.proto.actor/` label prefix. It watches for pods matching the cluster
name label and only considers pods whose containers are all in `Ready` state.

### etcd Provider

```go
import (
    "github.com/asynkron/protoactor-go/cluster"
    "github.com/asynkron/protoactor-go/cluster/clusterproviders/etcd"
    clientv3 "go.etcd.io/etcd/client/v3"
)

// Using default configuration (connects to 127.0.0.1:2379)
provider, err := etcd.New()
if err != nil {
    log.Fatal(err)
}

// Or with custom etcd configuration
provider, err := etcd.NewWithConfig("/my-app", clientv3.Config{
    Endpoints:   []string{"etcd1:2379", "etcd2:2379", "etcd3:2379"},
    DialTimeout: 5 * time.Second,
}, etcd.WithKeepAliveTTL(5*time.Second))
if err != nil {
    log.Fatal(err)
}

clusterConfig := cluster.Configure("my-cluster", provider, /* identity lookup */)
```

### ZooKeeper Provider

```go
import (
    "github.com/asynkron/protoactor-go/cluster"
    "github.com/asynkron/protoactor-go/cluster/clusterproviders/zk"
)

provider, err := zk.New(
    []string{"zk1:2181", "zk2:2181", "zk3:2181"},
    zk.WithSessionTimeout(10*time.Second),
    zk.WithBaseKey("/my-app"),
)
if err != nil {
    log.Fatal(err)
}

clusterConfig := cluster.Configure("my-cluster", provider, /* identity lookup */)
```

### Automanaged Provider

```go
import (
    "github.com/asynkron/protoactor-go/cluster"
    "github.com/asynkron/protoactor-go/cluster/clusterproviders/automanaged"
)

// Single-node local development
provider := automanaged.New()

// Multi-node with known hosts
provider := automanaged.NewWithConfig(
    2*time.Second, // refresh TTL
    6330,          // auto-manage HTTP port
    "host1:6330", "host2:6330",
)

clusterConfig := cluster.Configure("my-cluster", provider, /* identity lookup */)
```

---

## Decision Flowchart

1. **Running in Kubernetes?** Use the **Kubernetes** provider -- no extra
   infrastructure needed.
2. **Already have Consul?** Use the **Consul** provider -- great health checks,
   blocking queries, and multi-DC support.
3. **Need leader election or singleton actors?** Use **etcd** or **ZooKeeper**,
   depending on which you already operate.
4. **Already have etcd?** Use the **etcd** provider.
5. **Already have ZooKeeper?** Use the **ZooKeeper** provider.
6. **Local development or testing?** Use the **Automanaged** provider.
