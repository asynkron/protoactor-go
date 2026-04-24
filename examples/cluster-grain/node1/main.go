package main

import (
	"fmt"
	"log"

	"cluster-grain/shared"

	console "github.com/asynkron/goconsole"
	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/cluster/clusterproviders/consul"
	"github.com/awevoke/protoactor-go/cluster/identitylookup/disthash"
	"github.com/awevoke/protoactor-go/remote"
)

func main() {
	system := actor.NewActorSystem()

	provider, _ := consul.New()
	lookup := disthash.New()
	config := remote.Configure("localhost", 0)
	clusterConfig := cluster.Configure("my-cluster", provider, lookup, config)
	c := cluster.NewCluster(system, clusterConfig)
	if err := c.StartMember(); err != nil {
		log.Fatal(err)
	}

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
