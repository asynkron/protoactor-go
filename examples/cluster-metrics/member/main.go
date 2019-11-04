package main

import (
	"fmt"
	"log"
	"time"

	console "github.com/AsynkronIT/goconsole"
	"github.com/otherview/protoactor-go/actor"
	"github.com/otherview/protoactor-go/cluster"
	"github.com/otherview/protoactor-go/cluster/consul"
	"github.com/otherview/protoactor-go/eventstream"
	"github.com/otherview/protoactor-go/examples/cluster/shared"
	"github.com/otherview/protoactor-go/remote"
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
	// this node knows about Hello kind
	remote.Register("Hello", actor.PropsFromProducer(func() actor.Actor {
		return &shared.HelloActor{}
	}).WithReceiverMiddleware(Logger))

	//
	eventstream.Subscribe(func(event interface{}) {
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
		case cluster.ClusterTopologyEvent:
			log.Printf("Cluster Topology Poll")
		}
	})

	cp, err := consul.New()
	if err != nil {
		log.Fatal(err)
	}
	cluster.Start("mycluster", "127.0.0.1:8081", cp)

	sync()
	async()

	console.ReadLine()
}

func sync() {
	hello := shared.GetHelloGrain("abc")
	options := cluster.NewGrainCallOptions().WithTimeout(5 * time.Second).WithRetry(5)
	res, err := hello.SayHelloWithOpts(&shared.HelloRequest{Name: "GAM"}, options)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Message from SayHello: %v", res.Message)
	for i := 0; i < 10000; i++ {
		x := shared.GetHelloGrain(fmt.Sprintf("hello%v", i))
		x.SayHello(&shared.HelloRequest{Name: "GAM"})
	}
	log.Println("Done")
}

func async() {
	hello := shared.GetHelloGrain("abc")
	c, e := hello.AddChan(&shared.AddRequest{A: 123, B: 456})

	for {
		select {
		case <-time.After(100 * time.Millisecond):
			log.Println("Tick..") // this might not happen if res returns fast enough
		case err := <-e:
			log.Fatal(err)
		case res := <-c:
			log.Printf("Result is %v", res.Result)
			return
		}
	}
}
