package cluster

import (
	"time"

	"github.com/asynkron/protoactor-go/actor"
)

type BatchingProducerConfigOption func(config *BatchingProducerConfig)

// WithBatchingProducerBatchSize sets maximum size of the published batch. Default: 2000.
func WithBatchingProducerBatchSize(batchSize int) BatchingProducerConfigOption {
	return func(config *BatchingProducerConfig) {
		config.BatchSize = batchSize
	}
}

// WithBatchingProducerMaxQueueSize set max size of the requests waiting in queue. If value is provided, the producer will throw
// ProducerQueueFullException when queue size is exceeded. If 0 or unset, the queue is unbounded
// Note that bounded queue has better performance than unbounded queue.
// Default: 0 (unbounded)
func WithBatchingProducerMaxQueueSize(maxQueueSize int) BatchingProducerConfigOption {
	return func(config *BatchingProducerConfig) {
		config.MaxQueueSize = maxQueueSize
	}
}

// WithBatchingProducerPublishTimeout sets how long to wait for the publishing to complete.
// Default: 5s
func WithBatchingProducerPublishTimeout(publishTimeout time.Duration) BatchingProducerConfigOption {
	return func(config *BatchingProducerConfig) {
		config.PublishTimeout = publishTimeout
	}
}

// WithBatchingProducerOnPublishingError sets error handler that can decide what to do with an error when publishing a batch.
// Default: Fail and stop the BatchingProducer
func WithBatchingProducerOnPublishingError(onPublishingError PublishingErrorHandler) BatchingProducerConfigOption {
	return func(config *BatchingProducerConfig) {
		config.OnPublishingError = onPublishingError
	}
}

// WithBatchingProducerLogThrottle sets a throttle for logging from this producer. By default, a throttle shared between all instances of
// BatchingProducer is used, that allows for 10 events in 10 seconds.
func WithBatchingProducerLogThrottle(logThrottle actor.ShouldThrottle) BatchingProducerConfigOption {
	return func(config *BatchingProducerConfig) {
		config.LogThrottle = logThrottle
	}
}

// WithBatchingProducerPublisherIdleTimeout sets an optional idle timeout which will specify to the `IPublisher` how long it should wait before invoking clean
// up code to recover resources.
func WithBatchingProducerPublisherIdleTimeout(publisherIdleTimeout time.Duration) BatchingProducerConfigOption {
	return func(config *BatchingProducerConfig) {
		config.PublisherIdleTimeout = publisherIdleTimeout
	}
}

type PublishingErrorDecision struct {
	Delay time.Duration
}

// NewPublishingErrorDecision creates a new PublishingErrorDecision
func NewPublishingErrorDecision(delay time.Duration) *PublishingErrorDecision {
	return &PublishingErrorDecision{Delay: delay}
}

// RetryBatchAfter returns a new PublishingErrorDecision with the Delay set to the given duration
func RetryBatchAfter(delay time.Duration) *PublishingErrorDecision {
	return NewPublishingErrorDecision(delay)
}

// FailBatchAndStop causes the BatchingProducer to stop and fail the pending messages
var FailBatchAndStop = NewPublishingErrorDecision(0)

// FailBatchAndContinue skips the current batch and proceeds to the next one. The delivery reports (tasks) related to that batch are still
// failed with the exception that triggered the error handling.
var FailBatchAndContinue = NewPublishingErrorDecision(0)

// RetryBatchImmediately retries the current batch immediately
var RetryBatchImmediately = NewPublishingErrorDecision(0)
