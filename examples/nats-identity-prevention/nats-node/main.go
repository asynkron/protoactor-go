package main

import (
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
	natsidentity "github.com/awevoke/protoactor-go/cluster/identitylookup/nats"
	"github.com/awevoke/protoactor-go/cluster/identitylookup/storage"
	"github.com/awevoke/protoactor-go/remote"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// CounterGrain implements the Counter virtual actor.
type CounterGrain struct {
	count int32
}

func (g *CounterGrain) Init(ctx cluster.GrainContext) {
	ctx.Logger().Info("CounterGrain activated (nats-node)",
		slog.String("identity", ctx.Identity()),
	)
}

func (g *CounterGrain) Terminate(ctx cluster.GrainContext) {
	ctx.Logger().Info("CounterGrain deactivated (nats-node)",
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
	// Connect to NATS
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()
	fmt.Println("Connected to NATS")

	// Get JetStream context
	js, err := jetstream.New(nc)
	if err != nil {
		log.Fatalf("Failed to create JetStream context: %v", err)
	}

	// Create StorageLookup backed by NATS JetStream KV
	natsStorage, err := natsidentity.New("identity-example", js)
	if err != nil {
		log.Fatalf("Failed to create NATS identity storage: %v", err)
	}
	fmt.Println("NATS identity storage ready")

	// Create the IdentityLookup adapter
	identityLookup := storage.New(natsStorage)

	// Set up Proto.Actor cluster
	system := actor.NewActorSystem()

	provider, err := consul.New()
	if err != nil {
		log.Fatalf("Failed to create Consul provider: %v", err)
	}

	remoteConfig := remote.Configure("localhost", 0)

	counterKind := shared.NewCounterKind(func() shared.Counter {
		n := grainCount.Add(1)
		fmt.Printf("Creating CounterGrain instance #%d (nats-node)\n", n)
		return &CounterGrain{}
	}, 0)

	clusterConfig := cluster.Configure("identity-example", provider, identityLookup, remoteConfig,
		cluster.WithKinds(counterKind),
	)

	c := cluster.NewCluster(system, clusterConfig)
	if err := c.StartMember(); err != nil {
		log.Fatalf("Failed to start cluster member: %v", err)
	}

	fmt.Println("NATS-node started. Press Ctrl+C to stop.")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("Shutting down...")
	c.Shutdown(true)
}
