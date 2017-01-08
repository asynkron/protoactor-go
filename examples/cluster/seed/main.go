package main

import (
	"log"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/cluster"
	"github.com/AsynkronIT/protoactor-go/consul_cluster"
	"github.com/AsynkronIT/protoactor-go/examples/cluster/shared"
)

func main() {
	cp, err := consul_cluster.New()
	if err != nil {
		log.Fatal(err)
	}
	cluster.StartWithProvider("mycluster", "127.0.0.1:8080", cp)
	hello := shared.GetHelloGrain("MyGrain")

	res, err := hello.SayHello(&shared.HelloRequest{Name: "Roger"})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Message from grain: %v", res.Message)
	console.ReadLine()
}
