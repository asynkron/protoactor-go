// Copyright (C) 2017 - 2024 Asynkron.se <http://www.asynkron.se>

package actor

import (
	"reflect"
	"strings"
	"sync"

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

// actorTypeLabels caches the "actortype" attribute keyed by the actor's concrete
// reflect.Type, avoiding the repeated reflection/formatting cost of computing the
// type name on every metric emission.
var actorTypeLabels sync.Map // map[reflect.Type]attribute.KeyValue

// actorTypeLabel returns a cached "actortype" label for the given actor, computing
// (and caching) it on first use for each concrete type.
func actorTypeLabel(actor Actor) attribute.KeyValue {
	t := reflect.TypeOf(actor)
	if t == nil {
		return attribute.String("actortype", "<nil>")
	}
	if v, ok := actorTypeLabels.Load(t); ok {
		return v.(attribute.KeyValue)
	}

	// Match the previous fmt.Sprintf("%T", ...) output, stripping a leading '*'
	// for pointer types.
	name := t.String()
	if t.Kind() == reflect.Ptr {
		name = strings.Replace(name, "*", "", 1)
	}

	kv := attribute.String("actortype", name)
	actorTypeLabels.Store(t, kv)

	return kv
}

// CommonLabels returns the default set of labels for an actor metric, including
// system-wide labels and the specific actor type.
func (m *Metrics) CommonLabels(ctx Context) []attribute.KeyValue {
	return append(SystemLabels(ctx.ActorSystem()), actorTypeLabel(ctx.Actor()))
}
