package main

import (
	"flag"
	"log"
	"time"

	"cluster-metrics/shared"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/AsynkronIT/protoactor-go/cluster/consul"
	logmod "github.com/AsynkronIT/protoactor-go/log"
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
	return &shared.HelloActor{
		Timeout: 20 * time.Second,
	}
}

func main() {
	port := flag.Int("port", 0, "")
	flag.Parse()
	system := actor.NewActorSystem()
	remoteConfig := remote.Configure("127.0.0.1", *port)
	props := actor.PropsFromProducer(newHelloActor).WithReceiverMiddleware(Logger)
	helloKind := cluster.NewKind("Hello", props)
	cluster.SetLogLevel(logmod.InfoLevel)

	provider, _ := consul.New()
	clusterConfig := cluster.Configure("my-cluster", provider, remoteConfig, helloKind)
	c := cluster.New(system, clusterConfig)
	c.Start()

	// this node knows about Hello kind
	hello := shared.GetHelloGrainClient(c, "MyGrain")
	msg := &shared.HelloRequest{Name: "Roger"}
	res, err := hello.SayHello(msg)
	if err != nil {
		log.Fatalf("failed to call SayHello, err:%v", err)
	}
	log.Printf("Message from grain: %v", res.Message)
	_, _ = console.ReadLine()
	c.Shutdown(true)
}
