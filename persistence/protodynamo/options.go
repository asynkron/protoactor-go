package protodynamo

import (
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// Config holds all configuration for the DynamoDB persistence provider.
type Config struct {
	// Client is the DynamoDB client used for all operations. Required.
	Client *dynamodb.Client

	// EventsTable is the name of the DynamoDB table used to store events.
	// Default: "protoactor_events".
	EventsTable string

	// SnapshotsTable is the name of the DynamoDB table used to store snapshots.
	// Default: "protoactor_snapshots".
	SnapshotsTable string

	// SnapshotInterval controls how often snapshots are taken. A value of N
	// means a snapshot is persisted every N events. Default: 1.
	SnapshotInterval int
}

// Option is a functional option for configuring the DynamoDB provider.
type Option func(*Config)

// WithClient sets the DynamoDB client. This option is required.
func WithClient(client *dynamodb.Client) Option {
	return func(c *Config) {
		c.Client = client
	}
}

// WithEventsTable sets the name of the events table.
func WithEventsTable(name string) Option {
	return func(c *Config) {
		c.EventsTable = name
	}
}

// WithSnapshotsTable sets the name of the snapshots table.
func WithSnapshotsTable(name string) Option {
	return func(c *Config) {
		c.SnapshotsTable = name
	}
}

// WithSnapshotInterval sets the snapshot interval.
func WithSnapshotInterval(interval int) Option {
	return func(c *Config) {
		c.SnapshotInterval = interval
	}
}

func defaultConfig() *Config {
	return &Config{
		EventsTable:      "protoactor_events",
		SnapshotsTable:   "protoactor_snapshots",
		SnapshotInterval: 1,
	}
}
