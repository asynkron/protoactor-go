package protopg

import "github.com/jackc/pgx/v5/pgxpool"

// Config holds configuration for the PostgreSQL persistence provider.
type Config struct {
	// ConnectionString is a PostgreSQL connection URI (e.g.
	// "postgres://user:pass@localhost:5432/mydb?sslmode=disable").
	// Either ConnectionString or Pool must be provided.
	ConnectionString string

	// Pool allows providing a pre-configured pgxpool.Pool. When set,
	// ConnectionString is ignored and the provider will NOT close the
	// pool on Close().
	Pool *pgxpool.Pool

	// EventsTable is the name of the table used to store events.
	// Defaults to "events".
	EventsTable string

	// SnapshotsTable is the name of the table used to store snapshots.
	// Defaults to "snapshots".
	SnapshotsTable string

	// SnapshotInterval controls how often snapshots are taken.
	// Defaults to 1.
	SnapshotInterval int
}

// Option is a functional option for configuring the PostgreSQL provider.
type Option func(*Config)

// WithConnectionString sets the PostgreSQL connection string.
func WithConnectionString(connStr string) Option {
	return func(c *Config) {
		c.ConnectionString = connStr
	}
}

// WithPool provides a pre-configured connection pool. When set, the provider
// will not create its own pool from ConnectionString and will not close the
// pool when Close() is called.
func WithPool(pool *pgxpool.Pool) Option {
	return func(c *Config) {
		c.Pool = pool
	}
}

// WithSnapshotInterval sets the snapshot interval returned by
// GetSnapshotInterval.
func WithSnapshotInterval(interval int) Option {
	return func(c *Config) {
		c.SnapshotInterval = interval
	}
}

// WithEventsTable overrides the default events table name ("events").
func WithEventsTable(table string) Option {
	return func(c *Config) {
		c.EventsTable = table
	}
}

// WithSnapshotsTable overrides the default snapshots table name ("snapshots").
func WithSnapshotsTable(table string) Option {
	return func(c *Config) {
		c.SnapshotsTable = table
	}
}

func applyDefaults(c *Config) {
	if c.EventsTable == "" {
		c.EventsTable = "events"
	}
	if c.SnapshotsTable == "" {
		c.SnapshotsTable = "snapshots"
	}
	if c.SnapshotInterval == 0 {
		c.SnapshotInterval = 1
	}
}
