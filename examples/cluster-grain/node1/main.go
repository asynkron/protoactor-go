package main

import (
	"fmt"

	"cluster-grain/shared"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/consul"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/remote"
)

func main() {
	system := actor.NewActorSystem()

	provider, _ := consul.New()
	lookup := disthash.New()
	config := remote.Configure("localhost", 0)
	clusterConfig := cluster.Configure("my-cluster", provider, lookup, config)
	c := cluster.New(system, clusterConfig)
	c.StartMember()

	fmt.Print("\nBoot other nodes and press Enter\n")
	console.ReadLine()
	client := shared.GetHelloGrainClient(c, "mygrain1")
	res, err := client.SayHello(&shared.HelloRequest{
		Name: "World",
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Response: %v\n", res)
	fmt.Println()
	console.ReadLine()

	res, err = client.SayHello(&shared.HelloRequest{
		Name: "World",
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Response: %v\n", res)
	fmt.Println()

	console.ReadLine()
	c.Shutdown(true)
}
