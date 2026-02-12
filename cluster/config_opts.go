package cluster

import (
	"time"
)

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

func WithRequestLog(enabled bool) ConfigOption {
	return func(c *Config) {
		c.RequestLog = enabled
	}
}

// WithGossipInterval sets the interval between gossip rounds.
func WithGossipInterval(d time.Duration) ConfigOption {
	return func(c *Config) {
		c.GossipInterval = d
	}
}

// WithGossipRequestTimeout sets the timeout for gossip requests.
func WithGossipRequestTimeout(d time.Duration) ConfigOption {
	return func(c *Config) {
		c.GossipRequestTimeout = d
	}
}

// WithGossipFanOut sets the number of peers to gossip with per round.
func WithGossipFanOut(n int) ConfigOption {
	return func(c *Config) {
		c.GossipFanOut = n
	}
}

// WithGossipMaxSend sets the maximum number of gossip messages per round.
func WithGossipMaxSend(n int) ConfigOption {
	return func(c *Config) {
		c.GossipMaxSend = n
	}
}

// WithMemberStrategyBuilder sets the strategy builder for member selection.
func WithMemberStrategyBuilder(b func(cluster *Cluster, kind string) MemberStrategy) ConfigOption {
	return func(c *Config) {
		c.MemberStrategyBuilder = b
	}
}
