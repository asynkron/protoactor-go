package main

import (
	fmt "fmt"

	actor "github.com/asynkron/protoactor-go/actor"
	cluster "github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/test"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/remote"
)

func main() {
	system := actor.NewActorSystem()
	provider := test.NewTestProvider(test.NewInMemAgent())
	lookup := disthash.New()
	config := remote.Configure("localhost", 0)
	helloKind := NewHelloKind(NewHelloGrain, 0)
	clusterConfig := cluster.Configure("core", provider, lookup, config, cluster.WithKinds(
		helloKind))
	cluster := cluster.New(system, clusterConfig)
	cluster.StartMember()

	client := GetHelloGrainClient(cluster, "1")
	resp, err := client.InvokeService(&InvokeServiceRequest{Name: "Alice"})
	if err != nil {
		panic(err)
	}
	fmt.Printf("%v\n", resp)
}
