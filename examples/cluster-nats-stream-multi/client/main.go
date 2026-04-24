package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/cluster/clusterproviders/natsstream"
	"github.com/awevoke/protoactor-go/examples/cluster-nats-stream-multi/shared"
	"github.com/awevoke/protoactor-go/remote"
	"github.com/nats-io/nats.go"
)

func main() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}

	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	system := actor.NewActorSystem()

	provider, err := natsstream.New(nc)
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	lookup := provider.IdentityLookup()
	remoteCfg := remote.Configure("0.0.0.0", 0)
	clusterCfg := cluster.Configure("multi-example", provider, lookup, remoteCfg)
	c := cluster.NewCluster(system, clusterCfg)

	if err := c.StartClient(); err != nil {
		log.Fatalf("Failed to start cluster client: %v", err)
	}

	fmt.Println("Cluster client started.")

	// Give time for topology to settle.
	time.Sleep(3 * time.Second)

	// Resolve grains on the cluster. The HelloActor kind must be registered
	// on at least one cluster member node for this to succeed.
	for i := 0; i < 5; i++ {
		identity := fmt.Sprintf("grain-%d", i)
		pid := c.Get(identity, shared.HelloKind)
		if pid == nil {
			slog.Warn("Failed to resolve grain", slog.String("identity", identity))
			continue
		}
		slog.Info("Resolved grain",
			slog.String("identity", identity),
			slog.String("pid", pid.String()))
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("Shutting down...")
	c.Shutdown(true)
}
