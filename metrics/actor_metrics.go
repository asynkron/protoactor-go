// Copyright (C) 2017 - 2022 Asynkron.se <http://www.asynkron.se>

package metrics

import (
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/unit"
)

const libName string = "protoactor"

type ActorMetrics struct {
	// Actors
	ActorFailureCount metric.Int64Counter
	// actorMailboxLength
	ActorMessageReceiveHistogram metric.Int64Histogram
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
