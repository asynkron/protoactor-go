package nats

import (
	"log/slog"
	"time"
)

// Config holds the configuration for the NATS JetStream identity storage backend.
type Config struct {
	// ClusterName is the name of the Proto.Actor cluster. It is used as a
	// prefix for NATS KV bucket names to allow multiple clusters to share
	// the same NATS JetStream instance.
	ClusterName string

	// LockTTL is the maximum time a spawn lock is held before it is
	// considered stale. This is used as the timeout for WaitForActivation.
	// Defaults to 5 seconds.
	LockTTL time.Duration

	// MaxConcurrency limits the number of concurrent NATS operations.
	// Defaults to 200.
	MaxConcurrency int

	// Logger is an optional structured logger. If nil, slog.Default() is used.
	Logger *slog.Logger
}

// Option is a functional option for configuring Config.
type Option func(*Config)

// WithLockTTL sets the lock time-to-live duration.
func WithLockTTL(ttl time.Duration) Option {
	return func(c *Config) {
		c.LockTTL = ttl
	}
}

// WithMaxConcurrency sets the maximum number of concurrent NATS operations.
func WithMaxConcurrency(n int) Option {
	return func(c *Config) {
		c.MaxConcurrency = n
	}
}

// WithLogger sets a custom logger for the NATS identity storage.
func WithLogger(logger *slog.Logger) Option {
	return func(c *Config) {
		c.Logger = logger
	}
}

func defaultConfig(clusterName string) *Config {
	return &Config{
		ClusterName:    clusterName,
		LockTTL:        5 * time.Second,
		MaxConcurrency: 200,
	}
}
