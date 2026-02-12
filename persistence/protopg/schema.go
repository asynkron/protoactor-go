package protopg

import (
	"context"
	"fmt"
)

// CreateSchemaIfNotExists creates the events and snapshots tables along with
// the required indexes if they do not already exist. The table names are taken
// from the provider's configuration.
func (p *PostgresProvider) CreateSchemaIfNotExists(ctx context.Context) error {
	eventsSQL := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
    actor_name  TEXT        NOT NULL,
    event_index BIGINT      NOT NULL,
    data        BYTEA       NOT NULL,
    data_type   TEXT        NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (actor_name, event_index)
)`, p.config.EventsTable)

	snapshotsSQL := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
    actor_name     TEXT        NOT NULL,
    snapshot_index BIGINT      NOT NULL,
    data           BYTEA       NOT NULL,
    data_type      TEXT        NOT NULL,
    created_at     TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (actor_name, snapshot_index)
)`, p.config.SnapshotsTable)

	indexSQL := fmt.Sprintf(
		`CREATE INDEX IF NOT EXISTS idx_%s_actor_latest ON %s(actor_name, snapshot_index DESC)`,
		p.config.SnapshotsTable, p.config.SnapshotsTable,
	)

	for _, stmt := range []string{eventsSQL, snapshotsSQL, indexSQL} {
		if _, err := p.pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("protopg: schema creation failed: %w", err)
		}
	}
	return nil
}
