// Copyright (C) 2017 - 2024 Asynkron.se <http://www.asynkron.se>

package actor

import (
	"fmt"
	"strings"

	"github.com/asynkron/protoactor-go/extensions"
	"github.com/asynkron/protoactor-go/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var extensionId = extensions.NextExtensionID()

type Metrics struct {
	metrics     *metrics.ProtoMetrics
	enabled     bool
	actorSystem *ActorSystem
}

var _ extensions.Extension = &Metrics{}

func (m *Metrics) Enabled() bool {
	return m.enabled
}

func (m *Metrics) ExtensionID() extensions.ExtensionID {
	return extensionId
}

// NewMetrics initializes metrics collection for the given actor system using the
// supplied OpenTelemetry MeterProvider. It also sets the global MeterProvider via
// otel.SetMeterProvider, which changes process-wide state so other packages will
// use the same provider.
func NewMetrics(system *ActorSystem, provider metric.MeterProvider) *Metrics {
	if provider == nil || !system.Config.MetricsEnabled {
		return &Metrics{}
	}

	// Configure the global OpenTelemetry MeterProvider so that subsequent metric
	// instruments use the supplied provider.
	otel.SetMeterProvider(provider)

	return &Metrics{
		metrics:     metrics.NewProtoMetrics(system.Logger()),
		enabled:     true,
		actorSystem: system,
	}
}

func (m *Metrics) CommonLabels(ctx Context) []attribute.KeyValue {
	labels := []attribute.KeyValue{
		attribute.String("address", ctx.ActorSystem().Address()),
		attribute.String("id", ctx.ActorSystem().ID),
		attribute.String("actortype", strings.Replace(fmt.Sprintf("%T", ctx.Actor()), "*", "", 1)),
	}

	return labels
}
