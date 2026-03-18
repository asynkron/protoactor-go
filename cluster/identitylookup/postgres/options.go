package postgres

import (
	"database/sql"
	"log/slog"
	"time"
)

// Config holds the configuration for the Postgres identity storage backend.
type Config struct {
	// DB is the database/sql connection used for all Postgres operations.
	DB *sql.DB

	// ClusterName is the name of the Proto.Actor cluster. It is used as a
	// prefix for the identity table name to allow multiple clusters to share
	// the same Postgres database.
	ClusterName string

	// LockTTL is the maximum time a spawn lock is held before it is considered
	// expired. This prevents stale locks if a node crashes during activation.
	// Defaults to 5 seconds.
	LockTTL time.Duration

	// MaxConcurrency limits the number of concurrent Postgres operations.
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

// WithMaxConcurrency sets the maximum number of concurrent Postgres operations.
func WithMaxConcurrency(n int) Option {
	return func(c *Config) {
		c.MaxConcurrency = n
	}
}

// WithLogger sets a custom logger for the Postgres identity storage.
func WithLogger(logger *slog.Logger) Option {
	return func(c *Config) {
		c.Logger = logger
	}
}

func defaultConfig(clusterName string, db *sql.DB) *Config {
	return &Config{
		DB:             db,
		ClusterName:    clusterName,
		LockTTL:        5 * time.Second,
		MaxConcurrency: 200,
	}
}
