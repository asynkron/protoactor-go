package main

import (
	"log"
	"strconv"

	console "github.com/asynkron/goconsole"
	"github.com/awevoke/protoactor-go/actor"
	"github.com/awevoke/protoactor-go/cluster"
	"github.com/awevoke/protoactor-go/cluster/clusterproviders/test"
	"github.com/awevoke/protoactor-go/cluster/identitylookup/disthash"
	"github.com/awevoke/protoactor-go/remote"
)

func main() {
	c := startNode()

	for i := 0; i < 3; i++ {
		GetUserActorGrainClient(c, "user"+strconv.Itoa(i)).Connect(&Empty{})
	}

	console.ReadLine()
	c.Shutdown(true)
}

func startNode() *cluster.Cluster {
	// how long before the grain poisons itself
	system := actor.NewActorSystem()

	provider := test.NewTestProvider(test.NewInMemAgent())
	lookup := disthash.New()
	config := remote.Configure("localhost", 0)

	userKind := NewUserActorKind(func() UserActor {
		return &User{}
	}, 0)

	clusterConfig := cluster.Configure("my-cluster", provider, lookup, config,
		cluster.WithKinds(userKind))

	cluster := cluster.NewCluster(system, clusterConfig)

	if err := cluster.StartMember(); err != nil {
		log.Fatal(err)
	}

	return cluster
}
