//go:build integration

package nats_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/identitylookup"
	natsidentity "github.com/asynkron/protoactor-go/cluster/identitylookup/nats"
)

var testJS jetstream.JetStream

const testCluster = "test_cluster"

func TestMain(m *testing.M) {
	natsURL := os.Getenv("NATS_URL")

	if natsURL == "" {
		ctx := context.Background()
		container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        "nats:2-alpine",
				ExposedPorts: []string{"4222/tcp"},
				Cmd:          []string{"-js"},
				WaitingFor:   wait.ForLog("Server is ready").WithStartupTimeout(30 * time.Second),
			},
			Started: true,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to start nats container: %v\n", err)
			os.Exit(1)
		}

		host, err := container.Host(ctx)
		if err != nil {
			_ = container.Terminate(ctx)
			fmt.Fprintf(os.Stderr, "failed to get container host: %v\n", err)
			os.Exit(1)
		}

		port, err := container.MappedPort(ctx, "4222")
		if err != nil {
			_ = container.Terminate(ctx)
			fmt.Fprintf(os.Stderr, "failed to get container port: %v\n", err)
			os.Exit(1)
		}

		natsURL = fmt.Sprintf("nats://%s:%s", host, port.Port())

		defer func() {
			_ = container.Terminate(ctx)
		}()
	}

	nc, err := nats.Connect(natsURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to nats: %v\n", err)
		os.Exit(1)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create jetstream context: %v\n", err)
		os.Exit(1)
	}

	testJS = js

	os.Exit(m.Run())
}

// TestNatsConformance runs the full StorageLookup conformance suite against
// a real NATS JetStream instance started via testcontainers.
//
// Run with: go test -tags integration -v -count=1 ./...
func TestNatsConformance(t *testing.T) {
	ctx := context.Background()

	suite := &identitylookup.StorageConformanceSuite{
		NewStorage: func() cluster.StorageLookup {
			storage, err := natsidentity.New(testCluster, testJS)
			if err != nil {
				t.Fatalf("New failed: %v", err)
			}
			return storage
		},
		Cleanup: func() {
			// Delete and recreate both KV buckets between tests.
			_ = testJS.DeleteKeyValue(ctx, testCluster+"_identities")
			_ = testJS.DeleteKeyValue(ctx, testCluster+"_members")
		},
	}

	suite.RunAll(t)
}
