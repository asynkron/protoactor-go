//go:build integration

package postgres_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/identitylookup"
	pgidentity "github.com/asynkron/protoactor-go/cluster/identitylookup/postgres"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	dsn := os.Getenv("POSTGRES_DSN")

	if dsn == "" {
		ctx := context.Background()
		container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        "postgres:16-alpine",
				ExposedPorts: []string{"5432/tcp"},
				Env: map[string]string{
					"POSTGRES_USER":     "test",
					"POSTGRES_PASSWORD": "test",
					"POSTGRES_DB":       "testdb",
				},
				WaitingFor: wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).
					WithStartupTimeout(60 * time.Second),
			},
			Started: true,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to start postgres container: %v\n", err)
			os.Exit(1)
		}

		host, err := container.Host(ctx)
		if err != nil {
			_ = container.Terminate(ctx)
			fmt.Fprintf(os.Stderr, "failed to get container host: %v\n", err)
			os.Exit(1)
		}

		port, err := container.MappedPort(ctx, "5432")
		if err != nil {
			_ = container.Terminate(ctx)
			fmt.Fprintf(os.Stderr, "failed to get container port: %v\n", err)
			os.Exit(1)
		}

		dsn = fmt.Sprintf("postgres://test:test@%s:%s/testdb?sslmode=disable", host, port.Port())

		defer func() {
			_ = container.Terminate(ctx)
		}()
	}

	var err error
	testDB, err = sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open database: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// TestPostgresConformance runs the full StorageLookup conformance suite against
// a real PostgreSQL instance started via testcontainers.
//
// Run with: go test -tags integration -v ./...
func TestPostgresConformance(t *testing.T) {
	ctx := context.Background()

	suite := &identitylookup.StorageConformanceSuite{
		NewStorage: func() cluster.StorageLookup {
			storage := pgidentity.New("test_cluster", testDB)
			if err := storage.EnsureSchema(ctx); err != nil {
				t.Fatalf("EnsureSchema failed: %v", err)
			}
			return storage
		},
		Cleanup: func() {
			// Clean up all test rows after each test.
			_, _ = testDB.ExecContext(ctx, "DELETE FROM test_cluster_identities")
		},
	}

	suite.RunAll(t)
}

// TestPostgresEnumeratorConformance runs the StorageGrainEnumerator conformance
// suite against a real PostgreSQL instance started via testcontainers.
func TestPostgresEnumeratorConformance(t *testing.T) {
	ctx := context.Background()

	suite := &identitylookup.EnumeratorConformanceSuite{
		NewStorage: func() identitylookup.EnumerableStorage {
			storage := pgidentity.New("test_cluster", testDB)
			if err := storage.EnsureSchema(ctx); err != nil {
				t.Fatalf("EnsureSchema failed: %v", err)
			}
			return storage
		},
		Cleanup: func() {
			_, _ = testDB.ExecContext(ctx, "DELETE FROM test_cluster_identities")
		},
	}

	suite.RunAll(t)
}
