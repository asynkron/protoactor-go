package main

import (
	"fmt"
	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/AsynkronIT/protoactor-go/cluster/consul"
	"github.com/AsynkronIT/protoactor-go/cluster/partition"
	"github.com/AsynkronIT/protoactor-go/remote"
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
	lookup := partition.New()
	config := remote.Configure("localhost", 0)

	props := actor.PropsFromFunc(func(ctx actor.Context) {
		switch msg := ctx.Message().(type) {
		case *actor.Started:
			fmt.Printf("Started %v", msg)
			//case *shared.Noop:
			//	fmt.Printf("Hello %v\n", msg.Who)
		}
	})
	helloKind := cluster.NewKind("hello", props)
	clusterConfig := cluster.Configure("my-cluster", provider, lookup, config, helloKind)
	c := cluster.New(system, clusterConfig)

	c.StartMember()
	return c
}
