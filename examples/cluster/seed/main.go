package main

import (
	"log"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/AsynkronIT/protoactor-go/consul_cluster"
	"github.com/AsynkronIT/protoactor-go/examples/cluster/shared"
	"github.com/AsynkronIT/protoactor-go/remoting"
)

func main() {

	//this node knows about Hello kind
	remoting.Register("Hello", actor.FromProducer(func() actor.Actor {
		return &shared.HelloActor{}
	}))

	cp, err := consul_cluster.New()
	if err != nil {
		log.Fatal(err)
	}
	cluster.Start("mycluster", "127.0.0.1:8080", cp)

	hello := shared.GetHelloGrain("MyGrain")

	res, err := hello.SayHello(&shared.HelloRequest{Name: "Roger"})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Message from grain: %v", res.Message)
	console.ReadLine()
}
