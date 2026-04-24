package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

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

	helloKind := cluster.NewKind(shared.HelloKind, actor.PropsFromProducer(func() actor.Actor {
		return &shared.HelloActor{}
	}))

	lookup := provider.IdentityLookup()
	remoteCfg := remote.Configure("0.0.0.0", 0)
	clusterCfg := cluster.Configure("multi-example", provider, lookup, remoteCfg,
		cluster.WithKinds(helloKind))
	c := cluster.NewCluster(system, clusterCfg)

	if err := c.StartMember(); err != nil {
		log.Fatalf("Failed to start cluster member: %v", err)
	}

	fmt.Println("Cluster node started. Press Ctrl+C to stop.")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("Shutting down...")
	c.Shutdown(true)
}
