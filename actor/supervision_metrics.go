package actor

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/asynkron/protoactor-go/metrics"
)

// recordSupervisionMetric records a supervision event metric if metrics are enabled.
func recordSupervisionMetric(actorSystem *ActorSystem, child *PID, strategy, action string) {
	if !actorSystem.Config.MetricsEnabled {
		return
	}
	metricsSystem, ok := actorSystem.Extensions.Get(extensionID).(*Metrics)
	if !ok || !metricsSystem.Enabled() {
		return
	}
	m := metricsSystem.metrics.Get(metrics.InternalActorMetrics)
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
