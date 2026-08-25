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

	// JanitorSweepDuration observes how long one janitor sweep took, in
	// SECONDS. The sweep is one enumeration of the identities bucket plus one
	// Get per live key plus one enumeration of the members bucket, so its cost
	// tracks the live key count; this is the only in-process report of it.
	JanitorSweepDuration metric.Float64Histogram

	// JanitorLiveKeys reports the number of LIVE identity keys the most recent
	// sweep enumerated. A gauge, not a counter: each sweep replaces the
	// previous answer rather than adding to it.
	JanitorLiveKeys metric.Int64Gauge

	// JanitorTombstonePurgeTotal counts identity records the janitor actually
	// purged, i.e. purges that stored a delete marker. A CAS-rejected purge
	// and a delete of an already-absent key store nothing and are not counted.
	JanitorTombstonePurgeTotal metric.Int64Counter

	// MarkerTTLEnabled reports, per identity bucket, whether that bucket was
	// created with KV marker TTLs (1) or fell back to retaining every delete
	// marker forever (0). Recorded once per process from the provider's start,
	// so the series exists from the first scrape rather than appearing only
	// once something has gone wrong.
	MarkerTTLEnabled metric.Int64Gauge
}

// janitorSweepBuckets are the explicit histogram boundaries for
// JanitorSweepDuration, in seconds.
//
// They are declared rather than defaulted because the OTel SDK's default
// boundaries (0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000,
// 7500, 10000) are MILLISECOND-scaled: against a seconds-valued observation
// every sweep this platform will ever run lands in the first non-zero bucket,
// so every quantile computed from it -- including the p99 the
// NatsKVJanitorSweepSlow alert evaluates -- is an interpolation across
// (0s, 5s] and therefore meaningless.
//
// The set is Prometheus's own DefBuckets plus 30s. 0.25 is present because it
// is that alert's threshold, so "p99 is at or under 250 ms" resolves exactly
// rather than by interpolation; 30s is the default JanitorInterval, so a sweep
// that has grown to consume its own interval is visible as the top finite
// bucket rather than only as +Inf.
var janitorSweepBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30}

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

	if m.JanitorSweepDuration, err = meter.Float64Histogram(
		"protocluster_natskv_janitor_sweep_duration",
		metric.WithDescription("Time one janitor sweep of the identities bucket took"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(janitorSweepBuckets...),
	); err != nil {
		err = fmt.Errorf("failed to create JanitorSweepDuration instrument, %w", err)
		logErr(err.Error(), err)
	}

	if m.JanitorLiveKeys, err = meter.Int64Gauge(
		"protocluster_natskv_janitor_live_keys",
		metric.WithDescription("Live identity keys the most recent janitor sweep enumerated"),
	); err != nil {
		err = fmt.Errorf("failed to create JanitorLiveKeys instrument, %w", err)
		logErr(err.Error(), err)
	}

	if m.JanitorTombstonePurgeTotal, err = meter.Int64Counter(
		"protocluster_natskv_janitor_tombstone_purge_total",
		metric.WithDescription("Identity records the janitor purged, leaving a delete marker"),
	); err != nil {
		err = fmt.Errorf("failed to create JanitorTombstonePurgeTotal instrument, %w", err)
		logErr(err.Error(), err)
	}

	if m.MarkerTTLEnabled, err = meter.Int64Gauge(
		"protocluster_natskv_marker_ttl_enabled",
		metric.WithDescription("1 when an identity bucket's delete markers expire server-side, 0 when they are retained forever"),
	); err != nil {
		err = fmt.Errorf("failed to create MarkerTTLEnabled instrument, %w", err)
		logErr(err.Error(), err)
	}

	return m
}
