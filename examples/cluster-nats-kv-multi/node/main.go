package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/natskv"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/nats-io/nats.go"
)

type HelloGrain struct{}

func (h *HelloGrain) Receive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		slog.Info("HelloGrain started")
	case *actor.Stopping:
		slog.Info("HelloGrain stopping")
	case string:
		slog.Info("HelloGrain received", slog.String("message", msg))
		ctx.Respond(fmt.Sprintf("Hello from %s!", msg))
	}
}

func main() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}
	host := os.Getenv("NODE_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	portStr := os.Getenv("NODE_PORT")
	port := 0
	if portStr != "" {
		port, _ = strconv.Atoi(portStr)
	}

	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	system := actor.NewActorSystem()

	provider, err := natskv.New(nc)
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	lookup := provider.IdentityLookup()
	remoteCfg := remote.Configure(host, port)

	helloKind := cluster.NewKind("HelloGrain", actor.PropsFromProducer(func() actor.Actor {
		return &HelloGrain{}
	}))

	clusterCfg := cluster.Configure("multi-example", provider, lookup, remoteCfg,
		cluster.WithKinds(helloKind),
	)
	c := cluster.NewCluster(system, clusterCfg)

	if err := c.StartMember(); err != nil {
		log.Fatalf("Failed to start: %v", err)
	}

	slog.Info("Node started", slog.String("host", host), slog.Int("port", port))

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	c.Shutdown(true)
}
