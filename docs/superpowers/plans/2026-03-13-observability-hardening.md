# Observability Hardening Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add comprehensive metrics, distributed tracing for cluster internals, and cross-node trace propagation tests to protoactor-go.

**Architecture:** Extend existing per-layer metric packages with new instruments. Add direct OTEL tracing spans in cluster, identity, and gossip code. Gate opt-in high-cardinality metrics behind a config flag. Test cross-node trace propagation with in-memory exporters.

**Tech Stack:** Go, OpenTelemetry SDK (`go.opentelemetry.io/otel` v1.38.0), Prometheus exporter, `log/slog`, testify

---

## Chunk 1: Actor & Remote Metrics Expansion

### Task 1: Add message sent/received counters and supervision metrics to ActorMetrics

**Files:**
- Modify: `metrics/actor_metrics.go:18-34` (add new fields to struct)
- Modify: `metrics/actor_metrics.go:44-132` (create new instruments in `newInstruments`)

- [ ] **Step 1: Add new fields to ActorMetrics struct**

In `metrics/actor_metrics.go`, add these fields to the `ActorMetrics` struct after the Futures section:

```go
// Messages
ActorMessageSentCount     metric.Int64Counter
ActorMessageReceivedCount metric.Int64Counter

// Supervision
SupervisionEscalationCount metric.Int64Counter
SupervisionRestartCount    metric.Int64Counter
SupervisionStopCount       metric.Int64Counter
```

- [ ] **Step 2: Create the new instruments in newInstruments**

In the `newInstruments` function, after the `FuturesTimedOutCount` block, add:

```go
if instruments.ActorMessageSentCount, err = meter.Int64Counter(
    "protoactor_actor_message_sent_total",
    metric.WithDescription("Messages sent by actors"),
); err != nil {
    err = fmt.Errorf("failed to create ActorMessageSentCount instrument, %w", err)
    logger.Error(err.Error(), slog.Any("error", err))
}

if instruments.ActorMessageReceivedCount, err = meter.Int64Counter(
    "protoactor_actor_message_received_total",
    metric.WithDescription("Messages received by actors"),
); err != nil {
    err = fmt.Errorf("failed to create ActorMessageReceivedCount instrument, %w", err)
    logger.Error(err.Error(), slog.Any("error", err))
}

if instruments.SupervisionEscalationCount, err = meter.Int64Counter(
    "protoactor_supervision_escalation_total",
    metric.WithDescription("Failures escalated to parent"),
); err != nil {
    err = fmt.Errorf("failed to create SupervisionEscalationCount instrument, %w", err)
    logger.Error(err.Error(), slog.Any("error", err))
}

if instruments.SupervisionRestartCount, err = meter.Int64Counter(
    "protoactor_supervision_restart_total",
    metric.WithDescription("Restarts triggered by supervisor strategy"),
); err != nil {
    err = fmt.Errorf("failed to create SupervisionRestartCount instrument, %w", err)
    logger.Error(err.Error(), slog.Any("error", err))
}

if instruments.SupervisionStopCount, err = meter.Int64Counter(
    "protoactor_supervision_stop_total",
    metric.WithDescription("Actors stopped by supervisor strategy"),
); err != nil {
    err = fmt.Errorf("failed to create SupervisionStopCount instrument, %w", err)
    logger.Error(err.Error(), slog.Any("error", err))
}
```

- [ ] **Step 3: Run existing tests to verify no regressions**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./metrics/... -v -count=1`
Expected: All existing tests pass.

- [ ] **Step 4: Commit**

```bash
git add metrics/actor_metrics.go
git commit -m "feat(metrics): add message sent/received and supervision metric instruments"
```

### Task 2: Record supervision metrics in strategy files

**Files:**
- Modify: `actor/strategy_one_for_one.go:25-52`
- Modify: `actor/strategy_all_for_one.go:26-54`
- Modify: `actor/strategy_exponential_backoff.go:26-36`
- Modify: `actor/strategy_restarting.go:11-15`

- [ ] **Step 1: Add supervision metric recording to oneForOneStrategy.HandleFailure**

In `actor/strategy_one_for_one.go`, the `HandleFailure` method needs to record metrics. The strategy has access to `actorSystem *ActorSystem` which has `actorSystem.Metrics`. Add metric recording after each directive action:

```go
func (strategy *oneForOneStrategy) HandleFailure(actorSystem *ActorSystem, supervisor Supervisor, child *PID, rs *RestartStatistics, reason any, message any) {
	directive := strategy.decider(reason)

	switch directive {
	case ResumeDirective:
		logFailure(actorSystem, child, reason, directive)
		supervisor.ResumeChildren(child)
	case RestartDirective:
		if strategy.shouldStop(rs) {
			logFailure(actorSystem, child, reason, StopDirective)
			supervisor.StopChildren(child)
			recordSupervisionMetric(actorSystem, child, "OneForOne", "stop")
		} else {
			logFailure(actorSystem, child, reason, RestartDirective)
			supervisor.RestartChildren(child)
			recordSupervisionMetric(actorSystem, child, "OneForOne", "restart")
		}
	case StopDirective:
		logFailure(actorSystem, child, reason, directive)
		supervisor.StopChildren(child)
		recordSupervisionMetric(actorSystem, child, "OneForOne", "stop")
	case EscalateDirective:
		supervisor.EscalateFailure(reason, message)
		recordSupervisionMetric(actorSystem, child, "OneForOne", "escalate")
	}
}
```

- [ ] **Step 2: Add the shared recordSupervisionMetric helper**

Create a new helper in a new file `actor/supervision_metrics.go`:

```go
package actor

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/awevoke/protoactor-go/metrics"
)

// recordSupervisionMetric records a supervision event metric if metrics are enabled.
func recordSupervisionMetric(actorSystem *ActorSystem, child *PID, strategy, action string) {
	if actorSystem.Metrics == nil {
		return
	}
	m := actorSystem.Metrics.Get(metrics.InternalActorMetrics)
	if m == nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("actortype", fmt.Sprintf("%s", child.Id)),
		attribute.String("strategy", strategy),
	}
	ctx := context.Background()
	switch action {
	case "escalate":
		m.SupervisionEscalationCount.Add(ctx, 1, metric.WithAttributes(attrs...))
	case "restart":
		m.SupervisionRestartCount.Add(ctx, 1, metric.WithAttributes(attrs...))
	case "stop":
		m.SupervisionStopCount.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}
```

- [ ] **Step 3: Add supervision metric recording to allForOneStrategy.HandleFailure**

In `actor/strategy_all_for_one.go`, apply the same pattern:

```go
func (strategy *allForOneStrategy) HandleFailure(actorSystem *ActorSystem, supervisor Supervisor, child *PID, rs *RestartStatistics, reason any, message any) {
	directive := strategy.decider(reason)
	switch directive {
	case ResumeDirective:
		logFailure(actorSystem, child, reason, directive)
		supervisor.ResumeChildren(child)
	case RestartDirective:
		children := supervisor.Children()
		if strategy.shouldStop(rs) {
			logFailure(actorSystem, child, reason, StopDirective)
			supervisor.StopChildren(children...)
			recordSupervisionMetric(actorSystem, child, "AllForOne", "stop")
		} else {
			logFailure(actorSystem, child, reason, RestartDirective)
			supervisor.RestartChildren(children...)
			recordSupervisionMetric(actorSystem, child, "AllForOne", "restart")
		}
	case StopDirective:
		children := supervisor.Children()
		logFailure(actorSystem, child, reason, directive)
		supervisor.StopChildren(children...)
		recordSupervisionMetric(actorSystem, child, "AllForOne", "stop")
	case EscalateDirective:
		supervisor.EscalateFailure(reason, message)
		recordSupervisionMetric(actorSystem, child, "AllForOne", "escalate")
	}
}
```

- [ ] **Step 4: Add supervision metric recording to restartingStrategy.HandleFailure**

In `actor/strategy_restarting.go`:

```go
func (strategy *restartingStrategy) HandleFailure(actorSystem *ActorSystem, supervisor Supervisor, child *PID, _ *RestartStatistics, reason any, _ any) {
	logFailure(actorSystem, child, reason, RestartDirective)
	supervisor.RestartChildren(child)
	recordSupervisionMetric(actorSystem, child, "Restarting", "restart")
}
```

- [ ] **Step 5: Add supervision metric recording to exponentialBackoffStrategy.HandleFailure**

In `actor/strategy_exponential_backoff.go`:

```go
func (strategy *exponentialBackoffStrategy) HandleFailure(actorSystem *ActorSystem, supervisor Supervisor, child *PID, rs *RestartStatistics, reason any, _ any) {
	strategy.setFailureCount(rs)

	backoff := rs.FailureCount() * int(strategy.initialBackoff.Nanoseconds())
	noise := rand.Intn(500)
	dur := time.Duration(backoff + noise)
	time.AfterFunc(dur, func() {
		logFailure(actorSystem, child, reason, RestartDirective)
		supervisor.RestartChildren(child)
		recordSupervisionMetric(actorSystem, child, "ExponentialBackoff", "restart")
	})
}
```

- [ ] **Step 6: Run actor tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./actor/... -v -count=1 -race -timeout 120s`
Expected: All tests pass.

- [ ] **Step 7: Commit**

```bash
git add actor/supervision_metrics.go actor/strategy_one_for_one.go actor/strategy_all_for_one.go actor/strategy_restarting.go actor/strategy_exponential_backoff.go
git commit -m "feat(metrics): record supervision metrics in all strategy implementations"
```

### Task 3: Add remote metrics (batch size, message size, inflight, opt-in per-endpoint)

**Files:**
- Modify: `remote/metrics/remote_metrics.go:15-22` (add new fields)
- Modify: `remote/metrics/remote_metrics.go:26-81` (create new instruments)
- Modify: `remote/config.go:111-137` (add EnablePerEndpointMetrics field)
- Modify: `remote/config-opts.go` (add WithPerEndpointMetrics option)

- [ ] **Step 1: Add EnablePerEndpointMetrics to remote Config**

In `remote/config.go`, add after the `SupervisorMaxRestarts` field:

```go
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
```

- [ ] **Step 2: Add WithPerEndpointMetrics config option**

In `remote/config-opts.go`, add:

```go
// WithPerEndpointMetrics enables per-destination/source address metrics.
// WARNING: Cardinality scales O(n^2) with cluster size. See Config.EnablePerEndpointMetrics.
func WithPerEndpointMetrics() ConfigOption {
	return func(config *Config) {
		config.EnablePerEndpointMetrics = true
	}
}
```

- [ ] **Step 3: Add new fields to RemoteMetrics struct**

In `remote/metrics/remote_metrics.go`, add to the struct:

```go
RemoteMessageBatchSize       metric.Int64Histogram
RemoteMessageSizeBytes       metric.Int64Histogram
RemoteInflightRequests       metric.Int64UpDownCounter
RemoteMessageSentTotal       metric.Int64Counter
RemoteMessageReceivedTotal   metric.Int64Counter
```

- [ ] **Step 4: Create the new instruments in NewRemoteMetrics**

In `remote/metrics/remote_metrics.go`, add after the existing instruments:

```go
if m.RemoteMessageBatchSize, err = meter.Int64Histogram(
    "protoremote_message_batch_size",
    metric.WithDescription("Envelopes per batch on remote writes"),
); err != nil {
    err = fmt.Errorf("failed to create RemoteMessageBatchSize instrument, %w", err)
    logger.Error(err.Error(), slog.Any("error", err))
}

if m.RemoteMessageSizeBytes, err = meter.Int64Histogram(
    "protoremote_message_size_bytes",
    metric.WithDescription("Serialized payload size in bytes"),
    metric.WithUnit("By"),
); err != nil {
    err = fmt.Errorf("failed to create RemoteMessageSizeBytes instrument, %w", err)
    logger.Error(err.Error(), slog.Any("error", err))
}

if m.RemoteInflightRequests, err = meter.Int64UpDownCounter(
    "protoremote_inflight_requests",
    metric.WithDescription("Currently pending remote requests/futures"),
); err != nil {
    err = fmt.Errorf("failed to create RemoteInflightRequests instrument, %w", err)
    logger.Error(err.Error(), slog.Any("error", err))
}

if m.RemoteMessageSentTotal, err = meter.Int64Counter(
    "protoremote_message_sent_total",
    metric.WithDescription("Per-destination message counts (opt-in, O(n^2) cardinality)"),
); err != nil {
    err = fmt.Errorf("failed to create RemoteMessageSentTotal instrument, %w", err)
    logger.Error(err.Error(), slog.Any("error", err))
}

if m.RemoteMessageReceivedTotal, err = meter.Int64Counter(
    "protoremote_message_received_total",
    metric.WithDescription("Per-source message counts (opt-in, O(n^2) cardinality)"),
); err != nil {
    err = fmt.Errorf("failed to create RemoteMessageReceivedTotal instrument, %w", err)
    logger.Error(err.Error(), slog.Any("error", err))
}
```

- [ ] **Step 5: Run remote metrics tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./remote/metrics/... -v -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add remote/config.go remote/config-opts.go remote/metrics/remote_metrics.go
git commit -m "feat(metrics): add remote batch size, message size, inflight, and opt-in per-endpoint metrics"
```

### Task 4: Record new remote metrics in endpoint_writer and endpoint_reader

**Files:**
- Modify: `remote/endpoint_writer.go:189-326` (record batch size, message size, opt-in sent)
- Modify: `remote/endpoint_reader.go:171-262` (record opt-in received)

- [ ] **Step 1: Record batch size and message size in endpoint_writer.sendEnvelopes**

In `remote/endpoint_writer.go`, in the `sendEnvelopes` method:

After the successful `Serialize` call (around line 260, after `bytes, typeName, err := Serialize(...)`), add message size recording:

```go
if state.remote.metricsEnabled {
    _ctx := context.Background()
    attrs := append(actor.SystemLabels(state.remote.actorSystem), attribute.String("messagetype", typeName))
    state.remote.metrics.RemoteSerializedMessageCount.Add(_ctx, 1, metric.WithAttributes(attrs...))
    state.remote.metrics.RemoteMessageSizeBytes.Record(_ctx, int64(len(bytes)), metric.WithAttributes(attrs...))
}
```

(This replaces the existing `RemoteSerializedMessageCount` block so both are recorded together.)

After building the envelopes and before the `stream.Send` call (around line 299, after `if len(envelopes) == 0 { return }`), add batch size recording:

```go
if state.remote.metricsEnabled {
    _ctx := context.Background()
    attrs := append(actor.SystemLabels(state.remote.actorSystem), attribute.String("destinationaddress", state.address))
    state.remote.metrics.RemoteMessageBatchSize.Record(_ctx, int64(len(envelopes)), metric.WithAttributes(attrs...))
}
```

After the write duration recording block (around line 318), add opt-in per-endpoint sent:

```go
if state.remote.metricsEnabled && state.remote.config.EnablePerEndpointMetrics {
    _ctx := context.Background()
    attrs := append(actor.SystemLabels(state.remote.actorSystem), attribute.String("destinationaddress", state.address))
    state.remote.metrics.RemoteMessageSentTotal.Add(_ctx, int64(len(envelopes)), metric.WithAttributes(attrs...))
}
```

- [ ] **Step 2: Record opt-in per-endpoint received in endpoint_reader.onMessageBatch**

In `remote/endpoint_reader.go`, in the `onMessageBatch` method, after the existing `RemoteDeserializedMessageCount` recording block (around line 196), add:

```go
if s.remote.metricsEnabled && s.remote.config.EnablePerEndpointMetrics {
    // Source address is not available per-envelope in the batch protocol,
    // so we count the entire batch as received. The source address comes
    // from the connect request, which is tracked at connection level.
    // For per-source tracking, use the existing endpoint connect/disconnect metrics.
}
```

Note: The endpoint_reader doesn't have easy access to the source address per message. The `sourceaddress` would need to come from the connect request's `ServerConnection.Address`. We need to store it on the `endpointReader`. Add a field `connectedAddress string` to `endpointReader` and set it in `onServerConnection`:

In `remote/endpoint_reader.go`, add field to struct:

```go
type endpointReader struct {
    suspended        atomic.Bool
    remote           *Remote
    connectedAddress string // set on server connection for per-endpoint metrics
}
```

In `onServerConnection`, after the non-blocked branch sends the response, add:

```go
s.connectedAddress = sc.Address
```

Then in `onMessageBatch`, after the existing `RemoteDeserializedMessageCount` block:

```go
if s.remote.metricsEnabled && s.remote.config.EnablePerEndpointMetrics && s.connectedAddress != "" {
    _ctx := context.Background()
    attrs := append(actor.SystemLabels(s.remote.actorSystem), attribute.String("sourceaddress", s.connectedAddress))
    s.remote.metrics.RemoteMessageReceivedTotal.Add(_ctx, 1, metric.WithAttributes(attrs...))
}
```

- [ ] **Step 3: Run remote tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./remote/... -v -count=1 -race -timeout 120s`
Expected: All tests pass.

- [ ] **Step 4: Commit**

```bash
git add remote/endpoint_writer.go remote/endpoint_reader.go
git commit -m "feat(metrics): record batch size, message size, and opt-in per-endpoint metrics in remote transport"
```

---

## Chunk 2: Cluster Metrics Expansion

### Task 5: Add new cluster metrics instruments (topology, gossip, identity, activation)

**Files:**
- Modify: `cluster/metrics/cluster_metrics.go:46-109` (add new fields and instruments)
- Modify: `cluster/metrics/cluster_metrics_test.go` (verify new instruments)

- [ ] **Step 1: Add new fields to ClusterMetrics struct**

In `cluster/metrics/cluster_metrics.go`, add to the `ClusterMetrics` struct:

```go
// Message counts
ClusterMessageSentCount     metric.Int64Counter
ClusterMessageReceivedCount metric.Int64Counter

// Topology
ClusterTopologyUpdateCount metric.Int64Counter
ClusterMemberJoinCount     metric.Int64Counter
ClusterMemberLeaveCount    metric.Int64Counter

// Activations
ClusterActivationCount metric.Int64UpDownCounter

// Gossip
GossipSentCount          metric.Int64Counter
GossipReceivedCount      metric.Int64Counter
GossipRoundtripDuration  metric.Float64Histogram
GossipMessageSizeBytes   metric.Int64Histogram

// Identity
IdentityLookupDuration      metric.Float64Histogram
IdentityLookupFailureCount  metric.Int64Counter
IdentityCacheHitCount       metric.Int64Counter
IdentityCacheMissCount      metric.Int64Counter
```

- [ ] **Step 2: Create the new instruments in NewClusterMetrics**

In `cluster/metrics/cluster_metrics.go`, in `NewClusterMetrics`, after the `ClusterMembersCount` block, add all the new instrument creation blocks following the same error-handling pattern used by existing instruments. Use these metric names:

- `protocluster_message_sent_total` — "Messages sent to virtual actors by cluster kind"
- `protocluster_message_received_total` — "Messages received by virtual actors by cluster kind"
- `protocluster_topology_update_total` — "Topology changes observed by this node"
- `protocluster_member_join_total` — "Members joined"
- `protocluster_member_leave_total` — "Members left"
- `protocluster_activation_count` — "Active virtual actor activations by kind on this node"
- `protocluster_gossip_sent_total` — "Gossip messages sent"
- `protocluster_gossip_received_total` — "Gossip messages received"
- `protocluster_gossip_roundtrip_duration` — "Gossip send round-trip time", unit "s"
- `protocluster_gossip_message_size_bytes` — "Size of gossip state payloads", unit "By"
- `protocluster_identity_lookup_duration` — "Time to look up or activate a virtual actor identity", unit "s"
- `protocluster_identity_lookup_failure_total` — "Failed identity lookups"
- `protocluster_identity_cache_hit_total` — "PID cache hits"
- `protocluster_identity_cache_miss_total` — "PID cache misses"

- [ ] **Step 3: Update the cluster metrics test**

In `cluster/metrics/cluster_metrics_test.go`, extend the test to verify the new fields are non-nil:

```go
func TestNewClusterMetrics(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := NewClusterMetrics(logger)
	if m == nil {
		t.Fatalf("expected metrics instance")
	}
	if m.VirtualActorsCount == nil || m.ClusterMembersCount == nil {
		t.Fatalf("expected gauges to be initialized")
	}
	if m.ClusterMessageSentCount == nil || m.GossipSentCount == nil || m.IdentityLookupDuration == nil {
		t.Fatalf("expected new instruments to be initialized")
	}
}
```

- [ ] **Step 4: Run cluster metrics tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./cluster/metrics/... -v -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cluster/metrics/cluster_metrics.go cluster/metrics/cluster_metrics_test.go
git commit -m "feat(metrics): add cluster topology, gossip, identity, and activation metric instruments"
```

### Task 6: Record cluster metrics in default_context.go (cache hit/miss, message counts)

**Files:**
- Modify: `cluster/default_context.go:230-257` (add cache hit/miss metrics to getPid)
- Modify: `cluster/default_context.go:49-172` (add message sent metrics to Request)

- [ ] **Step 1: Add cache hit/miss metrics to getPid**

In `cluster/default_context.go`, modify the `getPid` method to record cache hit/miss:

```go
func (dcc *DefaultContext) getPid(identity, kind string) (*actor.PID, bool) {
	if pid, ok := dcc.cluster.PidCache.Get(identity, kind); ok {
		if dcc.cluster.metricsEnabled {
			_ctx := context.Background()
			dcc.cluster.metrics.IdentityCacheHitCount.Add(_ctx, 1, metric.WithAttributes(actor.SystemLabels(dcc.cluster.ActorSystem)...))
		}
		return pid, true
	}

	if dcc.cluster.metricsEnabled {
		_ctx := context.Background()
		dcc.cluster.metrics.IdentityCacheMissCount.Add(_ctx, 1, metric.WithAttributes(actor.SystemLabels(dcc.cluster.ActorSystem)...))
	}

	var pid *actor.PID
	if dcc.cluster.metricsEnabled {
		start := time.Now()
		pid = dcc.cluster.Get(identity, kind)
		if pid != nil {
			dcc.cluster.PidCache.Set(identity, kind, pid)
		}
		elapsed := time.Since(start)
		_ctx := context.Background()
		attrs := append(
			actor.SystemLabels(dcc.cluster.ActorSystem),
			attribute.String("clusterkind", kind),
		)
		dcc.cluster.metrics.ClusterResolvePidDuration.Record(_ctx, elapsed.Seconds(), metric.WithAttributes(attrs...))
		dcc.cluster.metrics.IdentityLookupDuration.Record(_ctx, elapsed.Seconds(), metric.WithAttributes(attribute.String("kind", kind)))
		if pid == nil {
			dcc.cluster.metrics.IdentityLookupFailureCount.Add(_ctx, 1, metric.WithAttributes(attribute.String("kind", kind)))
		}
	} else {
		pid = dcc.cluster.Get(identity, kind)
		if pid != nil {
			dcc.cluster.PidCache.Set(identity, kind, pid)
		}
	}

	return pid, false
}
```

- [ ] **Step 2: Add cluster message sent metric to Request**

In `cluster/default_context.go`, in the `Request` method, after the successful `break selectloop` following `resp != nil` (around line 126), record cluster message sent. Best placement is right before the method returns, after the `ClusterRequestDuration` recording block:

```go
if dcc.cluster.metricsEnabled && err == nil {
    _ctx := context.Background()
    dcc.cluster.metrics.ClusterMessageSentCount.Add(_ctx, 1, metric.WithAttributes(attribute.String("kind", kind)))
}
```

- [ ] **Step 3: Run cluster tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./cluster/... -v -count=1 -race -timeout 120s`
Expected: All tests pass.

- [ ] **Step 4: Commit**

```bash
git add cluster/default_context.go
git commit -m "feat(metrics): record cache hit/miss, identity lookup, and cluster message metrics"
```

### Task 7: Record gossip metrics in gossip_actor.go and informer.go

**Files:**
- Modify: `cluster/gossip_actor.go:146-195` (add gossip sent/received metrics)
- Modify: `cluster/informer.go:111-143` (add gossip message size)

- [ ] **Step 1: Add gossip metrics recording to gossip_actor.go**

The gossip actor needs access to the Cluster to check `metricsEnabled` and access `metrics`. Modify `NewGossipActor` to accept `*Cluster`:

In `cluster/gossip_actor.go`, add a `cluster` field to `GossipActor`:

```go
type GossipActor struct {
	gossipRequestTimeout time.Duration
	gossip               Gossip
	cluster              *Cluster
	throttler            actor.ShouldThrottle
}
```

Update `NewGossipActor` to accept and store it:

```go
func NewGossipActor(requestTimeout time.Duration, myID string, getBlockedMembers func() set.Set[string], fanOut int, maxSend int, system *actor.ActorSystem, cluster *Cluster) *GossipActor {
```

Set `cluster: cluster` in the struct initialization.

Update the call site in `cluster/gossiper.go:279-290` (`StartGossiping`) to pass `g.cluster`.

- [ ] **Step 2: Record gossip sent metric in sendGossipForMember**

In `cluster/gossip_actor.go`, in `sendGossipForMember`, after `future := ctx.RequestFuture(...)`, add:

```go
if ga.cluster != nil && ga.cluster.metricsEnabled {
    _ctx := context.Background()
    attrs := actor.SystemLabels(ga.cluster.ActorSystem)
    ga.cluster.metrics.GossipSentCount.Add(_ctx, 1, metric.WithAttributes(attrs...))
}
```

In the `ReenterAfter` callback, after `memberStateDelta.CommitOffsets()`, record roundtrip duration. To do this, capture `start := time.Now()` before the `ctx.RequestFuture` call:

```go
start := time.Now()
future := ctx.RequestFuture(pid, &msg, ga.gossipRequestTimeout)

ctx.ReenterAfter(future, func(res any, err error) {
    if err != nil {
        ctx.Logger().Warn("sendGossipForMember failed", slog.String("MemberId", member.Id), slog.Any("error", err))
        return
    }

    if ga.cluster != nil && ga.cluster.metricsEnabled {
        _ctx := context.Background()
        attrs := actor.SystemLabels(ga.cluster.ActorSystem)
        ga.cluster.metrics.GossipRoundtripDuration.Record(_ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs...))
    }

    resp, ok := res.(*GossipResponse)
    // ... rest unchanged
})
```

- [ ] **Step 3: Record gossip received metric in ReceiveState**

In `cluster/gossip_actor.go`, in the `ReceiveState` method:

```go
func (ga *GossipActor) ReceiveState(remoteState *GossipState, ctx actor.Context) {
	if ga.cluster != nil && ga.cluster.metricsEnabled {
		_ctx := context.Background()
		attrs := actor.SystemLabels(ga.cluster.ActorSystem)
		ga.cluster.metrics.GossipReceivedCount.Add(_ctx, 1, metric.WithAttributes(attrs...))
	}
	updates := ga.gossip.ReceiveState(remoteState)
	for _, update := range updates {
		ctx.ActorSystem().EventStream.Publish(update)
	}
}
```

- [ ] **Step 4: Record gossip message size in informer.go SendState**

In `cluster/informer.go`, the `SendState` method calls the `sendStateToMember` callback. We can measure the size of the state being sent. However, since the Informer doesn't have access to the Cluster metrics, the simplest approach is to measure the gossip payload size in the gossip actor's `sendGossipForMember` where we have access to the cluster.

In `cluster/gossip_actor.go`, in `sendGossipForMember`, after building the `msg`, add size recording:

```go
if ga.cluster != nil && ga.cluster.metricsEnabled {
    if size := proto.Size(memberStateDelta.State); size > 0 {
        _ctx := context.Background()
        attrs := actor.SystemLabels(ga.cluster.ActorSystem)
        ga.cluster.metrics.GossipMessageSizeBytes.Record(_ctx, int64(size), metric.WithAttributes(attrs...))
    }
}
```

Use `proto.Size(memberStateDelta.State)` from `google.golang.org/protobuf/proto` (the codebase does not use vtprotobuf).

- [ ] **Step 5: Run cluster tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./cluster/... -v -count=1 -race -timeout 120s`
Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
git add cluster/gossip_actor.go cluster/gossiper.go
git commit -m "feat(metrics): record gossip sent/received, roundtrip, and message size metrics"
```

### Task 8: Record topology update and member join/leave metrics

**Files:**
- Modify: `cluster/cluster.go` (record topology metrics on topology events)

- [ ] **Step 1: Add topology metrics in cluster.go subscribeToTopologyEvents**

In `cluster/cluster.go:88-99`, the `subscribeToTopologyEvents` method already processes `ClusterTopology` events and sets `ClusterMembersCount`. Extend the existing `if c.metricsEnabled` block to also record topology/join/leave metrics:

```go
func (c *Cluster) subscribeToTopologyEvents() {
	c.ActorSystem.EventStream.Subscribe(func(evt any) {
		if clusterTopology, ok := evt.(*ClusterTopology); ok {
			for _, member := range clusterTopology.Left {
				c.PidCache.RemoveByMember(member)
			}
			if c.metricsEnabled {
				_ctx := context.Background()
				attrs := actor.SystemLabels(c.ActorSystem)
				c.metrics.ClusterMembersCount.Set(int64(len(clusterTopology.Members)))
				c.metrics.ClusterTopologyUpdateCount.Add(_ctx, 1, metric.WithAttributes(attrs...))
				c.metrics.ClusterMemberJoinCount.Add(_ctx, int64(len(clusterTopology.Joined)), metric.WithAttributes(attrs...))
				c.metrics.ClusterMemberLeaveCount.Add(_ctx, int64(len(clusterTopology.Left)), metric.WithAttributes(attrs...))
			}
		}
	})
}
```

Add imports for `"context"` and `"go.opentelemetry.io/otel/metric"` if not already present.

- [ ] **Step 2: Run cluster tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./cluster/... -v -count=1 -race -timeout 120s`
Expected: All tests pass.

- [ ] **Step 3: Commit**

```bash
git add cluster/cluster.go
git commit -m "feat(metrics): record topology update, member join, and member leave metrics"
```

---

## Chunk 3: Provider-Specific Metrics

### Task 9: Create NATS KV provider metrics package

**Files:**
- Create: `cluster/clusterproviders/natskv/metrics/natskv_metrics.go`

- [ ] **Step 1: Create the metrics package**

```go
package natskvmetrics

import (
	"fmt"
	"log/slog"

	"github.com/awevoke/protoactor-go/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// NatsKVMetrics contains OpenTelemetry instruments for tracking NATS KV
// cluster provider operations.
type NatsKVMetrics struct {
	TopologyUpdateDuration  metric.Float64Histogram
	KeyRefreshFailureCount  metric.Int64Counter
	WatchReconnectCount     metric.Int64Counter
	LeaderElectionCount     metric.Int64Counter
}

// NewNatsKVMetrics creates all metric instruments for the NATS KV provider.
func NewNatsKVMetrics(logger *slog.Logger) *NatsKVMetrics {
	meter := otel.Meter(metrics.LibName)
	m := &NatsKVMetrics{}
	var err error

	if m.TopologyUpdateDuration, err = meter.Float64Histogram(
		"protocluster_natskv_topology_update_duration",
		metric.WithDescription("Time to process a topology update"),
		metric.WithUnit("s"),
	); err != nil {
		err = fmt.Errorf("failed to create TopologyUpdateDuration instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.KeyRefreshFailureCount, err = meter.Int64Counter(
		"protocluster_natskv_key_refresh_failure_total",
		metric.WithDescription("Failed member/leader key refreshes"),
	); err != nil {
		err = fmt.Errorf("failed to create KeyRefreshFailureCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.WatchReconnectCount, err = meter.Int64Counter(
		"protocluster_natskv_watch_reconnect_total",
		metric.WithDescription("KV watch reconnections"),
	); err != nil {
		err = fmt.Errorf("failed to create WatchReconnectCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.LeaderElectionCount, err = meter.Int64Counter(
		"protocluster_natskv_leader_election_total",
		metric.WithDescription("Leader election events"),
	); err != nil {
		err = fmt.Errorf("failed to create LeaderElectionCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	return m
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /home/cchamplin/development/protoactor-go && go build ./cluster/clusterproviders/natskv/metrics/...`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add cluster/clusterproviders/natskv/metrics/natskv_metrics.go
git commit -m "feat(metrics): add NATS KV provider metrics package"
```

### Task 10: Wire NATS KV metrics into the provider

**Files:**
- Modify: `cluster/clusterproviders/natskv/natskv_provider.go` (add metrics field, record metrics)

- [ ] **Step 1: Add metrics to the Provider struct and initialize in StartMember/StartClient**

In `cluster/clusterproviders/natskv/natskv_provider.go`:

1. Add fields to the `Provider` struct:
```go
providerMetrics *natskvmetrics.NatsKVMetrics
metricsEnabled  bool
```

2. In the `StartMember` method, after `p.cluster = cluster` is set, initialize:
```go
p.providerMetrics = natskvmetrics.NewNatsKVMetrics(cluster.Logger())
p.metricsEnabled = cluster.MetricsEnabled()
```

3. Record `TopologyUpdateDuration` — wrap the topology update logic (where `cluster.Logger().Debug("Update cluster topology", ...)` is logged) with timing:
```go
if p.metricsEnabled {
    start := time.Now()
    // ... existing topology update logic ...
    p.providerMetrics.TopologyUpdateDuration.Record(context.Background(), time.Since(start).Seconds(), metric.WithAttributes(actor.SystemLabels(p.cluster.ActorSystem)...))
}
```

4. Record `KeyRefreshFailureCount` — at each site where `cluster.Logger().Warn("Failed to refresh member/leader key", ...)` is logged, add:
```go
if p.metricsEnabled {
    p.providerMetrics.KeyRefreshFailureCount.Add(context.Background(), 1, metric.WithAttributes(actor.SystemLabels(p.cluster.ActorSystem)...))
}
```

5. Record `WatchReconnectCount` and `LeaderElectionCount` at their respective event points, guarded by `if p.metricsEnabled`.

- [ ] **Step 2: Run provider tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./cluster/clusterproviders/natskv/... -v -count=1 -race -timeout 120s`
Expected: All tests pass.

- [ ] **Step 3: Commit**

```bash
git add cluster/clusterproviders/natskv/
git commit -m "feat(metrics): wire NATS KV provider metrics into provider operations"
```

### Task 11: Create NATS Stream provider metrics package and wire it

**Files:**
- Create: `cluster/clusterproviders/natsstream/metrics/natsstream_metrics.go`
- Modify: `cluster/clusterproviders/natsstream/natsstream_provider.go`

- [ ] **Step 1: Create the metrics package**

```go
package natsstreammetrics

import (
	"fmt"
	"log/slog"

	"github.com/awevoke/protoactor-go/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// NatsStreamMetrics contains OpenTelemetry instruments for tracking NATS Stream
// cluster provider operations.
type NatsStreamMetrics struct {
	TopologyUpdateDuration  metric.Float64Histogram
	KeyRefreshFailureCount  metric.Int64Counter
	WatchReconnectCount     metric.Int64Counter
	LeaderElectionCount     metric.Int64Counter
}

// NewNatsStreamMetrics creates all metric instruments for the NATS Stream provider.
func NewNatsStreamMetrics(logger *slog.Logger) *NatsStreamMetrics {
	meter := otel.Meter(metrics.LibName)
	m := &NatsStreamMetrics{}
	var err error

	if m.TopologyUpdateDuration, err = meter.Float64Histogram(
		"protocluster_natsstream_topology_update_duration",
		metric.WithDescription("Time to process a topology update"),
		metric.WithUnit("s"),
	); err != nil {
		err = fmt.Errorf("failed to create TopologyUpdateDuration instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.KeyRefreshFailureCount, err = meter.Int64Counter(
		"protocluster_natsstream_key_refresh_failure_total",
		metric.WithDescription("Failed member/leader key refreshes"),
	); err != nil {
		err = fmt.Errorf("failed to create KeyRefreshFailureCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.WatchReconnectCount, err = meter.Int64Counter(
		"protocluster_natsstream_watch_reconnect_total",
		metric.WithDescription("Stream watch reconnections"),
	); err != nil {
		err = fmt.Errorf("failed to create WatchReconnectCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.LeaderElectionCount, err = meter.Int64Counter(
		"protocluster_natsstream_leader_election_total",
		metric.WithDescription("Leader election events"),
	); err != nil {
		err = fmt.Errorf("failed to create LeaderElectionCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	return m
}
```

- [ ] **Step 2: Wire metrics into the provider**

In `cluster/clusterproviders/natsstream/natsstream_provider.go`:

1. Add a `metrics *natsstreammetrics.NatsStreamMetrics` field and `metricsEnabled bool` to the Provider struct
2. In the provider's `StartMember` method, after the cluster is available, initialize: `p.metrics = natsstreammetrics.NewNatsStreamMetrics(p.cluster.Logger())` and set `p.metricsEnabled = p.cluster.MetricsEnabled()`
3. Record `TopologyUpdateDuration` around topology update processing (where `"Update cluster topology"` debug log is emitted)
4. Record `KeyRefreshFailureCount` where key refresh failure warnings are logged
5. Record `WatchReconnectCount` where watch reconnections occur
6. Record `LeaderElectionCount` where leader role changes are detected
7. Guard all recordings with `if p.metricsEnabled`

- [ ] **Step 3: Run provider tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./cluster/clusterproviders/natsstream/... -v -count=1 -race -timeout 120s`
Expected: All tests pass.

- [ ] **Step 4: Commit**

```bash
git add cluster/clusterproviders/natsstream/
git commit -m "feat(metrics): add NATS Stream provider metrics package and wire into provider"
```

### Task 12: Create K8s provider metrics package and wire it

**Files:**
- Create: `cluster/clusterproviders/k8s/metrics/k8s_metrics.go`
- Modify: `cluster/clusterproviders/k8s/k8s_provider.go`

- [ ] **Step 1: Create the metrics package**

```go
package k8smetrics

import (
	"fmt"
	"log/slog"

	"github.com/awevoke/protoactor-go/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

type K8sMetrics struct {
	PodWatcherRestartCount   metric.Int64Counter
	TopologyUpdateDuration   metric.Float64Histogram
	PodReadinessDuration     metric.Float64Histogram
}

func NewK8sMetrics(logger *slog.Logger) *K8sMetrics {
	meter := otel.Meter(metrics.LibName)
	m := &K8sMetrics{}
	var err error

	if m.PodWatcherRestartCount, err = meter.Int64Counter(
		"protocluster_k8s_pod_watcher_restart_total",
		metric.WithDescription("Pod watcher restarts"),
	); err != nil {
		err = fmt.Errorf("failed to create PodWatcherRestartCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.TopologyUpdateDuration, err = meter.Float64Histogram(
		"protocluster_k8s_topology_update_duration",
		metric.WithDescription("Time to process a pod list update"),
		metric.WithUnit("s"),
	); err != nil {
		err = fmt.Errorf("failed to create TopologyUpdateDuration instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.PodReadinessDuration, err = meter.Float64Histogram(
		"protocluster_k8s_pod_readiness_duration",
		metric.WithDescription("Time from pod seen to pod ready"),
		metric.WithUnit("s"),
	); err != nil {
		err = fmt.Errorf("failed to create PodReadinessDuration instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	return m
}
```

- [ ] **Step 2: Wire metrics into the K8s provider**

Add a `metrics` field and `metricsEnabled` bool to the K8s `Provider` struct. Record metrics at:
- Pod watcher restarts
- Topology update processing duration
- Pod readiness transitions

- [ ] **Step 3: Run K8s provider tests (if any exist)**

Run: `cd /home/cchamplin/development/protoactor-go && go build ./cluster/clusterproviders/k8s/...`
Expected: Compiles successfully.

- [ ] **Step 4: Commit**

```bash
git add cluster/clusterproviders/k8s/
git commit -m "feat(metrics): add K8s provider metrics package and wire into provider"
```

---

## Chunk 4: Distributed Tracing for Cluster Internals

### Task 13: Add tracing spans to cluster request path

**Files:**
- Modify: `cluster/default_context.go:49-172` (add cluster.request, cluster.resolve_pid spans)

- [ ] **Step 1: Add OTEL imports to default_context.go**

Add these imports:

```go
"go.opentelemetry.io/otel"
"go.opentelemetry.io/otel/trace"
```

(The `attribute` and `metric` imports already exist.)

- [ ] **Step 2: Add cluster.request span to the Request method**

At the beginning of the `Request` method, after `start := time.Now()`, add:

```go
traceCtx, span := otel.Tracer("protoactor/cluster").Start(context.Background(), "cluster.request",
    trace.WithAttributes(
        attribute.String("kind", kind),
        attribute.String("identity", identity),
        attribute.String("messagetype", reflect.TypeOf(message).String()),
    ),
)
defer span.End()
```

Store `traceCtx` — this is used as the parent for child spans in `getPid`.

- [ ] **Step 3: Add cluster.resolve_pid span to getPid**

Modify `getPid` to accept a `context.Context` parameter for trace parent propagation:

```go
func (dcc *DefaultContext) getPid(traceCtx context.Context, identity, kind string) (*actor.PID, bool) {
```

At the start of the non-cache path (when the PID is not found in cache), add:

```go
resolveCtx, resolveSpan := otel.Tracer("protoactor/cluster").Start(traceCtx, "cluster.resolve_pid",
    trace.WithAttributes(
        attribute.String("kind", kind),
        attribute.String("identity", identity),
    ),
)
defer resolveSpan.End()
```

Store `resolveCtx` in a package-level or request-scoped way so the identity lookup can use it as a parent. Since the `IdentityLookup.Get()` interface only takes `*ClusterIdentity`, use a context key on the cluster to temporarily hold the trace context during the lookup call:

In `cluster/`, add a small helper file `cluster/trace_context.go`:

```go
package cluster

import "context"

// traceContextKey is used to pass trace context through identity lookups.
type traceContextKey struct{}

// WithTraceContext attaches a trace context for use by identity lookups.
func WithTraceContext(ctx context.Context) context.Context {
    return context.WithValue(context.Background(), traceContextKey{}, ctx)
}

// TraceContextFromContext retrieves the trace context if set.
func TraceContextFromContext() context.Context {
    return nil // placeholder — actual impl uses a cluster-scoped field
}
```

**Alternative approach (simpler):** Since `getPid` calls `dcc.cluster.Get(identity, kind)` which calls the identity lookup, and we cannot change the `IdentityLookup.Get()` interface without modifying all implementations, store the trace context on the `Cluster` struct as a request-scoped field. However, this introduces concurrency issues.

**Recommended approach:** Accept that identity lookup spans will be root spans (not parented to `cluster.resolve_pid`) in this iteration. The `cluster.request` → `cluster.resolve_pid` parent-child relationship works because both are in the same method. The identity lookup spans start their own trace context. This is pragmatic and avoids an interface change. Document this limitation and note that a future `IdentityLookup` interface change could add context threading.

Update call sites — there are exactly 2 call sites for `getPid`:
1. `Request` method at line 98: change to `dcc.getPid(traceCtx, identity, kind)`
2. `RequestFuture` method at line 206: change to `dcc.getPid(traceCtx, identity, kind)`

For `RequestFuture`, also add a `traceCtx` span:

```go
traceCtx, span := otel.Tracer("protoactor/cluster").Start(context.Background(), "cluster.request",
    trace.WithAttributes(
        attribute.String("kind", kind),
        attribute.String("identity", identity),
        attribute.String("messagetype", reflect.TypeOf(message).String()),
    ),
)
defer span.End()
```

- [ ] **Step 4: Run cluster tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./cluster/... -v -count=1 -race -timeout 120s`
Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add cluster/default_context.go
git commit -m "feat(tracing): add cluster.request and cluster.resolve_pid spans"
```

### Task 14: Add tracing spans to identity storage lookup

**Files:**
- Modify: `cluster/identitylookup/storage/identity_storage_lookup.go:44-80` (add identity spans)

- [ ] **Step 1: Add OTEL imports**

```go
"context"
"go.opentelemetry.io/otel"
"go.opentelemetry.io/otel/attribute"
"go.opentelemetry.io/otel/trace"
```

- [ ] **Step 2: Add identity.lookup span to the Get method**

Note: The `IdentityLookup.Get()` interface takes only `*ClusterIdentity` with no `context.Context` parameter. Changing this interface would require modifying all identity lookup implementations (disthash, storage, NATS, Redis, Postgres). For this iteration, identity spans start as root spans. A future interface change can add context threading to parent them under `cluster.resolve_pid`. Document this in a code comment.

At the start of the `Get` method:

```go
func (l *IdentityStorageLookup) Get(ci *cluster.ClusterIdentity) *actor.PID {
	// Note: identity spans are root spans because the IdentityLookup.Get()
	// interface does not accept context.Context. A future interface change
	// could parent these under cluster.resolve_pid.
	ctx, span := otel.Tracer("protoactor/identity").Start(context.Background(), "identity.lookup",
		trace.WithAttributes(
			attribute.String("kind", ci.Kind),
			attribute.String("identity", ci.Identity),
			attribute.String("provider", "storage"),
		),
	)
	defer span.End()
```

- [ ] **Step 3: Add identity.lock_acquire span around TryAcquireLock**

Wrap the `TryAcquireLock` call using `ctx` from the parent span:

```go
_, lockSpan := otel.Tracer("protoactor/identity").Start(ctx, "identity.lock_acquire",
    trace.WithAttributes(
        attribute.String("kind", ci.Kind),
        attribute.String("identity", ci.Identity),
    ),
)
lock := l.storage.TryAcquireLock(ci)
lockSpan.End()
```

- [ ] **Step 4: Add identity.store_placement span in spawnActivation**

Modify `spawnActivation` to accept a `context.Context` parameter (this is a private method, so only one call site to update):

```go
func (l *IdentityStorageLookup) spawnActivation(ctx context.Context, ci *cluster.ClusterIdentity, lock *cluster.SpawnLock) *actor.PID {
```

Then wrap `StoreActivation`:

```go
_, storeSpan := otel.Tracer("protoactor/identity").Start(ctx, "identity.store_placement",
    trace.WithAttributes(
        attribute.String("kind", ci.Kind),
        attribute.String("identity", ci.Identity),
        attribute.String("address", pid.Address),
    ),
)
l.storage.StoreActivation(l.memberID, lock, pid)
storeSpan.End()
```

Update the call site in `Get` to pass `ctx`: `pid := l.spawnActivation(ctx, ci, lock)`

- [ ] **Step 5: Run identity lookup tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./cluster/identitylookup/... -v -count=1 -race -timeout 120s`
Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
git add cluster/identitylookup/storage/identity_storage_lookup.go
git commit -m "feat(tracing): add identity.lookup, identity.lock_acquire, and identity.store_placement spans"
```

### Task 15: Add tracing spans to gossip

**Files:**
- Modify: `cluster/gossip_actor.go:146-195` (add gossip.send, gossip.receive spans)
- Modify: `cluster/gossiper.go:249-263` (add gossip.send_round span)

- [ ] **Step 1: Add OTEL imports to gossip_actor.go**

```go
"context"
"go.opentelemetry.io/otel"
"go.opentelemetry.io/otel/attribute"
```

- [ ] **Step 2: Add gossip.send_round span in gossiper.go SendState**

In `cluster/gossiper.go`, in the `SendState` method:

```go
func (g *Gossiper) SendState() {
	if g.pid == nil {
		return
	}

	_, span := otel.Tracer("protoactor/gossip").Start(context.Background(), "gossip.send_round")
	defer span.End()

	r, err := g.cluster.ActorSystem.Root.RequestFuture(g.pid, &SendGossipStateRequest{}, 5*time.Second).Result()
	// ... rest unchanged
}
```

Add `"go.opentelemetry.io/otel"` and `"context"` imports to gossiper.go.

Note: The `gossip.send_round` span and `gossip.send` spans cannot have a direct parent-child relationship because `SendState` in gossiper.go communicates with the gossip actor via `RequestFuture`, crossing an actor boundary. The `sendGossipForMember` call happens inside the gossip actor's message handler, not in the same goroutine. The gossip.send spans will be root spans — this is correct for the actor model where each actor is its own execution boundary.

- [ ] **Step 3: Add gossip.send span in sendGossipForMember**

In `cluster/gossip_actor.go`, in `sendGossipForMember`:

```go
func (ga *GossipActor) sendGossipForMember(member *Member, memberStateDelta *MemberStateDelta, ctx actor.Context) {
	_, span := otel.Tracer("protoactor/gossip").Start(context.Background(), "gossip.send",
		trace.WithAttributes(attribute.String("target_member_id", member.Id)),
	)

	pid := actor.NewPID(member.Address(), DefaultGossipActorName)
	// ... build msg ...
	future := ctx.RequestFuture(pid, &msg, ga.gossipRequestTimeout)

	ctx.ReenterAfter(future, func(res any, err error) {
		defer span.End()
		// ... rest unchanged
	})
}
```

Add `"go.opentelemetry.io/otel/trace"` import.

- [ ] **Step 4: Add gossip.receive span in ReceiveState**

In `cluster/gossip_actor.go`, add `source_member_id` attribute. The source member ID is available from the `GossipRequest.FromMemberId` field. Modify `ReceiveState` to accept the source member ID, or add the span in `onGossipRequest` instead where the request is available:

In `onGossipRequest` (line 95), add the span there instead of in `ReceiveState`:

```go
func (ga *GossipActor) onGossipRequest(r *GossipRequest, ctx actor.Context) {
	_, span := otel.Tracer("protoactor/gossip").Start(context.Background(), "gossip.receive",
		trace.WithAttributes(attribute.String("source_member_id", r.FromMemberId)),
	)
	defer span.End()

	if ga.throttler() == actor.Open {
		ctx.Logger().Debug("OnGossipRequest", slog.Any("sender", ctx.Sender()))
	}
	ga.ReceiveState(r.State, ctx)
	// ... rest unchanged
}
```

- [ ] **Step 5: Run cluster tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./cluster/... -v -count=1 -race -timeout 120s`
Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
git add cluster/gossip_actor.go cluster/gossiper.go
git commit -m "feat(tracing): add gossip.send_round, gossip.send, and gossip.receive spans"
```

---

## Chunk 5: Cross-Node Trace Propagation Tests

**Implementation notes for all tests in this chunk:**
1. `systemA.Root.Send()` does NOT go through sender middleware — sends must happen from within an actor's context for `SenderMiddleware` to fire. All test actors must send via `ctx.Send()`, not `Root.Send()`.
2. `remote.Ping` does not exist. Use a proto message type that is registered with the remote serializer. Check `remote/messages.go` or `remote/protos.pb.go` for available types (e.g., `ActorPidResponse`, `ConnectRequest`). Alternatively, create a test-only proto message.
3. The `setupOtelForTest` and `setupRemoteNode` helpers below should be shared across all test files in this chunk.
4. When setting global `TracerProvider`, always save/restore previous values to avoid test pollution.

### Task 16: Write cross-node trace propagation test (fire-and-forget)

**Files:**
- Create: `remote/trace_propagation_test.go`

- [ ] **Step 1: Write the test file**

This test sets up two in-process remote nodes with OTEL middleware, sends a message from Node A to Node B, and verifies trace propagation.

```go
package remote_test

import (
	"context"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/actor/middleware/opentelemetry"
	"github.com/awevoke/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

func TestTraceContextPropagatesAcrossNodes_FireAndForget(t *testing.T) {
	// Set up in-memory span exporter
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer tp.Shutdown(context.Background())

	// Set global tracer and propagator
	prevTP := otel.GetTracerProvider()
	prevProp := otel.GetTextMapPropagator()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	defer func() {
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
	}()

	// Create two actor systems with OTEL middleware
	systemA := actor.NewActorSystem()
	systemB := actor.NewActorSystem()

	configA := remote.Configure("127.0.0.1", 0)
	configB := remote.Configure("127.0.0.1", 0)

	remoteA := remote.NewRemote(systemA, configA)
	remoteB := remote.NewRemote(systemB, configB)

	remoteA.Start()
	remoteB.Start()
	defer remoteA.Shutdown(true)
	defer remoteB.Shutdown(true)

	// Spawn a receiver on Node B with OTEL middleware
	received := make(chan bool, 1)
	propsB := actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *ActorPidResponse: // Use a registered proto type, not remote.Ping
			received <- true
		}
	}).WithReceiverMiddleware(opentelemetry.ReceiverMiddleware()).
		WithSenderMiddleware(opentelemetry.SenderMiddleware())

	pidB, err := systemB.Root.SpawnNamed(propsB, "receiver")
	require.NoError(t, err)

	// Spawn a sender on Node A with OTEL middleware.
	// IMPORTANT: Must send from within actor context so SenderMiddleware fires.
	remotePidB := actor.NewPID(remoteB.GetAddress(), "receiver")
	propsA := actor.PropsFromFunc(func(ctx actor.Context) {
		if _, ok := ctx.Message().(*actor.Started); ok {
			ctx.Send(remotePidB, &ActorPidResponse{Pid: ctx.Self()})
		}
	}).WithReceiverMiddleware(opentelemetry.ReceiverMiddleware()).
		WithSenderMiddleware(opentelemetry.SenderMiddleware())

	_, err = systemA.Root.SpawnNamed(propsA, "sender")
	require.NoError(t, err)

	// Wait for message to be received
	select {
	case <-received:
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for message")
	}

	// Flush spans
	tp.ForceFlush(context.Background())

	// Verify spans
	spans := exporter.GetSpans()
	assert.NotEmpty(t, spans, "Expected spans to be recorded")

	// Find spans from both nodes
	// Node A should have a sender span, Node B should have a receiver span
	// They should share the same trace ID
	var traceIDs []string
	for _, s := range spans {
		traceIDs = append(traceIDs, s.SpanContext.TraceID().String())
	}
	// All trace IDs should match (same distributed trace)
	if len(traceIDs) > 1 {
		for _, id := range traceIDs[1:] {
			assert.Equal(t, traceIDs[0], id, "All spans should share the same trace ID")
		}
	}
}
```

Note: This is a starting point. The exact message types and remote setup may need adjustment based on how the remote package registers message types. Use `remote.Ping` if it exists, or register a test proto message.

- [ ] **Step 2: Run the test to verify it works**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./remote/ -run TestTraceContextPropagatesAcrossNodes -v -count=1 -race -timeout 30s`
Expected: PASS with spans sharing the same trace ID.

- [ ] **Step 3: Commit**

```bash
git add remote/trace_propagation_test.go
git commit -m "test(tracing): add cross-node trace propagation test for fire-and-forget"
```

### Task 17: Add request/response and multi-hop trace propagation tests

**Files:**
- Modify: `remote/trace_propagation_test.go`

- [ ] **Step 1: Add request/response trace propagation test**

Add `TestTraceContextPropagatesAcrossNodes_RequestResponse` — Node A sends `Request` to Node B, B responds. Verify the response span on A is parented correctly.

- [ ] **Step 2: Add multi-hop trace propagation test**

Add `TestTraceContextPropagatesAcrossNodes_MultiHop` — Set up 3 nodes. A sends to B, B forwards to C. Verify all 3 nodes' spans share the same trace ID and parent chain is correct: A → B → C.

- [ ] **Step 3: Run all trace tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./remote/ -run TestTraceContext -v -count=1 -race -timeout 60s`
Expected: All PASS.

- [ ] **Step 4: Commit**

```bash
git add remote/trace_propagation_test.go
git commit -m "test(tracing): add request/response and multi-hop trace propagation tests"
```

### Task 18: Write zero-overhead no-op tracer tests

**Files:**
- Create: `remote/trace_noop_test.go`
- Create: `cluster/trace_noop_test.go`

- [ ] **Step 1: Write remote no-op tracer test**

In `remote/trace_noop_test.go`:

```go
package remote_test

import (
	"context"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/remote"
	"github.com/stretchr/testify/assert"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel"
)

func TestNoOpTracer_NoSpansExported(t *testing.T) {
	// Set up an exporter attached to a TracerProvider that IS set globally,
	// but actors are spawned WITHOUT OTEL middleware. This verifies that
	// without middleware, no spans are produced even with a provider configured.
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer tp.Shutdown(context.Background())

	prevTP := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(prevTP)

	// Create two nodes and send a message — NO middleware on actors
	systemA := actor.NewActorSystem()
	systemB := actor.NewActorSystem()

	configA := remote.Configure("127.0.0.1", 0)
	configB := remote.Configure("127.0.0.1", 0)

	remoteA := remote.NewRemote(systemA, configA)
	remoteB := remote.NewRemote(systemB, configB)

	remoteA.Start()
	remoteB.Start()
	defer remoteA.Shutdown(true)
	defer remoteB.Shutdown(true)

	received := make(chan bool, 1)
	propsB := actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *ActorPidResponse: // Use a registered proto type, not remote.Ping
			received <- true
		}
	})
	_, err := systemB.Root.SpawnNamed(propsB, "receiver")
	assert.NoError(t, err)

	remotePidB := actor.NewPID(remoteB.GetAddress(), "noop-receiver")
	systemA.Root.Send(remotePidB, &ActorPidResponse{})

	select {
	case <-received:
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for message")
	}

	// No middleware → no spans should be exported even with a TracerProvider
	tp.ForceFlush(context.Background())
	assert.Empty(t, exporter.GetSpans(), "No spans should be exported without middleware")
}

func TestNoOpTracer_ZeroAllocations(t *testing.T) {
	// Verify that the no-op tracer path doesn't allocate
	// This is a basic check — exact alloc counts depend on Go version
	allocs := testing.AllocsPerRun(10, func() {
		_, span := otel.Tracer("protoactor/cluster").Start(context.Background(), "test.span")
		span.End()
	})
	assert.LessOrEqual(t, allocs, float64(2), "No-op tracer should have minimal allocations")
}
```

- [ ] **Step 2: Write cluster no-op tracer test**

In `cluster/trace_noop_test.go`:

```go
package cluster_test

import (
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/cluster/clusterproviders/automanaged"
	"github.com/awevoke/protoactor-go/cluster/identitylookup/disthash"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestNoOpTracer_ClusterOperations_NoPanic(t *testing.T) {
	// Use empty TracerProvider (no exporter attached)
	prevTP := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider())
	defer otel.SetTracerProvider(prevTP)

	// Start a minimal single-node cluster
	system := actor.NewActorSystem()
	provider := automanaged.NewWithConfig(1*time.Second, 0, "localhost:0")
	lookup := disthash.New()
	clusterConfig := cluster.Configure("noop-test-cluster", provider, lookup)
	c := cluster.New(system, clusterConfig)

	err := c.StartMember()
	require.NoError(t, err)
	defer c.Shutdown(true)

	// Wait for cluster to initialize
	time.Sleep(2 * time.Second)

	// Make a cluster request — exercises the traced code paths with no-op tracer
	// This will likely fail (no kind registered) but should NOT panic
	_, _ = c.Request("test-identity", "nonexistent-kind", &struct{}{})

	// If we reach here without panicking, the test passes
}
```

Note: The exact cluster setup may need adjustment based on the available `automanaged` constructor. Check `automanaged.New()` or `automanaged.NewWithConfig()` signatures. The key assertion is that no panics occur when tracing code runs with a no-op/empty tracer.

- [ ] **Step 3: Run tests**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./remote/ -run TestNoOp -v -count=1 -race -timeout 30s`
Run: `cd /home/cchamplin/development/protoactor-go && go test ./cluster/ -run TestNoOp -v -count=1 -race -timeout 30s`
Expected: All PASS.

- [ ] **Step 4: Commit**

```bash
git add remote/trace_noop_test.go cluster/trace_noop_test.go
git commit -m "test(tracing): add zero-overhead no-op tracer tests for remote and cluster"
```

### Task 19: Write cluster operation span tests

**Files:**
- Create: `cluster/trace_cluster_test.go`

- [ ] **Step 1: Write cluster span integration test**

Set up a minimal 2-3 node automanaged cluster with in-memory span exporter. Send a request to a virtual actor and verify:

- `cluster.request` span exists
- `cluster.resolve_pid` is a child of `cluster.request`
- Span attributes (`kind`, `identity`) are set correctly
- `gossip.send_round`, `gossip.send`, `gossip.receive` spans are created during cluster convergence

```go
package cluster_test

import (
	"context"
	"testing"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/cluster/clusterproviders/automanaged"
	"github.com/awevoke/protoactor-go/cluster/identitylookup/disthash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

func TestClusterOperationSpans(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer tp.Shutdown(context.Background())

	prevTP := otel.GetTracerProvider()
	prevProp := otel.GetTextMapPropagator()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	defer func() {
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
	}()

	// Create a test kind that echoes back
	testKindProps := actor.PropsFromFunc(func(ctx actor.Context) {
		switch ctx.Message().(type) {
		case *actor.Started, *actor.Stopping, *actor.Stopped:
			// lifecycle
		default:
			ctx.Respond(ctx.Message())
		}
	})

	// Set up single-node cluster with automanaged provider
	system := actor.NewActorSystem()
	provider := automanaged.NewWithConfig(1*time.Second, 0, "localhost:0")
	lookup := disthash.New()
	kind := cluster.NewKind("TestGrain", testKindProps)
	clusterConfig := cluster.Configure("trace-test-cluster", provider, lookup,
		cluster.WithKinds(kind))
	c := cluster.New(system, clusterConfig)

	err := c.StartMember()
	require.NoError(t, err)
	defer c.Shutdown(true)

	// Wait for cluster to converge
	time.Sleep(3 * time.Second)

	// Make a cluster request — triggers cluster.request and cluster.resolve_pid spans
	_, _ = c.Request("test-actor-1", "TestGrain", &actor.Touch{})

	// Allow gossip to run at least one cycle
	time.Sleep(2 * time.Second)

	tp.ForceFlush(context.Background())
	spans := exporter.GetSpans()

	// Verify cluster.request span exists
	var foundClusterRequest, foundResolvePid bool
	var clusterRequestSpanID, resolvePidParentSpanID string
	for _, s := range spans {
		switch s.Name {
		case "cluster.request":
			foundClusterRequest = true
			clusterRequestSpanID = s.SpanContext.SpanID().String()
			// Verify attributes
			for _, attr := range s.Attributes {
				if attr.Key == "kind" {
					assert.Equal(t, "TestGrain", attr.Value.AsString())
				}
				if attr.Key == "identity" {
					assert.Equal(t, "test-actor-1", attr.Value.AsString())
				}
			}
		case "cluster.resolve_pid":
			foundResolvePid = true
			if s.Parent.IsValid() {
				resolvePidParentSpanID = s.Parent.SpanID().String()
			}
		}
	}

	assert.True(t, foundClusterRequest, "Expected cluster.request span")
	assert.True(t, foundResolvePid, "Expected cluster.resolve_pid span")

	// Verify parent-child: resolve_pid should be child of cluster.request
	if foundClusterRequest && foundResolvePid {
		assert.Equal(t, clusterRequestSpanID, resolvePidParentSpanID,
			"cluster.resolve_pid should be a child of cluster.request")
	}

	// Log gossip spans found (timing-dependent, so just informational)
	var gossipSpanNames []string
	for _, s := range spans {
		if s.Name == "gossip.send" || s.Name == "gossip.send_round" || s.Name == "gossip.receive" {
			gossipSpanNames = append(gossipSpanNames, s.Name)
		}
	}
	t.Logf("Gossip spans found: %v", gossipSpanNames)
}
```

Note: The exact test setup depends on how the cluster test helpers work. Check `cluster/cluster_test_tool.go` or `newClusterForTest()` for patterns.

- [ ] **Step 2: Run test**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./cluster/ -run TestClusterOperationSpans -v -count=1 -race -timeout 60s`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add cluster/trace_cluster_test.go
git commit -m "test(tracing): add cluster operation span integration test"
```

---

## Chunk 6: Go Module Dependencies and Final Verification

### Task 20: Update go.mod with any new OTEL SDK dependencies

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Tidy go modules**

Run: `cd /home/cchamplin/development/protoactor-go && go mod tidy`
Expected: go.mod and go.sum updated with any new transitive dependencies (particularly `go.opentelemetry.io/otel/sdk/trace` for the test exporter).

- [ ] **Step 2: Verify full build**

Run: `cd /home/cchamplin/development/protoactor-go && go build ./...`
Expected: No compilation errors.

- [ ] **Step 3: Run full test suite**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./... -race -timeout 300s`
Expected: All tests pass.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: tidy go modules for new OTEL SDK test dependencies"
```

### Task 21: Verify opt-in per-endpoint metrics test

**Files:**
- Create or modify: `remote/metrics_opt_in_test.go`

- [ ] **Step 1: Write test verifying opt-in metrics are off by default**

Create a test that sets up a remote with `EnablePerEndpointMetrics: false` (default), sends messages, and verifies `RemoteMessageSentTotal` / `RemoteMessageReceivedTotal` are NOT recorded. Then a second test with `EnablePerEndpointMetrics: true` that verifies they ARE recorded.

Use an in-memory metric reader from `go.opentelemetry.io/otel/sdk/metric/metricdata` to read metric values.

- [ ] **Step 2: Run the test**

Run: `cd /home/cchamplin/development/protoactor-go && go test ./remote/ -run TestOptInMetrics -v -count=1 -race -timeout 30s`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add remote/metrics_opt_in_test.go
git commit -m "test(metrics): verify opt-in per-endpoint metrics are disabled by default and enabled with flag"
```
