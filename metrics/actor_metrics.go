// Copyright (C) 2017 - 2022 Asynkron.se <http://www.asynkron.se>

package metrics

import (
	"fmt"
	"sync"

	"github.com/AsynkronIT/protoactor-go/log"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/unit"
)

const LibName string = "protoactor"

type ActorMetrics struct {
	// Mutual Exclusion Primitive to use with ActorMailboxLength
	mu *sync.Mutex

	// MetricsID
	ID string

	// Actors
	ActorFailureCount            metric.Int64Counter
	ActorMailboxLength           metric.Int64GaugeObserver
	ActorMessageReceiveHistogram metric.Float64Histogram
	ActorRestartedCount          metric.Int64Counter
	ActorSpawnCount              metric.Int64Counter
	ActorStoppedCount            metric.Int64Counter

	// Deadletters
	DeadLetterCount       metric.Int64Counter
	FuturesCompletedCount metric.Int64Counter

	// Futures
	FuturesStartedCount  metric.Int64Counter
	FuturesTimedOutCount metric.Int64Counter

	// Threadpool
	ThreadPoolLatency metric.Int64Histogram
}

// NewActorMetrics creates a new ActorMetrics value and returns a pointer to it
func NewActorMetrics() *ActorMetrics {

	instruments := newInstruments()
	return instruments
}

// newInstruments will create instruments using a meter from
// the given provider p
func newInstruments() *ActorMetrics {

	meter := global.Meter(LibName)
	instruments := ActorMetrics{mu: &sync.Mutex{}}

	var err error
	if instruments.ActorFailureCount, err = meter.NewInt64Counter(
		"protoactor_actor_failure_count",
		metric.WithDescription("Number of actor failures"),
		metric.WithUnit(unit.Dimensionless),
	); err != nil {
		err = fmt.Errorf("Failed to create ActorFailureCount instrument, %w", err)
		plog.Error(err.Error(), log.Error(err))
	}

	if instruments.ActorMessageReceiveHistogram, err = meter.NewFloat64Histogram(
		"protoactor_actor_messagereceive_duration_seconds",
		metric.WithDescription("Actor's messages received duration in seconds"),
	); err != nil {
		err = fmt.Errorf("Failed to create ActorMessageReceiveHistogram instrument, %w", err)
		plog.Error(err.Error(), log.Error(err))
	}

	if instruments.ActorRestartedCount, err = meter.NewInt64Counter(
		"protoactor_actor_restarted_count",
		metric.WithDescription("Number of actors retarts"),
		metric.WithUnit(unit.Dimensionless),
	); err != nil {
		err = fmt.Errorf("Failed to create ActorRestartedCount instrument, %w", err)
		plog.Error(err.Error(), log.Error(err))
	}

	if instruments.ActorStoppedCount, err = meter.NewInt64Counter(
		"protoactor_actor_stopped_count",
		metric.WithDescription("Number of actors stopped"),
		metric.WithUnit(unit.Dimensionless),
	); err != nil {
		err = fmt.Errorf("Failed to create ActorStoppedCount instrument, %w", err)
		plog.Error(err.Error(), log.Error(err))
	}

	if instruments.ActorSpawnCount, err = meter.NewInt64Counter(
		"protoactor_actor_spawn_count",
		metric.WithDescription("Number of actors spawn"),
		metric.WithUnit(unit.Dimensionless),
	); err != nil {
		err = fmt.Errorf("Failed to create ActorSpawnCount instrument, %w", err)
		plog.Error(err.Error(), log.Error(err))
	}

	if instruments.DeadLetterCount, err = meter.NewInt64Counter(
		"protoactor_deadletter_count",
		metric.WithDescription("Number of deadletters"),
		metric.WithUnit(unit.Dimensionless),
	); err != nil {
		err = fmt.Errorf("Failed to create DeadLetterCount instrument, %w", err)
		plog.Error(err.Error(), log.Error(err))
	}

	if instruments.FuturesCompletedCount, err = meter.NewInt64Counter(
		"protoactor_futures_completed_count",
		metric.WithDescription("Number of futures completed"),
		metric.WithUnit(unit.Dimensionless),
	); err != nil {
		err = fmt.Errorf("Failed to create FuturesCompletedCount instrument, %w", err)
		plog.Error(err.Error(), log.Error(err))
	}

	if instruments.FuturesStartedCount, err = meter.NewInt64Counter(
		"protoactor_futures_started_count",
		metric.WithDescription("Number of futures started"),
		metric.WithUnit(unit.Dimensionless),
	); err != nil {
		err = fmt.Errorf("Failed to create FuturesStartedCount instrument, %w", err)
		plog.Error(err.Error(), log.Error(err))
	}

	if instruments.FuturesTimedOutCount, err = meter.NewInt64Counter(
		"protoactor_futures_timedout_count",
		metric.WithDescription("Number of futures timed out"),
		metric.WithUnit(unit.Dimensionless),
	); err != nil {
		err = fmt.Errorf("Failed to create FuturesTimedOutCount instrument, %w", err)
		plog.Error(err.Error(), log.Error(err))
	}

	if instruments.ThreadPoolLatency, err = meter.NewInt64Histogram(
		"protoactor_threadpool_latency_duraton_seconds",
		metric.WithDescription("History of latency in second"),
		metric.WithUnit(unit.Milliseconds),
	); err != nil {
		err = fmt.Errorf("Failed to create ThreadPoolLatency instrument, %w", err)
		plog.Error(err.Error(), log.Error(err))
	}

	return &instruments
}

// Makes sure access to ActorMailboxLength is sequenced
func (am *ActorMetrics) SetActorMailboxLengthGauge(gauge metric.Int64GaugeObserver) {

	// lock our mutex
	am.mu.Lock()
	defer am.mu.Unlock()

	am.ActorMailboxLength = gauge
}
