//go:build integration

package redis_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/cluster/identitylookup"
	redisidentity "github.com/awevoke/protoactor-go/cluster/identitylookup/redis"
)

var testClient *goredis.Client

func TestMain(m *testing.M) {
	addr := os.Getenv("REDIS_ADDR")

	if addr == "" {
		ctx := context.Background()
		container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        "redis:7-alpine",
				ExposedPorts: []string{"6379/tcp"},
				WaitingFor:   wait.ForLog("Ready to accept connections").WithStartupTimeout(30 * time.Second),
			},
			Started: true,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to start redis container: %v\n", err)
			os.Exit(1)
		}

		host, err := container.Host(ctx)
		if err != nil {
			_ = container.Terminate(ctx)
			fmt.Fprintf(os.Stderr, "failed to get container host: %v\n", err)
			os.Exit(1)
		}

		port, err := container.MappedPort(ctx, "6379")
		if err != nil {
			_ = container.Terminate(ctx)
			fmt.Fprintf(os.Stderr, "failed to get container port: %v\n", err)
			os.Exit(1)
		}

		addr = fmt.Sprintf("%s:%s", host, port.Port())

		defer func() {
			_ = container.Terminate(ctx)
		}()
	}

	testClient = goredis.NewClient(&goredis.Options{
		Addr: addr,
	})

	os.Exit(m.Run())
}

// TestRedisConformance runs the full StorageLookup conformance suite against
// a real Redis instance started via testcontainers.
//
// Run with: go test -tags integration -v ./...
func TestRedisConformance(t *testing.T) {
	suite := &identitylookup.StorageConformanceSuite{
		NewStorage: func() cluster.StorageLookup {
			return redisidentity.New("test-cluster", testClient)
		},
		Cleanup: func() {
			// Clean up all test keys after each test.
			ctx := context.Background()
			iter := testClient.Scan(ctx, 0, "test-cluster:*", 0).Iterator()
			for iter.Next(ctx) {
				testClient.Del(ctx, iter.Val())
			}
		},
	}

	suite.RunAll(t)
}

// TestRedisEnumeratorConformance runs the StorageGrainEnumerator conformance
// suite against a real Redis instance started via testcontainers.
func TestRedisEnumeratorConformance(t *testing.T) {
	counter := 0
	suite := &identitylookup.EnumeratorConformanceSuite{
		NewStorage: func() identitylookup.EnumerableStorage {
			counter++
			return redisidentity.New(fmt.Sprintf("test-enum-%d", counter), testClient)
		},
		Cleanup: func() {
			// Clean up all test keys after each test.
			ctx := context.Background()
			iter := testClient.Scan(ctx, 0, "test-enum-*", 0).Iterator()
			for iter.Next(ctx) {
				testClient.Del(ctx, iter.Val())
			}
		},
	}

	suite.RunAll(t)
}
