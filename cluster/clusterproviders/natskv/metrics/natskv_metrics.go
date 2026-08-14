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

	// IdentityWriteFailureTotal counts non-benign write failures on the
	// identities / member-tracking buckets, labelled by op (the write site).
	// A CAS conflict is NOT a failure and is not counted here.
	IdentityWriteFailureTotal metric.Int64Counter

	// JanitorSweepTotal counts completed janitor sweeps, labelled by outcome
	// ("clean", "work", or "error").
	JanitorSweepTotal metric.Int64Counter

	// JanitorReapTotal counts records reaped by the janitor, labelled by type
	// ("lock" or "activation").
	JanitorReapTotal metric.Int64Counter
}

// NewNatsKVMetrics creates all metric instruments for the NATS KV provider.
func NewNatsKVMetrics(logger *slog.Logger) *NatsKVMetrics {
	meter := otel.Meter(metrics.LibName)
	m := &NatsKVMetrics{}
	var err error

	logErr := func(msg string, logErr error) {
		if logger != nil {
			logger.Error(msg, slog.Any("error", logErr))
		}
	}

	if m.TopologyUpdateDuration, err = meter.Float64Histogram(
		"protocluster_natskv_topology_update_duration",
		metric.WithDescription("Time to process a topology update"),
		metric.WithUnit("s"),
	); err != nil {
		err = fmt.Errorf("failed to create TopologyUpdateDuration instrument, %w", err)
		logErr(err.Error(), err)
	}

	if m.KeyRefreshFailureCount, err = meter.Int64Counter(
		"protocluster_natskv_key_refresh_failure_total",
		metric.WithDescription("Failed member/leader key refreshes"),
	); err != nil {
		err = fmt.Errorf("failed to create KeyRefreshFailureCount instrument, %w", err)
		logErr(err.Error(), err)
	}

	if m.WatchReconnectCount, err = meter.Int64Counter(
		"protocluster_natskv_watch_reconnect_total",
		metric.WithDescription("KV watch reconnections"),
	); err != nil {
		err = fmt.Errorf("failed to create WatchReconnectCount instrument, %w", err)
		logErr(err.Error(), err)
	}

	if m.LeaderElectionCount, err = meter.Int64Counter(
		"protocluster_natskv_leader_election_total",
		metric.WithDescription("Leader election events"),
	); err != nil {
		err = fmt.Errorf("failed to create LeaderElectionCount instrument, %w", err)
		logErr(err.Error(), err)
	}

	if m.LockWaitTimeoutTotal, err = meter.Int64Counter(
		"protocluster_natskv_lock_wait_timeout_total",
		metric.WithDescription("Spawn-lock wait timeouts by outcome (activated_late or still_locked)"),
	); err != nil {
		err = fmt.Errorf("failed to create LockWaitTimeoutTotal instrument, %w", err)
		logErr(err.Error(), err)
	}

	if m.IdentityWriteFailureTotal, err = meter.Int64Counter(
		"protocluster_natskv_identity_write_failure_total",
		metric.WithDescription("Non-benign identity/tracking-bucket write failures by op"),
	); err != nil {
		err = fmt.Errorf("failed to create IdentityWriteFailureTotal instrument, %w", err)
		logErr(err.Error(), err)
	}

	if m.JanitorSweepTotal, err = meter.Int64Counter(
		"protocluster_natskv_janitor_sweep_total",
		metric.WithDescription("Completed janitor sweeps by outcome (clean, work, or error)"),
	); err != nil {
		err = fmt.Errorf("failed to create JanitorSweepTotal instrument, %w", err)
		logErr(err.Error(), err)
	}

	if m.JanitorReapTotal, err = meter.Int64Counter(
		"protocluster_natskv_janitor_reap_total",
		metric.WithDescription("Records reaped by the janitor by type (lock or activation)"),
	); err != nil {
		err = fmt.Errorf("failed to create JanitorReapTotal instrument, %w", err)
		logErr(err.Error(), err)
	}

	return m
}
