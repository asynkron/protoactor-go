package protopg

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/awevoke/protoactor-go/persistence"
)

// Compile-time check that PostgresProvider satisfies the persistence interfaces.
var _ persistence.ProviderState = (*PostgresProvider)(nil)

// PostgresProvider is a PostgreSQL-backed implementation of
// persistence.ProviderState. It stores events and snapshots as
// protobuf-encoded byte arrays.
type PostgresProvider struct {
	pool     *pgxpool.Pool
	config   *Config
	ownsPool bool // true when the provider created the pool itself
}

// New creates a new PostgresProvider with the given options. Either
// WithConnectionString or WithPool must be provided. If a connection string is
// provided, the provider creates and owns a pgxpool.Pool that will be closed
// when Close() is called.
func New(opts ...Option) (*PostgresProvider, error) {
	cfg := &Config{}
	for _, opt := range opts {
		opt(cfg)
	}
	applyDefaults(cfg)

	p := &PostgresProvider{
		config: cfg,
	}

	if cfg.Pool != nil {
		p.pool = cfg.Pool
		p.ownsPool = false
	} else if cfg.ConnectionString != "" {
		pool, err := pgxpool.New(context.Background(), cfg.ConnectionString)
		if err != nil {
			return nil, fmt.Errorf("protopg: failed to create connection pool: %w", err)
		}
		p.pool = pool
		p.ownsPool = true
	} else {
		return nil, errors.New("protopg: either WithConnectionString or WithPool must be provided")
	}

	return p, nil
}

// Close closes the underlying connection pool if it was created by the
// provider. If the pool was supplied externally via WithPool, Close is a
// no-op.
func (p *PostgresProvider) Close() {
	if p.ownsPool && p.pool != nil {
		p.pool.Close()
	}
}

// GetState returns the provider itself as a ProviderState so it can be used
// with the persistence.Provider interface.
func (p *PostgresProvider) GetState() persistence.ProviderState {
	return p
}

// Restart is a no-op for PostgreSQL because state persists across restarts.
func (p *PostgresProvider) Restart() {}

// GetSnapshotInterval returns the configured snapshot interval.
func (p *PostgresProvider) GetSnapshotInterval() int {
	return p.config.SnapshotInterval
}

// GetSnapshot retrieves the latest snapshot for the given actor. If no
// snapshot exists, ok is false.
func (p *PostgresProvider) GetSnapshot(actorName string) (snapshot any, eventIndex int, ok bool) {
	query := fmt.Sprintf(
		`SELECT data, data_type, snapshot_index FROM %s WHERE actor_name = $1 ORDER BY snapshot_index DESC LIMIT 1`,
		p.config.SnapshotsTable,
	)

	var data []byte
	var dataType string
	var snapshotIndex int

	err := p.pool.QueryRow(context.Background(), query, actorName).Scan(&data, &dataType, &snapshotIndex)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, 0, false
		}
		slog.Error("protopg: failed to get snapshot", "actor", actorName, "error", err)
		return nil, 0, false
	}

	msg, err := deserialize(data, dataType)
	if err != nil {
		slog.Error("protopg: failed to deserialize snapshot", "actor", actorName, "dataType", dataType, "error", err)
		return nil, 0, false
	}

	return msg, snapshotIndex, true
}

// PersistSnapshot stores a snapshot for the given actor. If a snapshot already
// exists at the given index, it is replaced.
func (p *PostgresProvider) PersistSnapshot(actorName string, snapshotIndex int, snapshot proto.Message) {
	data, dataType, err := serialize(snapshot)
	if err != nil {
		slog.Error("protopg: failed to serialize snapshot", "actor", actorName, "error", err)
		return
	}

	query := fmt.Sprintf(
		`INSERT INTO %s (actor_name, snapshot_index, data, data_type) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (actor_name, snapshot_index) DO UPDATE SET data = EXCLUDED.data, data_type = EXCLUDED.data_type`,
		p.config.SnapshotsTable,
	)

	if _, err := p.pool.Exec(context.Background(), query, actorName, snapshotIndex, data, dataType); err != nil {
		slog.Error("protopg: failed to persist snapshot", "actor", actorName, "snapshotIndex", snapshotIndex, "error", err)
	}
}

// DeleteSnapshots deletes all snapshots for the given actor with a
// snapshot_index less than or equal to inclusiveToIndex.
func (p *PostgresProvider) DeleteSnapshots(actorName string, inclusiveToIndex int) {
	query := fmt.Sprintf(
		`DELETE FROM %s WHERE actor_name = $1 AND snapshot_index <= $2`,
		p.config.SnapshotsTable,
	)

	if _, err := p.pool.Exec(context.Background(), query, actorName, inclusiveToIndex); err != nil {
		slog.Error("protopg: failed to delete snapshots", "actor", actorName, "toIndex", inclusiveToIndex, "error", err)
	}
}

// GetEvents retrieves events for the given actor within the specified index
// range. If eventIndexEnd is 0, all events from eventIndexStart onward are
// returned. Each event is passed to the callback in order.
func (p *PostgresProvider) GetEvents(actorName string, eventIndexStart int, eventIndexEnd int, callback func(e any)) {
	query := fmt.Sprintf(
		`SELECT data, data_type FROM %s WHERE actor_name = $1 AND event_index >= $2 AND ($3 = 0 OR event_index < $3) ORDER BY event_index ASC`,
		p.config.EventsTable,
	)

	rows, err := p.pool.Query(context.Background(), query, actorName, eventIndexStart, eventIndexEnd)
	if err != nil {
		slog.Error("protopg: failed to query events", "actor", actorName, "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var data []byte
		var dataType string

		if err := rows.Scan(&data, &dataType); err != nil {
			slog.Error("protopg: failed to scan event row", "actor", actorName, "error", err)
			return
		}

		msg, err := deserialize(data, dataType)
		if err != nil {
			slog.Error("protopg: failed to deserialize event", "actor", actorName, "dataType", dataType, "error", err)
			return
		}

		callback(msg)
	}

	if err := rows.Err(); err != nil {
		slog.Error("protopg: error iterating event rows", "actor", actorName, "error", err)
	}
}

// PersistEvent stores an event for the given actor at the specified index.
func (p *PostgresProvider) PersistEvent(actorName string, eventIndex int, event proto.Message) {
	data, dataType, err := serialize(event)
	if err != nil {
		slog.Error("protopg: failed to serialize event", "actor", actorName, "error", err)
		return
	}

	query := fmt.Sprintf(
		`INSERT INTO %s (actor_name, event_index, data, data_type) VALUES ($1, $2, $3, $4)`,
		p.config.EventsTable,
	)

	if _, err := p.pool.Exec(context.Background(), query, actorName, eventIndex, data, dataType); err != nil {
		slog.Error("protopg: failed to persist event", "actor", actorName, "eventIndex", eventIndex, "error", err)
	}
}

// DeleteEvents deletes all events for the given actor with an event_index
// less than or equal to inclusiveToIndex.
func (p *PostgresProvider) DeleteEvents(actorName string, inclusiveToIndex int) {
	query := fmt.Sprintf(
		`DELETE FROM %s WHERE actor_name = $1 AND event_index <= $2`,
		p.config.EventsTable,
	)

	if _, err := p.pool.Exec(context.Background(), query, actorName, inclusiveToIndex); err != nil {
		slog.Error("protopg: failed to delete events", "actor", actorName, "toIndex", inclusiveToIndex, "error", err)
	}
}

// serialize marshals a proto.Message into bytes and returns the fully-qualified
// message type name.
func serialize(msg proto.Message) ([]byte, string, error) {
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, "", fmt.Errorf("proto.Marshal: %w", err)
	}
	typeName := string(proto.MessageName(msg))
	if typeName == "" {
		return nil, "", errors.New("proto.MessageName returned empty string")
	}
	return data, typeName, nil
}

// deserialize creates a new proto.Message of the given type and unmarshals the
// data into it using the global protobuf type registry.
func deserialize(data []byte, dataType string) (proto.Message, error) {
	msgType, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(dataType))
	if err != nil {
		return nil, fmt.Errorf("protoregistry.GlobalTypes.FindMessageByName(%q): %w", dataType, err)
	}
	msg := msgType.New().Interface()
	if err := proto.Unmarshal(data, msg); err != nil {
		return nil, fmt.Errorf("proto.Unmarshal: %w", err)
	}
	return msg, nil
}
