//go:build integration

package redis_test

import (
	"os"
	"testing"

	goredis "github.com/redis/go-redis/v9"

	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/identitylookup"
	redisidentity "github.com/asynkron/protoactor-go/cluster/identitylookup/redis"
)

// TestRedisConformance runs the full StorageLookup conformance suite against
// a real Redis instance. Requires a running Redis at REDIS_ADDR (default localhost:6379).
//
// Run with: go test -tags integration -v ./...
func TestRedisConformance(t *testing.T) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	client := goredis.NewClient(&goredis.Options{
		Addr: addr,
	})

	suite := &identitylookup.StorageConformanceSuite{
		NewStorage: func() cluster.StorageLookup {
			return redisidentity.New("test-cluster", client)
		},
		Cleanup: func() {
			// Clean up all test keys after each test.
			ctx := client.Context()
			iter := client.Scan(ctx, 0, "test-cluster:*", 0).Iterator()
			for iter.Next(ctx) {
				client.Del(ctx, iter.Val())
			}
		},
	}

	suite.RunAll(t)
}
