// Package remotemetrics collects metrics for remote actors using OpenTelemetry.
package remotemetrics

import (
	"fmt"
	"log/slog"

	"github.com/asynkron/protoactor-go/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// RemoteMetrics contains OpenTelemetry instruments used to track remote
// actor operations such as message serialization and endpoint state.
type RemoteMetrics struct {
	RemoteWriteDuration             metric.Float64Histogram
	RemoteActorSpawnCount           metric.Int64Counter
	RemoteSerializedMessageCount    metric.Int64Counter
	RemoteDeserializedMessageCount  metric.Int64Counter
	RemoteEndpointConnectedCount    metric.Int64Counter
	RemoteEndpointDisconnectedCount metric.Int64Counter
}

// NewRemoteMetrics creates all metric instruments required for reporting remote
// activity to OpenTelemetry. Any failures to create instruments are logged.
func NewRemoteMetrics(logger *slog.Logger) *RemoteMetrics {
	meter := otel.Meter(metrics.LibName)
	m := &RemoteMetrics{}
	var err error

	if m.RemoteWriteDuration, err = meter.Float64Histogram(
		"protoremote_write_duration",
		metric.WithDescription("Time spent writing to the network stream"),
		metric.WithUnit("s"),
	); err != nil {
		err = fmt.Errorf("failed to create RemoteWriteDuration instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.RemoteActorSpawnCount, err = meter.Int64Counter(
		"protoremote_spawn_count",
		metric.WithDescription("Number of actors spawned over remote"),
	); err != nil {
		err = fmt.Errorf("failed to create RemoteActorSpawnCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.RemoteSerializedMessageCount, err = meter.Int64Counter(
		"protoremote_message_serialize_count",
		metric.WithDescription("Number of serialized messages"),
	); err != nil {
		err = fmt.Errorf("failed to create RemoteSerializedMessageCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.RemoteDeserializedMessageCount, err = meter.Int64Counter(
		"protoremote_message_deserialize_count",
		metric.WithDescription("Number of deserialized messages"),
	); err != nil {
		err = fmt.Errorf("failed to create RemoteDeserializedMessageCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.RemoteEndpointConnectedCount, err = meter.Int64Counter(
		"protoremote_endpoint_connected_count",
		metric.WithDescription("Number of endpoint connects"),
	); err != nil {
		err = fmt.Errorf("failed to create RemoteEndpointConnectedCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.RemoteEndpointDisconnectedCount, err = meter.Int64Counter(
		"protoremote_endpoint_disconnected_count",
		metric.WithDescription("Number of endpoint disconnects"),
	); err != nil {
		err = fmt.Errorf("failed to create RemoteEndpointDisconnectedCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	return m
}
