package main

import (
	"fmt"
	"github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/AsynkronIT/protoactor-go/cluster/consul"
	"github.com/AsynkronIT/protoactor-go/cluster/partition"
	"github.com/AsynkronIT/protoactor-go/remote"
)

func main() {
	c := startNode()

	fmt.Print("\nBoot other nodes and press Enter\n")
	console.ReadLine()
	pid := c.Get("abc", "hello")
	fmt.Printf("Got pid %v", pid)
	fmt.Println()
	console.ReadLine()
	c.Shutdown(true)
}

func startNode() *cluster.Cluster {
	system := actor.NewActorSystem()

	provider, _ := consul.New()
	lookup := partition.New()
	config := remote.Configure("localhost", 0)
	clusterConfig := cluster.Configure("my-cluster", provider, lookup, config)
	c := cluster.New(system, clusterConfig)
	c.StartMember()

	return c
}
