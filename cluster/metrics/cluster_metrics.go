// Package clustermetrics reports cluster metrics via OpenTelemetry.
package clustermetrics

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/awevoke/protoactor-go/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

type observableGauge struct {
	mu    sync.RWMutex
	value int64
}

func newObservableGauge(meter metric.Meter, name, description string, logger *slog.Logger) *observableGauge {
	g := &observableGauge{}
	_, err := meter.Int64ObservableGauge(name,
		metric.WithDescription(description),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			g.mu.RLock()
			v := g.value
			g.mu.RUnlock()
			o.Observe(v)
			return nil
		}),
	)
	if err != nil {
		err = fmt.Errorf("failed to create %s instrument, %w", name, err)
		logger.Error(err.Error(), slog.Any("error", err))
	}
	return g
}

func (g *observableGauge) Set(v int64) {
	g.mu.Lock()
	g.value = v
	g.mu.Unlock()
}

// ClusterMetrics exposes OpenTelemetry instruments used to record cluster statistics.
type ClusterMetrics struct {
	ClusterActorSpawnDuration metric.Float64Histogram
	ClusterRequestDuration    metric.Float64Histogram
	ClusterRequestRetryCount  metric.Int64Counter
	ClusterResolvePidDuration metric.Float64Histogram
	VirtualActorsCount        *observableGauge
	ClusterMembersCount       *observableGauge

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
	GossipSentCount         metric.Int64Counter
	GossipReceivedCount     metric.Int64Counter
	GossipRoundtripDuration metric.Float64Histogram
	GossipMessageSizeBytes  metric.Int64Histogram

	// Identity
	IdentityLookupDuration     metric.Float64Histogram
	IdentityLookupFailureCount metric.Int64Counter
	IdentityCacheHitCount      metric.Int64Counter
	IdentityCacheMissCount     metric.Int64Counter
}

// NewClusterMetrics creates a new metrics container wired to the given logger.
func NewClusterMetrics(logger *slog.Logger) *ClusterMetrics {
	meter := otel.Meter(metrics.LibName)
	m := &ClusterMetrics{}
	var err error

	if m.ClusterActorSpawnDuration, err = meter.Float64Histogram(
		"protocluster_virtualactor_spawn_duration",
		metric.WithDescription("Time it takes to spawn a virtual actor"),
		metric.WithUnit("s"),
	); err != nil {
		err = fmt.Errorf("failed to create ClusterActorSpawnDuration instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.ClusterRequestDuration, err = meter.Float64Histogram(
		"protocluster_virtualactor_requestasync_duration",
		metric.WithDescription("Cluster request duration"),
		metric.WithUnit("s"),
	); err != nil {
		err = fmt.Errorf("failed to create ClusterRequestDuration instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.ClusterRequestRetryCount, err = meter.Int64Counter(
		"protocluster_virtualactor_requestasync_retry_count",
		metric.WithDescription("Number of retries after failed cluster requests"),
	); err != nil {
		err = fmt.Errorf("failed to create ClusterRequestRetryCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.ClusterResolvePidDuration, err = meter.Float64Histogram(
		"protocluster_resolve_pid_duration",
		metric.WithDescription("Time it takes to resolve a pid"),
		metric.WithUnit("s"),
	); err != nil {
		err = fmt.Errorf("failed to create ClusterResolvePidDuration instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	m.VirtualActorsCount = newObservableGauge(meter,
		"protocluster_virtualactors",
		"Number of active virtual actors on this node",
		logger,
	)

	m.ClusterMembersCount = newObservableGauge(meter,
		"protocluster_members_count",
		"Number of cluster members as seen by this node",
		logger,
	)

	if m.ClusterMessageSentCount, err = meter.Int64Counter(
		"protocluster_message_sent_total",
		metric.WithDescription("Messages sent to virtual actors by cluster kind"),
	); err != nil {
		err = fmt.Errorf("failed to create ClusterMessageSentCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.ClusterMessageReceivedCount, err = meter.Int64Counter(
		"protocluster_message_received_total",
		metric.WithDescription("Messages received by virtual actors by cluster kind"),
	); err != nil {
		err = fmt.Errorf("failed to create ClusterMessageReceivedCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.ClusterTopologyUpdateCount, err = meter.Int64Counter(
		"protocluster_topology_update_total",
		metric.WithDescription("Topology changes observed by this node"),
	); err != nil {
		err = fmt.Errorf("failed to create ClusterTopologyUpdateCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.ClusterMemberJoinCount, err = meter.Int64Counter(
		"protocluster_member_join_total",
		metric.WithDescription("Members joined"),
	); err != nil {
		err = fmt.Errorf("failed to create ClusterMemberJoinCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.ClusterMemberLeaveCount, err = meter.Int64Counter(
		"protocluster_member_leave_total",
		metric.WithDescription("Members left"),
	); err != nil {
		err = fmt.Errorf("failed to create ClusterMemberLeaveCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.ClusterActivationCount, err = meter.Int64UpDownCounter(
		"protocluster_activation_count",
		metric.WithDescription("Active virtual actor activations by kind on this node"),
	); err != nil {
		err = fmt.Errorf("failed to create ClusterActivationCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.GossipSentCount, err = meter.Int64Counter(
		"protocluster_gossip_sent_total",
		metric.WithDescription("Gossip messages sent"),
	); err != nil {
		err = fmt.Errorf("failed to create GossipSentCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.GossipReceivedCount, err = meter.Int64Counter(
		"protocluster_gossip_received_total",
		metric.WithDescription("Gossip messages received"),
	); err != nil {
		err = fmt.Errorf("failed to create GossipReceivedCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.GossipRoundtripDuration, err = meter.Float64Histogram(
		"protocluster_gossip_roundtrip_duration",
		metric.WithDescription("Gossip send round-trip time"),
		metric.WithUnit("s"),
	); err != nil {
		err = fmt.Errorf("failed to create GossipRoundtripDuration instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.GossipMessageSizeBytes, err = meter.Int64Histogram(
		"protocluster_gossip_message_size_bytes",
		metric.WithDescription("Size of gossip state payloads"),
		metric.WithUnit("By"),
	); err != nil {
		err = fmt.Errorf("failed to create GossipMessageSizeBytes instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.IdentityLookupDuration, err = meter.Float64Histogram(
		"protocluster_identity_lookup_duration",
		metric.WithDescription("Time to look up or activate a virtual actor identity"),
		metric.WithUnit("s"),
	); err != nil {
		err = fmt.Errorf("failed to create IdentityLookupDuration instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.IdentityLookupFailureCount, err = meter.Int64Counter(
		"protocluster_identity_lookup_failure_total",
		metric.WithDescription("Failed identity lookups"),
	); err != nil {
		err = fmt.Errorf("failed to create IdentityLookupFailureCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.IdentityCacheHitCount, err = meter.Int64Counter(
		"protocluster_identity_cache_hit_total",
		metric.WithDescription("PID cache hits"),
	); err != nil {
		err = fmt.Errorf("failed to create IdentityCacheHitCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.IdentityCacheMissCount, err = meter.Int64Counter(
		"protocluster_identity_cache_miss_total",
		metric.WithDescription("PID cache misses"),
	); err != nil {
		err = fmt.Errorf("failed to create IdentityCacheMissCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	return m
}
