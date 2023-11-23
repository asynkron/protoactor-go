// Copyright (C) 2017 - 2022 Asynkron.se <http://www.asynkron.se>

package actor

import (
	"fmt"
	"log/slog"
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

func NewMetrics(system *ActorSystem, provider metric.MeterProvider) *Metrics {
	if provider == nil {
		return &Metrics{}
	}

	return &Metrics{
		metrics:     metrics.NewProtoMetrics(system.Logger()),
		enabled:     true,
		actorSystem: system,
	}
}

func (m *Metrics) PrepareMailboxLengthGauge() {
	meter := otel.Meter(metrics.LibName)
	gauge, err := meter.Int64ObservableGauge("protoactor_actor_mailbox_length",
		metric.WithDescription("Actor's Mailbox Length"),
		metric.WithUnit("1"))
	if err != nil {
		err = fmt.Errorf("failed to create ActorMailBoxLength instrument, %w", err)
		m.actorSystem.Logger().Error(err.Error(), slog.Any("error", err))
	}
	m.metrics.Instruments().SetActorMailboxLengthGauge(gauge)
}

func (m *Metrics) CommonLabels(ctx Context) []attribute.KeyValue {
	labels := []attribute.KeyValue{
		attribute.String("address", ctx.ActorSystem().Address()),
		attribute.String("actortype", strings.Replace(fmt.Sprintf("%T", ctx.Actor()), "*", "", 1)),
	}

	return labels
}
