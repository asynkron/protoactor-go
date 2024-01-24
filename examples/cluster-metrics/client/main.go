package main

import (
	"fmt"
	"log"
	"time"

	"github.com/asynkron/protoactor-go/cluster/identitylookup/disthash"

	"cluster-metrics/shared"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/consul"
	"github.com/asynkron/protoactor-go/remote"
)

func main() {
	system := actor.NewActorSystem()
	config := remote.Configure("localhost", 0)

	provider, _ := consul.New()
	lookup := disthash.New()

	clusterConfig := cluster.Configure("my-cluster", provider, lookup, config)
	c := cluster.New(system, clusterConfig)
	setupLogger(c)
	c.StartMember()

	callopts := []cluster.GrainCallOption{
		cluster.WithTimeout(5 * time.Second),
		cluster.WithRetryCount(5),
	}
	doRequests(c, callopts...)
	doRequestsAsync(c, callopts...)
	console.ReadLine()
}

func doRequests(c *cluster.Cluster, callopts ...cluster.GrainCallOption) {
	msg := &shared.HelloRequest{Name: "Proto.Actor"}
	helloGrain := shared.GetHelloGrainClient(c, "MyGrain123")
	// with default callopts
	resp, err := helloGrain.SayHello(msg)
	if err != nil {
		log.Fatalf("SayHello failed. err:%v", err)
	}

	// with custom callopts
	resp, err = helloGrain.SayHello(msg, callopts...)
	if err != nil {
		log.Fatalf("SayHello failed. err:%v", err)
	}
	log.Printf("Message from SayHello: %v", resp.Message)
	for i := 0; i < 10000; i++ {
		grainId := fmt.Sprintf("hello%v", i)
		x := shared.GetHelloGrainClient(c, grainId)
		x.SayHello(&shared.HelloRequest{Name: grainId})
	}
	log.Println("Done")
}

func doRequestsAsync(c *cluster.Cluster, callopts ...cluster.GrainCallOption) {
	// sorry, golang has not magic, just use goroutine.
	go func() {
		doRequests(c, callopts...)
	}()
}

func setupLogger(c *cluster.Cluster) {
	system := c.ActorSystem
	// Subscribe
	system.EventStream.Subscribe(func(event interface{}) {
		switch msg := event.(type) {
		case *cluster.MemberJoinedEvent:
			log.Printf("Member Joined " + msg.Name())
		case *cluster.MemberLeftEvent:
			log.Printf("Member Left " + msg.Name())
		case *cluster.MemberRejoinedEvent:
			log.Printf("Member Rejoined " + msg.Name())
		case *cluster.MemberUnavailableEvent:
			log.Printf("Member Unavailable " + msg.Name())
		case *cluster.MemberAvailableEvent:
			log.Printf("Member Available " + msg.Name())
		case cluster.ClusterTopology:
			log.Printf("Cluster Topology Poll")
		}
	})
}
