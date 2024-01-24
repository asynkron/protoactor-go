package main

import (
	"fmt"

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
	clusterConfig := cluster.Configure("test", provider, lookup, config, cluster.WithKinds(
		helloKind))
	cst := cluster.New(system, clusterConfig)
	cst.StartMember()

	client := GetHelloGrainClient(cst, "test")
	_, err := client.Hello(&HelloRequest{Name: "user-not-found"})
	if err != nil {
		if IsUserNotFound(err) {
			fmt.Println("user not found")
		} else {
			fmt.Printf("unknown error: %v\n", err)
		}
	}

	_, err = client.Hello(&HelloRequest{Name: "normal-error"})
	if err != nil {
		fmt.Printf("unknown error: %v\n", err)
	}
}
