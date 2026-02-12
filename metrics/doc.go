// Package metrics provides OpenTelemetry instrumentation for actor systems.
//
// The metrics package exposes built-in metrics for actors including spawn/stop counts,
// failure counts, mailbox length, message processing duration, and future completion metrics.
// All metrics are exposed through OpenTelemetry and can be exported to any compatible backend.
//
// Key types:
//   - ProtoMetrics: Manages metric instruments for the actor system
//   - ActorMetrics: Collection of OpenTelemetry metric instruments for actors
//
// Available metrics:
//   - ActorSpawnCount: Number of actors spawned
//   - ActorStoppedCount: Number of actors stopped
//   - ActorFailureCount: Number of actor failures
//   - ActorRestartedCount: Number of actor restarts
//   - ActorMailboxLength: Mailbox queue depth histogram
//   - ActorMessageReceiveDuration: Message processing time histogram
//   - DeadLetterCount: Number of dead letters
//   - FuturesStartedCount: Number of futures created
//   - FuturesCompletedCount: Number of futures completed
//   - FuturesTimedOutCount: Number of futures that timed out
//
// Basic usage:
//
//	// Metrics are automatically enabled when creating an actor system with MetricsEnabled
//	config := actor.NewConfig().WithMetricsEnabled(true)
//	system := actor.NewActorSystemWithConfig(config)
//
//	// Access metrics programmatically
//	protoMetrics := system.Metrics()
//	instruments := protoMetrics.Instruments()
package metrics
