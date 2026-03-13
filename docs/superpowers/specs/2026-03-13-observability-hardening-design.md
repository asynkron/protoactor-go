# Observability Hardening Design

**Date:** 2026-03-13
**Status:** Approved
**Scope:** Metrics expansion, distributed tracing, cross-node trace propagation testing

## Goals

1. Add comprehensive metrics throughout the actor, cluster, remote, and provider layers
2. Ensure cross-node distributed trace propagation works correctly and is tested
3. Add tracing spans for cluster-internal operations (PID resolution, identity lookup, gossip)
4. Add provider-specific metrics for NATS (KV + Stream) and K8s providers
5. Maintain zero overhead when observability is not configured

## Non-Goals

- Changing the existing OTEL middleware architecture
- Adding .NET compatibility for any observability features
- Consul, etcd, or Zookeeper provider metrics (out of scope for production targets)

## Approach

**Approach 2: Extend Existing Per-Layer Metrics + Direct Tracing**

- Add new metrics to existing per-layer metric packages (`metrics/`, `cluster/metrics/`, `remote/metrics/`)
- Create new metric packages for providers (`cluster/clusterproviders/natskv/metrics/`, `k8s/metrics/`)
- Add tracing spans directly in cluster code where middleware doesn't apply (hybrid approach)
- Keep existing middleware for actor-level tracing unchanged

## Metrics

### Message Passing Metrics

Added to `metrics/actor_metrics.go` and `remote/metrics/remote_metrics.go`.

| Metric | Type | Attributes | Description |
|--------|------|------------|-------------|
| `protoactor_actor_message_sent_total` | Counter | `actortype`, `messagetype` | Messages sent by actors |
| `protoactor_actor_message_received_total` | Counter | `actortype`, `messagetype` | Messages received by actors |
| `protoremote_message_batch_size` | Histogram | system labels, `destinationaddress` | Envelopes per batch on remote writes |
| `protoremote_message_size_bytes` | Histogram | system labels, `messagetype` | Serialized payload size in bytes |
| `protoremote_inflight_requests` | UpDownCounter | system labels | Currently pending remote requests/futures |
| `protoremote_message_sent_total` | Counter | system labels, `destinationaddress` | **Opt-in.** Per-destination message counts |
| `protoremote_message_received_total` | Counter | system labels, `sourceaddress` | **Opt-in.** Per-source message counts |

### Cluster Metrics

Added to `cluster/metrics/cluster_metrics.go`.

| Metric | Type | Attributes | Description |
|--------|------|------------|-------------|
| `protocluster_message_sent_total` | Counter | `kind` | Messages sent to virtual actors by cluster kind |
| `protocluster_message_received_total` | Counter | `kind` | Messages received by virtual actors by cluster kind |
| `protocluster_topology_update_total` | Counter | system labels | Topology changes observed by this node |
| `protocluster_member_join_total` | Counter | system labels | Members joined |
| `protocluster_member_leave_total` | Counter | system labels | Members left |
| `protocluster_activation_count` | UpDownCounter | `kind` | Currently active virtual actor activations on this node, broken down by kind. Complements the existing `protocluster_virtualactors` ObservableGauge (which reports total count without kind breakdown). |

### Gossip Metrics

Added to `cluster/metrics/cluster_metrics.go`.

| Metric | Type | Attributes | Description |
|--------|------|------------|-------------|
| `protocluster_gossip_sent_total` | Counter | system labels | Gossip messages sent |
| `protocluster_gossip_received_total` | Counter | system labels | Gossip messages received |
| `protocluster_gossip_roundtrip_duration` | Histogram | system labels | Gossip send round-trip time (seconds) |
| `protocluster_gossip_message_size_bytes` | Histogram | system labels | Size of gossip state payloads |

### Identity / Activation Metrics

Added to `cluster/metrics/cluster_metrics.go`.

| Metric | Type | Attributes | Description |
|--------|------|------------|-------------|
| `protocluster_identity_lookup_duration` | Histogram | `kind` | Time to look up or activate a virtual actor identity |
| `protocluster_identity_lookup_failure_total` | Counter | `kind` | Failed identity lookups |
| `protocluster_identity_cache_hit_total` | Counter | system labels | PID cache hits |
| `protocluster_identity_cache_miss_total` | Counter | system labels | PID cache misses |

### Supervision Metrics

Added to `metrics/actor_metrics.go`.

| Metric | Type | Attributes | Description |
|--------|------|------------|-------------|
| `protoactor_supervision_escalation_total` | Counter | `actortype`, `strategy` | Failures escalated to parent |
| `protoactor_supervision_restart_total` | Counter | `actortype`, `strategy` | Restarts triggered by supervisor strategy |
| `protoactor_supervision_stop_total` | Counter | `actortype`, `strategy` | Actors stopped by supervisor strategy |

### NATS KV Provider Metrics

New package: `cluster/clusterproviders/natskv/metrics/`.

| Metric | Type | Attributes | Description |
|--------|------|------------|-------------|
| `protocluster_natskv_topology_update_duration` | Histogram | system labels | Time to process a topology update |
| `protocluster_natskv_key_refresh_failure_total` | Counter | system labels | Failed member/leader key refreshes |
| `protocluster_natskv_watch_reconnect_total` | Counter | system labels | KV watch reconnections |
| `protocluster_natskv_leader_election_total` | Counter | system labels | Leader election events |

### NATS Stream Provider Metrics

New package: `cluster/clusterproviders/natsstream/metrics/`. Same pattern as NATS KV with `natsstream` prefix.

| Metric | Type | Attributes | Description |
|--------|------|------------|-------------|
| `protocluster_natsstream_topology_update_duration` | Histogram | system labels | Time to process a topology update |
| `protocluster_natsstream_key_refresh_failure_total` | Counter | system labels | Failed member/leader key refreshes |
| `protocluster_natsstream_watch_reconnect_total` | Counter | system labels | Stream watch reconnections |
| `protocluster_natsstream_leader_election_total` | Counter | system labels | Leader election events |

### K8s Provider Metrics

New package: `cluster/clusterproviders/k8s/metrics/`.

| Metric | Type | Attributes | Description |
|--------|------|------------|-------------|
| `protocluster_k8s_pod_watcher_restart_total` | Counter | system labels | Pod watcher restarts |
| `protocluster_k8s_topology_update_duration` | Histogram | system labels | Time to process a pod list update |
| `protocluster_k8s_pod_readiness_duration` | Histogram | system labels | Time from pod seen to pod ready |

### Opt-in Per-Endpoint Metrics

The cross-node address metrics (`protoremote_message_sent_total`, `protoremote_message_received_total`) are gated behind a config flag:

```go
// In remote/config.go
type Config struct {
    // ...existing fields...

    // EnablePerEndpointMetrics enables per-destination/source address
    // metrics (protoremote_message_sent_total, protoremote_message_received_total).
    //
    // WARNING: Cardinality scales O(n^2) with cluster size because each node
    // records a separate time series for every peer it communicates with.
    // For a 10-node cluster this produces ~100 series per metric; at 50 nodes
    // it becomes ~2500. Not recommended for clusters larger than ~20 nodes
    // unless your metrics backend handles high cardinality well.
    //
    // Default: false (disabled).
    EnablePerEndpointMetrics bool
}
```

These metrics are created unconditionally (so the instrument exists), but recording is skipped at the call site when the flag is off. This avoids conditional instrument creation while keeping the hot path zero-cost.

### Opt-in Behavior

All metrics follow the existing pattern: instruments are created via `otel.Meter()`, but values are only recorded when `metricsEnabled` is true on the relevant subsystem (Remote, Cluster). When no OTEL MeterProvider is configured, the global no-op meter is used and all record calls are no-ops at near-zero cost.

**System labels** refers to the `actor.SystemLabels()` function which returns `[]attribute.KeyValue` containing the node's `address` and `id` attributes. These are included on all metrics that are node-scoped to allow per-node filtering and aggregation.

## Tracing

### Existing Middleware (unchanged)

The OTEL middleware in `actor/middleware/opentelemetry/` continues to handle actor-level tracing:

- **ReceiverMiddleware** - creates spans for incoming actor messages, extracts parent context from envelope headers
- **SenderMiddleware** - injects active span context into outbound envelope headers
- **SpawnMiddleware** - propagates parent span to child actors

### Cross-Node Trace Propagation

Trace context already flows across nodes through the existing wire protocol:

1. SenderMiddleware injects span context into `MessageEnvelope.Header` (a `messageHeader` / `map[string]string`)
2. `endpointWriter` serializes headers into `MessageHeader.HeaderData` on the wire
3. `endpointReader` deserializes `HeaderData` back into a `messageHeader` on the `localEnvelope`
4. ReceiverMiddleware on the receiving node extracts span context from the envelope headers

This works because the existing `messageEnvelopeCarrier` in `actor/middleware/opentelemetry/envelope.go` wraps `*MessageEnvelope` and delegates to its `messageHeader` (which is `type messageHeader map[string]string` with `Get`/`Set`/`Keys` methods). The carrier satisfies `propagation.TextMapCarrier` and is what the middleware uses for injection/extraction. No changes needed to the transport layer.

### New Direct Instrumentation (cluster internals)

Spans added directly in cluster code for operations that don't flow through actor message dispatch.

**Cluster Request Path** (`cluster/default_context.go`):

| Span Name | Parent | Attributes |
|-----------|--------|------------|
| `cluster.request` | Caller's context (if available) | `kind`, `messagetype`, `identity` |
| `cluster.resolve_pid` | `cluster.request` | `kind`, `identity` |
| `cluster.activation` | `cluster.resolve_pid` | `kind`, `identity`, `address` |

**Identity Lookup** — spans added in the storage-based identity lookup base layer (`cluster/identitylookup/storage/identity_storage_lookup.go`) which is used by Redis, Postgres, NATS, and in-memory identity implementations. The `disthash` identity lookup uses a placement actor pattern instead and gets tracing through the existing actor middleware, so it does not need direct span instrumentation.

| Span Name | Parent | Attributes |
|-----------|--------|------------|
| `identity.lookup` | `cluster.resolve_pid` | `kind`, `identity`, `provider` |
| `identity.lock_acquire` | `identity.lookup` | `kind`, `identity` |
| `identity.store_placement` | `identity.lookup` | `kind`, `identity`, `address` |

**Gossip** — spans added in `cluster/gossiper.go` (for `SendState`) and `cluster/gossip_actor.go` (for `sendGossipForMember` and `ReceiveState`):

| Span Name | Parent | Attributes |
|-----------|--------|------------|
| `gossip.send_round` | None (root span) | — | Orchestrating send round in `gossiper.go` (SendState) |
| `gossip.send` | `gossip.send_round` | `target_member_id` | Per-member send in `gossip_actor.go` (sendGossipForMember) |
| `gossip.receive` | None (root span) | `source_member_id` | Processing incoming gossip in `gossip_actor.go` (ReceiveState) |

### Tracer Naming Convention

- `otel.Tracer("protoactor/cluster")` for cluster operations
- `otel.Tracer("protoactor/identity")` for identity lookups
- `otel.Tracer("protoactor/gossip")` for gossip
- `otel.Tracer("protoactor/middleware")` remains for existing actor middleware

### Enabling Tracing

Cluster/identity/gossip spans are created whenever an OTEL `TracerProvider` is configured globally via `otel.SetTracerProvider()`. When no provider is set, the default no-op tracer produces zero overhead. No additional configuration flags are needed — this matches how the global `otel.Meter()` already works for metrics.

Actor-level tracing still requires explicitly adding the middleware to actor props (unchanged).

## Testing

### Test Suite 1: Cross-Node Trace Propagation

**Location:** `remote/trace_propagation_test.go`

Set up two in-process nodes with OTEL middleware, backed by an in-memory span exporter (`go.opentelemetry.io/otel/sdk/trace/tracetest`).

**Assertions:**
- Both nodes produce spans
- Spans from Node B share the same `TraceID` as spans from Node A
- Node B receiver span's parent span ID matches the Node A sender span
- Envelope headers contain `traceparent` after sender middleware injection

**Variants:**
- Request/response: full round-trip trace continuity
- Fire-and-forget: one-way propagation
- Multi-hop (A -> B -> C): propagation chains across 3 nodes

### Test Suite 2: Cluster Operation Spans

**Location:** `cluster/trace_cluster_test.go`

Minimal cluster (2-3 nodes, automanaged provider) with in-memory span exporter.

**Assertions:**
- `cluster.request` span created when calling a virtual actor
- `cluster.resolve_pid` appears as child of `cluster.request`
- `identity.lookup` appears as child of `cluster.resolve_pid`
- `gossip.send` / `gossip.receive` spans created during cluster convergence
- Span attributes (`kind`, `identity`, `address`) populated correctly

### Test Suite 3: Zero-Overhead Without Provider

**Location:** `remote/trace_noop_test.go` and `cluster/trace_noop_test.go`

Same operations with NO TracerProvider configured.

**Assertions:**
- No spans exported (exporter receives zero spans)
- No panics or errors
- Headers pass through remote transport unmodified
- No measurable allocation overhead (`testing.AllocsPerRun` on no-op tracer path)

### Metrics Testing

Metrics assertions added to existing test files where operations already occur. Use an in-memory metric reader to verify counters/histograms are recorded with correct attributes. The opt-in per-endpoint metrics get a dedicated test verifying they are NOT recorded when the flag is off and ARE recorded when on.

## Files Modified

### Existing files (metrics additions):
- `metrics/actor_metrics.go` - message sent/received counters, supervision metrics
- `cluster/metrics/cluster_metrics.go` - cluster message counts, topology, gossip, identity metrics
- `remote/metrics/remote_metrics.go` - batch size, message size, inflight, opt-in per-endpoint
- `remote/config.go` - `EnablePerEndpointMetrics` flag

### Existing files (tracing additions):
- `cluster/default_context.go` - `cluster.request`, `cluster.resolve_pid`, `cluster.activation` spans
- `cluster/identitylookup/storage/identity_storage_lookup.go` - `identity.lookup`, `identity.lock_acquire`, `identity.store_placement` spans
- `cluster/gossiper.go` - `gossip.send` spans (SendState)
- `cluster/gossip_actor.go` - `gossip.send` spans (sendGossipForMember), `gossip.receive` spans (ReceiveState)

### Existing files (metrics recording):
- `remote/endpoint_writer.go` - batch size, message size, opt-in per-endpoint sent
- `remote/endpoint_reader.go` - opt-in per-endpoint received
- `actor/strategy_one_for_one.go` - supervision escalation/restart/stop metrics
- `actor/strategy_all_for_one.go` - supervision escalation/restart/stop metrics
- `actor/strategy_exponential_backoff.go` - supervision restart metrics
- `actor/strategy_restarting.go` - supervision restart metrics
- Provider files for NATS KV, NATS Stream, K8s - provider metric recording

### New files:
- `cluster/clusterproviders/natskv/metrics/natskv_metrics.go`
- `cluster/clusterproviders/natsstream/metrics/natsstream_metrics.go`
- `cluster/clusterproviders/k8s/metrics/k8s_metrics.go`
- `remote/trace_propagation_test.go`
- `remote/trace_noop_test.go`
- `cluster/trace_cluster_test.go`
- `cluster/trace_noop_test.go`
