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
	TopologyUpdateDuration metric.Float64Histogram
	KeyRefreshFailureCount metric.Int64Counter
	WatchReconnectCount    metric.Int64Counter
	LeaderElectionCount    metric.Int64Counter
	LockWaitTimeoutTotal   metric.Int64Counter
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

	if m.LockWaitTimeoutTotal, err = meter.Int64Counter(
		"protocluster_natskv_lock_wait_timeout_total",
		metric.WithDescription("Spawn-lock wait timeouts by outcome (activated_late or still_locked)"),
	); err != nil {
		err = fmt.Errorf("failed to create LockWaitTimeoutTotal instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	return m
}
