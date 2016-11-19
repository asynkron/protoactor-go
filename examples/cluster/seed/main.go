package main

import (
	"log"

	"github.com/AsynkronIT/gam/cluster"
	"github.com/AsynkronIT/gam/examples/cluster/shared"
	console "github.com/AsynkronIT/goconsole"
)

func main() {
	cluster.Start("127.0.0.1:7711")
	hello := shared.GetHelloGrain("abc")
	res := hello.SayHello(&shared.HelloRequest{Name: "Roger"})
	log.Printf("Message from grain %v", res.Message)
	console.ReadLine()
}
