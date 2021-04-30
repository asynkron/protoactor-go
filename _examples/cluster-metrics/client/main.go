package main

import (
	"fmt"
	"log"
	"time"

	"cluster-metrics/shared"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/AsynkronIT/protoactor-go/cluster/consul"
	"github.com/AsynkronIT/protoactor-go/remote"
)

// Logger is message middleware which logs messages before continuing to the next middleware
func Logger(next actor.ReceiverFunc) actor.ReceiverFunc {
	fn := func(context actor.ReceiverContext, env *actor.MessageEnvelope) {
		switch env.Message.(type) {
		case *actor.Started:
			log.Printf("actor started " + context.Self().String())
		case *actor.Stopped:
			log.Printf("actor stopped " + context.Self().String())
		}
		next(context, env)
	}
	return fn
}

func main() {
	system := actor.NewActorSystem()
	config := remote.Configure("localhost", 0)

	provider, _ := consul.New()
	clusterConfig := cluster.Configure("my-cluster", provider, config)
	c := cluster.New(system, clusterConfig)
	setupLogger(c)
	c.Start()

	callopts := cluster.NewGrainCallOptions(c).WithTimeout(5 * time.Second).WithRetry(5)
	doRequests(c, callopts)
	doRequestsAsync(c, callopts)
	console.ReadLine()
}

func doRequests(c *cluster.Cluster, callopts *cluster.GrainCallOptions) {
	msg := &shared.HelloRequest{Name: "GAM"}
	helloGrain := shared.GetHelloGrainClient(c, "abc")
	// with default callopts
	resp, err := helloGrain.SayHello(msg)
	if err != nil {
		log.Fatalf("SayHello failed. err:%v", err)
	}

	// with custom callopts
	resp, err = helloGrain.SayHello(msg, callopts)
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

func doRequestsAsync(c *cluster.Cluster, callopts *cluster.GrainCallOptions) {
	// sorry, golang has not magic, just use goroutine.
	go func() {
		doRequests(c, callopts)
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
		case cluster.TopologyEvent:
			log.Printf("Cluster Topology Poll")
		}
	})
}
