package main

import (
	"fmt"
	"log"

	"cluster-basic/shared"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/consul"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/remote"
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
