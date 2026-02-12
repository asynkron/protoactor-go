package main

import (
	"cluster-gossip/shared"
	"fmt"
	"log"
	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/consul"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/lmittmann/tint"
	"log/slog"
	"os"
	"time"
)

func main() {
	c := startNode()

	fmt.Printf("Cluster %v\n", c.ActorSystem.ID)

	//example on how to work with maps inside the gossip
	c.Gossip.SetMapState("someGossipEntry", "abc", &shared.StringValue{Value: "Hello World"})
	c.Gossip.SetMapState("someGossipEntry", "def", &shared.StringValue{Value: "Lorem Ipsum"})
	keys := c.Gossip.GetMapKeys("someGossipEntry")
	fmt.Printf("Keys %v\n", keys)
	for _, key := range keys {
		value := c.Gossip.GetMapState("someGossipEntry", key)
		v := &shared.StringValue{}
		_ = value.UnmarshalTo(v)
		fmt.Printf("Key %v Value %v\n", key, v.Value)
	}

	fmt.Print("\nBoot other nodes and press Enter\n")
	_, _ = console.ReadLine()
	pid := c.Get("abc", "hello")
	fmt.Printf("Got pid %v\n", pid)
	res, _ := c.Request("abc", "hello", &shared.HelloRequest{Name: "Roger"})
	fmt.Printf("Got response %v\n", res)

	fmt.Println()
	_, _ = console.ReadLine()
	c.Shutdown(true)
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
			if msg.Key != "heartbeat" {
				return
			}

			heartbeat := &cluster.MemberHeartbeat{}

			fmt.Printf("Member %v\n", msg.MemberID)
			fmt.Printf("Sequence Number %v\n", msg.SeqNumber)

			unpackErr := msg.Value.UnmarshalTo(heartbeat)
			if unpackErr != nil {
				fmt.Printf("Unpack error %v\n", unpackErr)
			} else {
				//loop over as.ActorCount map
				for k, v := range heartbeat.ActorStatistics.ActorCount {
					fmt.Printf("ActorCount %v %v\n", k, v)
				}
			}
		}
	})

	provider, _ := consul.New()
	lookup := disthash.New()
	config := remote.Configure("localhost", 0)
	clusterConfig := cluster.Configure("my-cluster", provider, lookup, config)
	c := cluster.New(system, clusterConfig)
	if err := c.StartMember(); err != nil {
		log.Fatal(err)
	}

	return c
}
