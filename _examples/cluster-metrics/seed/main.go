package main

import (
	"log"

	"cluster/shared"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/AsynkronIT/protoactor-go/cluster/consul"
	"github.com/AsynkronIT/protoactor-go/remote"
)

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

func newHelloActor() actor.Actor {
	return &shared.HelloActor{}
}

func main() {
	system := actor.NewActorSystem()
	remoteConfig := remote.Configure("127.0.0.1", 8080)

	helloKind := cluster.NewKind("Hello",
		actor.PropsFromProducer(newHelloActor).WithReceiverMiddleware(Logger))

	provider, _ := consul.New()
	clusterConfig := cluster.Configure("my-cluster", provider, remoteConfig, helloKind)
	c := cluster.New(system, clusterConfig)
	c.Start()

	// this node knows about Hello kind
	hello := shared.GetHelloGrain("MyGrain")

	res, err := hello.SayHello(&shared.HelloRequest{Name: "Roger"})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Message from grain: %v", res.Message)
	console.ReadLine()
}
