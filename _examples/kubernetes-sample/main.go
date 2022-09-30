package main

import (
	"context"
	"fmt"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/k8s"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/remote"
	"k8s.io/utils/env"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log.Printf("Starting node\n")

	c := startNode()
	defer c.Shutdown(true)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	sendMessages(ctx, c)

	log.Printf("Shutting down\n")
}

func sendMessages(ctx context.Context, c *cluster.Cluster) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
			if _, err := c.Call(
				"some-actor-123",
				"helloKind",
				&HelloRequest{Name: fmt.Sprintf("Hello from %s", c.ActorSystem.ID)}); err != nil {
				log.Printf("Error calling actor: %v\n", err)
			} else {
				log.Printf("Successfully called actor\n")
			}
		}
	}
}

func helloGrainReceive(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case *HelloRequest:
		log.Printf("Got hello %v\n", msg)
		ctx.Respond(&HelloResponse{})
	}
}

func startNode() *cluster.Cluster {
	host, port, advertisedHost := getHostInformation()

	system := actor.NewActorSystem()
	provider, err := k8s.New()
	if err != nil {
		log.Panic(err)
	}
	lookup := disthash.New()

	config := remote.Configure(host, port, remote.WithAdvertisedHost(advertisedHost))

	props := actor.PropsFromFunc(helloGrainReceive)
	helloKind := cluster.NewKind("helloKind", props)

	clusterConfig := cluster.Configure("my-cluster", provider, lookup, config, cluster.WithKinds(helloKind))

	c := cluster.New(system, clusterConfig)
	c.StartMember()

	return c
}

func getHostInformation() (host string, port int, advertisedHost string) {
	host = env.GetString("PROTOHOST", "127.0.0.1")
	port, err := env.GetInt("PROTOPORT", 0)
	if err != nil {
		log.Panic(err)
	}
	advertisedHost = env.GetString("PROTOADVERTISEDHOST", "")

	log.Printf("host: %s, port: %d, advertisedHost: %s", host, port, advertisedHost)

	return
}
