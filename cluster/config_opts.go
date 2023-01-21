package cluster

import "time"

type ConfigOption func(config *Config)

// WithRequestTimeout sets the request timeout.
func WithRequestTimeout(t time.Duration) ConfigOption {
	return func(c *Config) {
		c.RequestTimeoutTime = t
	}
}

// WithRequestsLogThrottlePeriod sets the requests log throttle period.
func WithRequestsLogThrottlePeriod(period time.Duration) ConfigOption {
	return func(c *Config) {
		c.RequestsLogThrottlePeriod = period
	}
}

// WithClusterContextProducer sets the cluster context producer.
func WithClusterContextProducer(producer ContextProducer) ConfigOption {
	return func(c *Config) {
		c.ClusterContextProducer = producer
	}
}

// WithMaxNumberOfEventsInRequestLogThrottlePeriod sets the max number of events in request log throttled period.
func WithMaxNumberOfEventsInRequestLogThrottlePeriod(maxNumber int) ConfigOption {
	return func(c *Config) {
		c.MaxNumberOfEventsInRequestLogThrottledPeriod = maxNumber
	}
}

func WithKinds(kinds ...*Kind) ConfigOption {
	return func(c *Config) {
		for _, kind := range kinds {
			c.Kinds[kind.Kind] = kind
		}
	}
}

// WithPubSubSubscriberTimeout sets a timeout used when delivering a message batch to a subscriber.
// Default is 5s.
func WithPubSubSubscriberTimeout(timeout time.Duration) ConfigOption {
	return func(c *Config) {
		c.PubSubConfig.SubscriberTimeout = timeout
	}
}

// WithHeartbeatExpiration sets the gossip heartbeat expiration.
func WithHeartbeatExpiration(t time.Duration) ConfigOption {
	return func(c *Config) {
		c.HeartbeatExpiration = t
	}
}
