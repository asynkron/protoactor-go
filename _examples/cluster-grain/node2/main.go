package main

import (
	"cluster-grain/shared"
	"fmt"
	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/consul"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/remote"
	"time"
)

func main() {
	cluster := startNode()

	fmt.Print("\nBoot other nodes and press Enter\n")
	console.ReadLine()

	cluster.Shutdown(true)
}

func startNode() *cluster.Cluster {
	system := actor.NewActorSystem()

	provider, _ := consul.New()
	lookup := disthash.New()
	config := remote.Configure("localhost", 0)

	kind := cluster.NewKind("Hello", actor.PropsFromProducer(func() actor.Actor {
		return &shared.HelloActor{
			Timeout: 60 * time.Second,
		}
	}))

	clusterConfig := cluster.Configure("my-cluster", provider, lookup, config, cluster.WithKind(kind))
	c := cluster.New(system, clusterConfig)

	c.StartMember()
	return c
}
