package main

import (
	"log"

	console "github.com/AsynkronIT/goconsole"
	"github.com/AsynkronIT/protoactor-go/examples/cluster/shared"
	"github.com/AsynkronIT/protoactor-go/cluster"
)

func main() {
	cluster.Start("127.0.0.1:7711")
	log.Println("starting")
	hello := shared.GetHelloGrain("abc")

	res, err := hello.SayHello(&shared.HelloRequest{Name: "Roger"})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Message from grain %v", res.Message)
	console.ReadLine()
}
