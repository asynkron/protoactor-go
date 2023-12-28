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
	cst := cluster.New(system, clusterConfig)
	cst.StartMember()
	// self call: request -> 1 -> 1
	client := GetHelloGrainClient(cst, "1")
	resp, err := client.InvokeService(&InvokeServiceRequest{Name: "Alice"})
	if err != nil {
		panic(err)
	}
	fmt.Printf("%v\n", resp)

	// reenter call: request -> 2 -> 1 -> 2
	client = GetHelloGrainClient(cst, "2")
	resp, err = client.InvokeService(&InvokeServiceRequest{Name: "Alice"})
	if err != nil {
		panic(err)
	}
	fmt.Printf("%v\n", resp)
}
