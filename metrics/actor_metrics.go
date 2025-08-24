// Copyright (C) 2017 - 2024 Asynkron.se <http://www.asynkron.se>

// Package metrics exposes instrumentation helpers for Proto.Actor.
package metrics

import (
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// LibName identifies the metrics instrumentation library.
const LibName string = "protoactor"

// ActorMetrics groups the metric instruments used by actors.
type ActorMetrics struct {
	// Actors
	ActorFailureCount           metric.Int64Counter
	ActorMailboxLength          metric.Int64Histogram
	ActorMessageReceiveDuration metric.Float64Histogram
	ActorRestartedCount         metric.Int64Counter
	ActorSpawnCount             metric.Int64Counter
	ActorStoppedCount           metric.Int64Counter

	// Deadletters
	DeadLetterCount metric.Int64Counter

	// Futures
	FuturesStartedCount   metric.Int64Counter
	FuturesCompletedCount metric.Int64Counter
	FuturesTimedOutCount  metric.Int64Counter
}

// NewActorMetrics creates a new ActorMetrics value and returns a pointer to it
func NewActorMetrics(logger *slog.Logger) *ActorMetrics {
	instruments := newInstruments(logger)
	return instruments
}

// newInstruments will create instruments using a meter from
// the given provider p
func newInstruments(logger *slog.Logger) *ActorMetrics {
	meter := otel.Meter(LibName)
	instruments := ActorMetrics{}

	var err error

	if instruments.ActorFailureCount, err = meter.Int64Counter(
		"protoactor_actor_failure_count",
		metric.WithDescription("Number of detected and escalated failures"),
	); err != nil {
		err = fmt.Errorf("failed to create ActorFailureCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if instruments.ActorMailboxLength, err = meter.Int64Histogram(
		"protoactor_actor_mailbox_length",
		metric.WithDescription("Histogram of queue lengths across all actor instances"),
	); err != nil {
		err = fmt.Errorf("failed to create ActorMailboxLength instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if instruments.ActorMessageReceiveDuration, err = meter.Float64Histogram(
		"protoactor_actor_messagereceive_duration",
		metric.WithDescription("Time spent in actor's receive handler"),
		metric.WithUnit("s"),
	); err != nil {
		err = fmt.Errorf("failed to create ActorMessageReceiveDuration instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if instruments.ActorRestartedCount, err = meter.Int64Counter(
		"protoactor_actor_restarted_count",
		metric.WithDescription("Number of restarted actors"),
	); err != nil {
		err = fmt.Errorf("failed to create ActorRestartedCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if instruments.ActorStoppedCount, err = meter.Int64Counter(
		"protoactor_actor_stopped_count",
		metric.WithDescription("Number of stopped actors"),
	); err != nil {
		err = fmt.Errorf("failed to create ActorStoppedCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if instruments.ActorSpawnCount, err = meter.Int64Counter(
		"protoactor_actor_spawn_count",
		metric.WithDescription("Number of spawned actor instances"),
	); err != nil {
		err = fmt.Errorf("failed to create ActorSpawnCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if instruments.DeadLetterCount, err = meter.Int64Counter(
		"protoactor_deadletter_count",
		metric.WithDescription("Number of messages sent to deadletter process"),
	); err != nil {
		err = fmt.Errorf("failed to create DeadLetterCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if instruments.FuturesCompletedCount, err = meter.Int64Counter(
		"protoactor_future_completed_count",
		metric.WithDescription("Number of completed futures"),
	); err != nil {
		err = fmt.Errorf("failed to create FuturesCompletedCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if instruments.FuturesStartedCount, err = meter.Int64Counter(
		"protoactor_future_started_count",
		metric.WithDescription("Number of started futures"),
	); err != nil {
		err = fmt.Errorf("failed to create FuturesStartedCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	if instruments.FuturesTimedOutCount, err = meter.Int64Counter(
		"protoactor_future_timedout_count",
		metric.WithDescription("Number of futures that timed out"),
	); err != nil {
		err = fmt.Errorf("failed to create FuturesTimedOutCount instrument, %w", err)
		logger.Error(err.Error(), slog.Any("error", err))
	}

	return &instruments
}
