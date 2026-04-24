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
	TopologyUpdateDuration metric.Float64Histogram
	KeyRefreshFailureCount metric.Int64Counter
	WatchReconnectCount    metric.Int64Counter
	LeaderElectionCount    metric.Int64Counter
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
