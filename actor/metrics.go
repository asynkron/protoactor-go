// Copyright (C) 2017 - 2022 Asynkron.se <http://www.asynkron.se>

package actor

import (
	"fmt"
	"strings"

	"github.com/asynkron/protoactor-go/extensions"
	"github.com/asynkron/protoactor-go/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/unit"
)

var extensionId = extensions.NextExtensionID()

type Metrics struct {
	metrics *metrics.ProtoMetrics
	enabled bool
}

var _ extensions.Extension = &Metrics{}

func (m *Metrics) Enabled() bool {
	return m.enabled
}

func (m *Metrics) ExtensionID() extensions.ExtensionID {
	return extensionId
}

func NewMetrics(provider metric.MeterProvider) *Metrics {
	if provider == nil {
		return &Metrics{}
	}

	return &Metrics{
		metrics: metrics.NewProtoMetrics(provider),
		enabled: true,
	}
}

func (m *Metrics) PrepareMailboxLengthGauge(cb metric.Int64ObserverFunc) {
	meter := global.Meter(metrics.LibName)
	gauge := metric.Must(meter).NewInt64GaugeObserver(
		"protoactor_actor_mailbox_length",
		cb,
		metric.WithDescription("Actor's Mailbox Length"),
		metric.WithUnit(unit.Dimensionless),
	)
	m.metrics.Instruments().SetActorMailboxLengthGauge(gauge)
}

func (m *Metrics) CommonLabels(ctx Context) []attribute.KeyValue {
	labels := []attribute.KeyValue{
		attribute.String("address", ctx.ActorSystem().Address()),
		attribute.String("actortype", strings.Replace(fmt.Sprintf("%T", ctx.Actor()), "*", "", 1)),
	}
	return labels
}
