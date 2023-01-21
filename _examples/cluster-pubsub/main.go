package main

import (
	"strconv"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/test"
	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"
	"github.com/asynkron/protoactor-go/remote"
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

	cluster := cluster.New(system, clusterConfig)

	cluster.StartMember()

	return cluster
}
