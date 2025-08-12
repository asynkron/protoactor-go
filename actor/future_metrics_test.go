package actor

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// TestFutureMetrics_NoTimeoutCount ensures non-timeout errors are not counted as timeouts.
func TestFutureMetrics_NoTimeoutCount(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	system := NewActorSystemWithConfig(&Config{
		MetricsProvider: provider,
		MetricsEnabled:  true,
		LoggerFactory: func(_ *ActorSystem) *slog.Logger {
			return slog.New(slog.NewTextHandler(io.Discard, nil))
		},
	})
	defer system.Shutdown()

	// Successful future; should increment FuturesCompletedCount.
	success := NewFuture(system, time.Second)
	system.Root.Send(success.PID(), "ok")
	_, _ = success.Result()

	// Future that resolves with a non-timeout error (dead letter).
	failed := NewFuture(system, time.Second)
	system.Root.Send(failed.PID(), &DeadLetterResponse{})
	_, _ = failed.Result()

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	var completed, timedout int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch m.Name {
			case "protoactor_future_completed_count":
				if data, ok := m.Data.(metricdata.Sum[int64]); ok && len(data.DataPoints) > 0 {
					completed = data.DataPoints[0].Value
				}
			case "protoactor_future_timedout_count":
				if data, ok := m.Data.(metricdata.Sum[int64]); ok && len(data.DataPoints) > 0 {
					timedout = data.DataPoints[0].Value
				}
			}
		}
	}

	if completed != 1 {
		t.Fatalf("expected 1 completed future, got %d", completed)
	}
	if timedout != 0 {
		t.Fatalf("expected 0 timed out futures, got %d", timedout)
	}
}

// TestFutureMetrics_StartedAndTimeoutCount ensures started and timed out futures
// are recorded correctly.
func TestFutureMetrics_StartedAndTimeoutCount(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	system := NewActorSystemWithConfig(&Config{
		MetricsProvider: provider,
		MetricsEnabled:  true,
		LoggerFactory: func(_ *ActorSystem) *slog.Logger {
			return slog.New(slog.NewTextHandler(io.Discard, nil))
		},
	})
	defer system.Shutdown()

	// One successful future.
	success := NewFuture(system, time.Second)
	system.Root.Send(success.PID(), "ok")
	_, _ = success.Result()

	// One future that times out.
	timeout := NewFuture(system, 50*time.Millisecond)
	_, _ = timeout.Result()
	time.Sleep(100 * time.Millisecond)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	var started, completed, timedout int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch m.Name {
			case "protoactor_future_started_count":
				if data, ok := m.Data.(metricdata.Sum[int64]); ok && len(data.DataPoints) > 0 {
					started = data.DataPoints[0].Value
				}
			case "protoactor_future_completed_count":
				if data, ok := m.Data.(metricdata.Sum[int64]); ok && len(data.DataPoints) > 0 {
					completed = data.DataPoints[0].Value
				}
			case "protoactor_future_timedout_count":
				if data, ok := m.Data.(metricdata.Sum[int64]); ok && len(data.DataPoints) > 0 {
					timedout = data.DataPoints[0].Value
				}
			}
		}
	}

	if started != 2 || completed != 1 || timedout != 1 {
		t.Fatalf("expected started=2 completed=1 timedout=1, got %d %d %d", started, completed, timedout)
	}
}
