package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/cluster/clusterproviders/natskv"
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

	provider, err := natskv.New(nc)
	if err != nil {
		log.Fatalf("Failed to create NATS KV provider: %v", err)
	}

	lookup := provider.IdentityLookup()
	remoteCfg := remote.Configure("127.0.0.1", 0)
	clusterCfg := cluster.Configure("example-cluster", provider, lookup, remoteCfg)
	c := cluster.NewCluster(system, clusterCfg)

	if err := c.StartMember(); err != nil {
		log.Fatalf("Failed to start cluster member: %v", err)
	}

	fmt.Println("Cluster member started. Press Ctrl+C to stop.")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("Shutting down...")
	c.Shutdown(true)
}
