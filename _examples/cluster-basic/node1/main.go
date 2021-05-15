package main

import (
	"fmt"
	"github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/AsynkronIT/protoactor-go/cluster/automanaged"
	"github.com/AsynkronIT/protoactor-go/cluster/partition"
	"github.com/AsynkronIT/protoactor-go/remote"
	"time"
)

func main() {
	c := startNode()

	fmt.Print("\nBoot other nodes and press Enter\n")
	console.ReadLine()
	pid := c.Get("abc", "hello")
	fmt.Printf("Got pid %v", pid)
	fmt.Println()

	c.Shutdown(true)
}

func startNode() *cluster.Cluster {
	system := actor.NewActorSystem()

	provider := automanaged.NewWithConfig(2*time.Second, 6331, "localhost:6330", "localhost:6331")
	lookup := partition.New()
	config := remote.Configure("localhost", 0)
	clusterConfig := cluster.Configure("my-cluster", provider, lookup, config)
	c := cluster.New(system, clusterConfig)
	c.StartMember()

	return c
}
