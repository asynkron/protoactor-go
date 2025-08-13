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

var extensionID = extensions.NextExtensionID()

// Metrics provides access to system-wide metric instrumentation.
type Metrics struct {
	metrics     *metrics.ProtoMetrics
	enabled     bool
	actorSystem *ActorSystem
}

var _ extensions.Extension = &Metrics{}

// Enabled reports whether metrics collection is enabled.
func (m *Metrics) Enabled() bool {
	return m.enabled
}

// ExtensionID returns the unique ID for the metrics extension.
func (m *Metrics) ExtensionID() extensions.ExtensionID {
	return extensionID
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

// SystemLabels returns a standard set of attributes that identify the actor system
// emitting the metric. These labels are used across modules to maintain
// consistency in OpenTelemetry reporting.
func SystemLabels(system *ActorSystem) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("address", system.Address()),
		attribute.String("id", system.ID),
	}
}

// CommonLabels returns the default set of labels for an actor metric, including
// system-wide labels and the specific actor type.
func (m *Metrics) CommonLabels(ctx Context) []attribute.KeyValue {
	return append(SystemLabels(ctx.ActorSystem()),
		attribute.String("actortype", strings.Replace(fmt.Sprintf("%T", ctx.Actor()), "*", "", 1)),
	)
}
