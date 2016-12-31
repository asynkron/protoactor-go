package main

import (
	"log"

	"github.com/AsynkronIT/gam/languages/golang/examples/cluster/shared"
	"github.com/AsynkronIT/gam/languages/golang/src/cluster"
	console "github.com/AsynkronIT/goconsole"
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
