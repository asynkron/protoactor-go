// Package clustermetrics reports cluster metrics via OpenTelemetry.
package clustermetrics

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/asynkron/protoactor-go/metrics"
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

	return m
}
