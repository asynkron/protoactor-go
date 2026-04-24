package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"nats-identity-prevention/shared"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/cluster/clusterproviders/consul"
	"github.com/awevoke/protoactor-go/cluster/identitylookup/storage"
	"github.com/awevoke/protoactor-go/remote"

	pgidentity "github.com/awevoke/protoactor-go/cluster/identitylookup/postgres"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// CounterGrain implements the Counter virtual actor.
type CounterGrain struct {
	count int32
}

func (g *CounterGrain) Init(ctx cluster.GrainContext) {
	ctx.Logger().Info("CounterGrain activated (postgres-node)",
		slog.String("identity", ctx.Identity()),
	)
}

func (g *CounterGrain) Terminate(ctx cluster.GrainContext) {
	ctx.Logger().Info("CounterGrain deactivated (postgres-node)",
		slog.String("identity", ctx.Identity()),
		slog.Int("finalCount", int(g.count)),
	)
}

func (g *CounterGrain) ReceiveDefault(ctx cluster.GrainContext) {}

func (g *CounterGrain) Increment(req *shared.IncrementRequest, ctx cluster.GrainContext) (*shared.CountResponse, error) {
	g.count += req.Amount
	ctx.Logger().Info("Increment",
		slog.String("identity", ctx.Identity()),
		slog.Int("amount", int(req.Amount)),
		slog.Int("count", int(g.count)),
	)
	return &shared.CountResponse{Count: g.count}, nil
}

func (g *CounterGrain) GetCount(req *shared.GetCountRequest, ctx cluster.GrainContext) (*shared.CountResponse, error) {
	return &shared.CountResponse{Count: g.count}, nil
}

// grainCount tracks the number of grain instances created for logging.
var grainCount atomic.Int32

func main() {
	// Connect to Postgres
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = "postgres://test:test@localhost:5432/test?sslmode=disable"
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("Failed to open Postgres connection: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping Postgres: %v", err)
	}
	fmt.Println("Connected to Postgres")

	// Create StorageLookup backed by Postgres
	pgStorage := pgidentity.New("identity-example", db)
	if err := pgStorage.EnsureSchema(context.Background()); err != nil {
		log.Fatalf("Failed to ensure Postgres schema: %v", err)
	}
	fmt.Println("Postgres schema ready")

	// Create the IdentityLookup adapter
	identityLookup := storage.New(pgStorage)

	// Set up Proto.Actor cluster
	system := actor.NewActorSystem()

	provider, err := consul.New()
	if err != nil {
		log.Fatalf("Failed to create Consul provider: %v", err)
	}

	remoteConfig := remote.Configure("localhost", 0)

	counterKind := shared.NewCounterKind(func() shared.Counter {
		n := grainCount.Add(1)
		fmt.Printf("Creating CounterGrain instance #%d (postgres-node)\n", n)
		return &CounterGrain{}
	}, 0)

	clusterConfig := cluster.Configure("identity-example", provider, identityLookup, remoteConfig,
		cluster.WithKinds(counterKind),
	)

	c := cluster.NewCluster(system, clusterConfig)
	if err := c.StartMember(); err != nil {
		log.Fatalf("Failed to start cluster member: %v", err)
	}

	fmt.Println("Postgres-node started. Press Ctrl+C to stop.")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("Shutting down...")
	c.Shutdown(true)
}
