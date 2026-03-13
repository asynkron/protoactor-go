package k8smetrics

import (
	"fmt"
	"log/slog"

	"github.com/asynkron/protoactor-go/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// K8sMetrics contains OpenTelemetry instruments for tracking Kubernetes
// cluster provider operations.
type K8sMetrics struct {
	PodWatcherRestartCount metric.Int64Counter
	TopologyUpdateDuration metric.Float64Histogram
	PodReadinessDuration   metric.Float64Histogram
}

// NewK8sMetrics creates all metric instruments for the K8s provider.
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
