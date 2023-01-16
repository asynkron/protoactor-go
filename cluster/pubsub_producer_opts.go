package cluster

import (
	"github.com/asynkron/protoactor-go/actor"
	"time"
)

type BatchProducerConfigOption func(config *BatchingProducerConfig)

// WithBatchProducerBatchSize sets maximum size of the published batch. Default: 2000.
func WithBatchProducerBatchSize(batchSize int) BatchProducerConfigOption {
	return func(config *BatchingProducerConfig) {
		config.BatchSize = batchSize
	}
}

// WithBatchProducerMaxQueueSize set max size of the requests waiting in queue. If value is provided, the producer will throw
// ProducerQueueFullException when queue size is exceeded. If 0 or unset, the queue is unbounded
// Note that bounded queue has better performance than unbounded queue.
// Default: 0 (unbounded)
func WithBatchProducerMaxQueueSize(maxQueueSize int) BatchProducerConfigOption {
	return func(config *BatchingProducerConfig) {
		config.MaxQueueSize = maxQueueSize
	}
}

// WithBatchProducerPublishTimeout sets how long to wait for the publishing to complete.
// Default: 5s
func WithBatchProducerPublishTimeout(publishTimeout time.Duration) BatchProducerConfigOption {
	return func(config *BatchingProducerConfig) {
		config.PublishTimeout = publishTimeout
	}
}

// WithBatchProducerOnPublishingError sets error handler that can decide what to do with an error when publishing a batch.
// Default: Fail and stop the BatchingProducer
func WithBatchProducerOnPublishingError(onPublishingError PublishingErrorHandler) BatchProducerConfigOption {
	return func(config *BatchingProducerConfig) {
		config.OnPublishingError = onPublishingError
	}
}

// WithBatchProducerLogThrottle sets a throttle for logging from this producer. By default, a throttle shared between all instances of
// BatchingProducer is used, that allows for 10 events in 10 seconds.
func WithBatchProducerLogThrottle(logThrottle actor.ShouldThrottle) BatchProducerConfigOption {
	return func(config *BatchingProducerConfig) {
		config.LogThrottle = logThrottle
	}
}

// WithBatchProducerPublisherIdleTimeout sets an optional idle timeout which will specify to the `IPublisher` how long it should wait before invoking clean
// up code to recover resources.
func WithBatchProducerPublisherIdleTimeout(publisherIdleTimeout time.Duration) BatchProducerConfigOption {
	return func(config *BatchingProducerConfig) {
		config.PublisherIdleTimeout = publisherIdleTimeout
	}
}

type PublishingErrorDecision struct {
	Delay time.Duration
}

// NewPublishingErrorDecision creates a new PublishingErrorDecision
func NewPublishingErrorDecision(delay time.Duration) PublishingErrorDecision {
	return PublishingErrorDecision{Delay: delay}
}

// RetryBatchAfter returns a new PublishingErrorDecision with the Delay set to the given duration
func RetryBatchAfter(delay time.Duration) PublishingErrorDecision {
	return NewPublishingErrorDecision(delay)
}

// FailBatchAndStop causes the BatchingProducer to stop and fail the pending messages
var FailBatchAndStop = NewPublishingErrorDecision(0)

// FailBatchAndContinue skips the current batch and proceeds to the next one. The delivery reports (tasks) related to that batch are still
// failed with the exception that triggered the error handling.
var FailBatchAndContinue = NewPublishingErrorDecision(0)

// RetryBatchImmediately retries the current batch immediately
var RetryBatchImmediately = NewPublishingErrorDecision(0)
