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
)

type HelloGrain struct {
	cluster.Grain
}

func (h HelloGrain) Terminate() {
}

func (h HelloGrain) ReceiveDefault(ctx actor.Context) {
}

func (h HelloGrain) SayHello(request *shared.HelloRequest, context cluster.GrainContext) (*shared.HelloResponse, error) {
	return &shared.HelloResponse{Message: "Hello " + request.Name}, nil
}

func main() {

	shared.HelloFactory(func() shared.Hello { return &HelloGrain{} })

	cluster := startNode()
	fmt.Print("\nBoot other nodes and press Enter\n")
	console.ReadLine()
	cluster.Shutdown(true)
}

func startNode() *cluster.Cluster {
	system := actor.NewActorSystem()
	provider, _ := consul.New()
	lookup := disthash.New()
	remoteConfig := remote.Configure("localhost", 0)
	helloKind := shared.GetHelloKind()
	clusterConfig := cluster.Configure("my-cluster", provider, lookup, remoteConfig, cluster.WithKinds(helloKind))
	c := cluster.New(system, clusterConfig)
	c.StartMember()
	return c
}
