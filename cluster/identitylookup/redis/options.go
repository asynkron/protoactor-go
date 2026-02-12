package redis

import (
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Config holds the configuration for the Redis identity storage backend.
type Config struct {
	// RedisClient is the go-redis client used for all Redis operations.
	RedisClient *goredis.Client

	// ClusterName is the name of the Proto.Actor cluster. It is used as a
	// prefix for all Redis keys to allow multiple clusters to share the same
	// Redis instance.
	ClusterName string

	// LockTTL is the maximum time a spawn lock is held before Redis
	// automatically expires it. This prevents stale locks if a node crashes
	// during activation. Defaults to 5 seconds.
	LockTTL time.Duration

	// MaxConcurrency limits the number of concurrent Redis operations.
	// Defaults to 200.
	MaxConcurrency int
}

// Option is a functional option for configuring Config.
type Option func(*Config)

// WithLockTTL sets the lock time-to-live duration.
func WithLockTTL(ttl time.Duration) Option {
	return func(c *Config) {
		c.LockTTL = ttl
	}
}

// WithMaxConcurrency sets the maximum number of concurrent Redis operations.
func WithMaxConcurrency(n int) Option {
	return func(c *Config) {
		c.MaxConcurrency = n
	}
}

func defaultConfig(clusterName string, client *goredis.Client) *Config {
	return &Config{
		RedisClient:    client,
		ClusterName:    clusterName,
		LockTTL:        5 * time.Second,
		MaxConcurrency: 200,
	}
}
