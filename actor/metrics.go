// Copyright (C) 2017 - 2022 Asynkron.se <http://www.asynkron.se>

package actor

import (
	"fmt"
	"strings"

	"github.com/AsynkronIT/protoactor-go/extensions"
	"github.com/AsynkronIT/protoactor-go/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/unit"
)

var extensionId = extensions.NextExtensionId()

type Metrics struct {
	metrics *metrics.ProtoMetrics
	enabled bool
}

func (m *Metrics) Enabled() bool {
	return m.enabled
}
func (m *Metrics) Id() extensions.ExtensionId {
	return extensionId
}

func NewMetrics(provider metric.MeterProvider) *Metrics {

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
