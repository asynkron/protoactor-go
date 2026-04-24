// Package remotemetrics collects metrics for remote actors using OpenTelemetry.
package remotemetrics

import (
	"fmt"
	"log/slog"

	"github.com/awevoke/protoactor-go/metrics"
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
	RemoteMessageBatchSize          metric.Int64Histogram
	RemoteMessageSizeBytes          metric.Int64Histogram
	RemoteInflightRequests          metric.Int64UpDownCounter
	RemoteMessageSentTotal          metric.Int64Counter
	RemoteMessageReceivedTotal      metric.Int64Counter
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

	if m.RemoteMessageBatchSize, err = meter.Int64Histogram(
		"protoremote_message_batch_size",
		metric.WithDescription("Envelopes per batch on remote writes"),
	); err != nil {
		err = fmt.Errorf("failed to create RemoteMessageBatchSize instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.RemoteMessageSizeBytes, err = meter.Int64Histogram(
		"protoremote_message_size_bytes",
		metric.WithDescription("Serialized payload size in bytes"),
		metric.WithUnit("By"),
	); err != nil {
		err = fmt.Errorf("failed to create RemoteMessageSizeBytes instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.RemoteInflightRequests, err = meter.Int64UpDownCounter(
		"protoremote_inflight_requests",
		metric.WithDescription("Currently pending remote requests/futures"),
	); err != nil {
		err = fmt.Errorf("failed to create RemoteInflightRequests instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.RemoteMessageSentTotal, err = meter.Int64Counter(
		"protoremote_message_sent_total",
		metric.WithDescription("Per-destination message counts (opt-in, O(n^2) cardinality)"),
	); err != nil {
		err = fmt.Errorf("failed to create RemoteMessageSentTotal instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if m.RemoteMessageReceivedTotal, err = meter.Int64Counter(
		"protoremote_message_received_total",
		metric.WithDescription("Per-source message counts (opt-in, O(n^2) cardinality)"),
	); err != nil {
		err = fmt.Errorf("failed to create RemoteMessageReceivedTotal instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	return m
}
