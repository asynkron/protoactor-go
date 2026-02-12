package main

import (
	"fmt"
	"github.com/lmittmann/tint"
	"log"
	"os"
	"time"

	"cluster-gossip/shared"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/consul"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/remote"
	"log/slog"
)

func main() {
	cluster := startNode()

	fmt.Print("\nBoot other nodes and press Enter\n")
	console.ReadLine()

	cluster.Shutdown(true)
}

func coloredConsoleLogging(system *actor.ActorSystem) *slog.Logger {
	return slog.New(tint.NewHandler(os.Stdout, &tint.Options{
		Level:      slog.LevelError,
		TimeFormat: time.RFC3339,
		AddSource:  true,
	})).With("lib", "Proto.Actor").
		With("system", system.ID)
}

func startNode() *cluster.Cluster {
	system := actor.NewActorSystem(actor.WithLoggerFactory(coloredConsoleLogging))
	system.EventStream.Subscribe(func(evt interface{}) {
		switch msg := evt.(type) {

		//subscribe to Cluster Topology changes
		case *cluster.ClusterTopology:
			fmt.Printf("\nClusterTopology %v\n\n", msg)

		//subscribe to Gossip updates, specifically MemberHeartbeat
		case *cluster.GossipUpdate:
			if msg.Key != "someGossipEntry" {
				return
			}

			fmt.Printf("GossipUpdate %v\n", msg)
		}
	})

	provider, _ := consul.New()
	lookup := disthash.New()
	config := remote.Configure("localhost", 0)

	props := actor.PropsFromFunc(func(ctx actor.Context) {
		switch msg := ctx.Message().(type) {
		case *actor.Started:
			fmt.Printf("Started %v", msg)
		case *shared.HelloRequest:
			fmt.Printf("Hello %v\n", msg.Name)
			ctx.Respond(&shared.HelloResponse{})
		}
	})
	helloKind := cluster.NewKind("hello", props)
	clusterConfig := cluster.Configure("my-cluster", provider, lookup, config, cluster.WithKinds(helloKind))
	c := cluster.New(system, clusterConfig)

	if err := c.StartMember(); err != nil {
		log.Fatal(err)
	}
	return c
}
